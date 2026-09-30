package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/store"
)

// ExtractionJob: mot lan trich xuat sau run (dua vao hang doi nen).
type ExtractionJob struct {
	RunID          string
	ConversationID string
	Prompt         string
	Reply          string
	DocumentKey    string
}

// Service: cua vao memory dai han cho Orchestrator, tool va API (muc 8.5). Doc lai MemoryEnabled /
// MemoryAutoExtract moi lan (doi trong Cai dat co hieu luc ngay). Mot worker nen cho trich xuat, tom tat hoi
// thoai va tinh bu embedding - khong chan pane, khong giu run.
type Service struct {
	store         *Store
	retriever     *Retriever
	conversations *store.Conversations
	runs          *store.Runs
	config        func() config.Config
	models        func(config.Config) *model.Client
	embeddings    *embeddingClient
	available     bool

	mu      sync.Mutex
	queue   []func(context.Context) error
	cond    *sync.Cond
	stop    context.Context
	stopped context.CancelFunc
	pending int
	closed  bool
}

func NewService(db *store.DB, conversations *store.Conversations, runs *store.Runs, load func() config.Config,
	models func(config.Config) *model.Client, httpClient *http.Client, available bool) *Service {
	service := &Service{
		store: NewStore(db), conversations: conversations, runs: runs, config: load, models: models,
		embeddings: newEmbeddingClient(httpClient), available: available,
	}
	service.retriever = NewRetriever(service.store)
	service.cond = sync.NewCond(&service.mu)
	service.stop, service.stopped = context.WithCancel(context.Background())
	go service.work()
	return service
}

func (s *Service) Available() bool { return s.available }

func (s *Service) Store() *Store { return s.store }

func (s *Service) Retriever() *Retriever { return s.retriever }

// Enabled: co san VA nguoi dung bat MemoryEnabled.
func (s *Service) Enabled() bool { return s.available && s.config().MemoryEnabled }

// Pending: so viec con trong hang doi nen (test cho doi).
func (s *Service) Pending() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending
}

// Today: ngay dia phuong (dung cho han cua memory).
func Today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
}

// Context: ngu canh cho prompt (muc 8.5.7, 8.8 phan 3).
func (s *Service) Context(ctx context.Context, prompt, documentKey string) Context {
	if !s.Enabled() {
		return EmptyContext()
	}
	semantic, problem := s.semanticScores(ctx, prompt)
	if problem != "" {
		corelog.Error("memory context failed: %s", problem)
		return EmptyContext()
	}
	return s.retriever.BuildContext(prompt, documentKey, Today(), semantic)
}

func (s *Service) Search(ctx context.Context, query, documentKey, scope string, limit int) []Hit {
	semantic, _ := s.semanticScores(ctx, query)
	return s.retriever.Search(query, documentKey, Today(), semantic, scope, limit)
}

// Add: ghi fact qua bo chong trung (muc 8.5.6): nguoi dung (API), agent (remember) hoac extract.
func (s *Service) Add(ctx context.Context, items []NewMemory, actor, runID string) []OpResult {
	cfg := s.config()
	var vectors map[int][]float32
	modelName := embeddingModelName(cfg)
	if modelName != "" && len(items) > 0 {
		texts := make([]string, 0, len(items))
		for _, item := range items {
			texts = append(texts, item.Text)
		}
		if computed := s.embeddings.embed(ctx, cfg, texts); computed != nil {
			vectors = map[int][]float32{}
			for index, vector := range computed {
				vectors[index] = vector
			}
		}
	}
	embeddingForSave := ""
	if vectors != nil {
		embeddingForSave = modelName
	}
	return s.store.AddBatch(items, actor, runID, vectors, embeddingForSave)
}

func (s *Service) TouchHits(ids []string) {
	if len(ids) == 0 {
		return
	}
	s.Enqueue(func(context.Context) error {
		s.store.TouchHits(ids)
		return nil
	})
}

// QueueExtraction: sau run completed - xep hang trich xuat (hoac danh dau skipped, khong ton lenh goi LLM).
func (s *Service) QueueExtraction(job ExtractionJob) string {
	cfg := s.config()
	reason := ""
	switch {
	case !s.available:
		reason = "memory unavailable"
	case !cfg.MemoryEnabled:
		reason = "memory disabled"
	case !cfg.MemoryAutoExtract:
		reason = "auto extract disabled"
	default:
		if skip, why := ShouldSkip(job.Prompt); skip {
			reason = why
		}
	}
	if reason != "" {
		s.runs.SetMemoryStatus(job.RunID, "skipped")
		corelog.Info("memory extract skipped for %s: %s", job.RunID, reason)
		return "skipped"
	}
	s.runs.SetMemoryStatus(job.RunID, "queued")
	s.Enqueue(func(ctx context.Context) error {
		s.extract(ctx, job)
		return nil
	})
	return "queued"
}

// Enqueue: xep mot viec vao hang doi nen (khong chan nguoi goi).
func (s *Service) Enqueue(work func(context.Context) error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.queue = append(s.queue, work)
	s.pending++
	s.cond.Signal()
	s.mu.Unlock()
}

func (s *Service) extract(ctx context.Context, job ExtractionJob) {
	cfg := s.config()
	recent := []RecentMessage{}
	for _, message := range s.conversations.Messages(job.ConversationID, 12) {
		if message.Role == "user" || message.Role == "assistant" {
			recent = append(recent, RecentMessage{Role: message.Role, Content: message.Content})
		}
	}

	// Lien quan theo tu khoa, lap them bang memory gan nhat cung pham vi: "toi da len pho giam doc" khong trung
	// tu nao voi "truong phong Ke toan" nhung extractor can thay ban cu de ghi su chuyen doi + link.
	related := []Item{}
	for _, hit := range s.retriever.Search(job.Prompt, job.DocumentKey, Today(), nil, "", MaxRelatedMemories) {
		related = append(related, hit.Item)
	}
	actives := s.store.Active(job.DocumentKey, Today())
	sort.SliceStable(actives, func(a, b int) bool { return actives[a].CreatedAt > actives[b].CreatedAt })
	for _, item := range actives {
		if len(related) >= MaxRelatedMemories {
			break
		}
		found := false
		for _, existing := range related {
			if existing.ID == item.ID {
				found = true
				break
			}
		}
		if !found {
			related = append(related, item)
		}
	}

	input := ExtractionInput{Prompt: job.Prompt, Reply: job.Reply, Recent: recent, Related: related,
		ObservedOn: Today(), DocumentKey: job.DocumentKey}
	facts, problem := Extract(ctx, s.models(cfg), input)
	if problem != "" {
		s.runs.SetMemoryStatus(job.RunID, "failed")
		corelog.Info("memory extract failed for %s: %s", job.RunID, problem)
		return
	}

	results := s.Add(ctx, facts, ActorExtract, job.RunID)
	s.runs.SetMemoryStatus(job.RunID, "done")
	added, duplicate, skipped := 0, 0, 0
	for _, result := range results {
		switch result.Event {
		case "ADD":
			added++
		case "DUPLICATE":
			duplicate++
		default:
			skipped++
		}
	}
	corelog.Info("memory extract %s: %d added, %d duplicate, %d skipped", job.RunID, added, duplicate, skipped)
}

// QueueEmbeddingBackfill: tinh bu embedding (doi EmbeddingModel, memory them khi endpoint loi) - chay nen.
func (s *Service) QueueEmbeddingBackfill() {
	s.Enqueue(func(ctx context.Context) error {
		cfg := s.config()
		modelName := embeddingModelName(cfg)
		if modelName == "" || !s.available {
			return nil
		}
		missing := s.store.MissingVectors(modelName, 64)
		if len(missing) == 0 {
			return nil
		}
		texts := make([]string, 0, len(missing))
		for _, item := range missing {
			texts = append(texts, item.Text)
		}
		vectors := s.embeddings.embed(ctx, cfg, texts)
		if vectors == nil {
			return nil
		}
		for index, item := range missing {
			if index >= len(vectors) {
				break
			}
			s.store.SaveVector(item.ID, modelName, vectors[index], Hash(item.Text))
		}
		return nil
	})
}

func (s *Service) semanticScores(ctx context.Context, query string) (map[string]float64, string) {
	cfg := s.config()
	modelName := embeddingModelName(cfg)
	if modelName == "" {
		return nil, ""
	}
	vectors := s.embeddings.embed(ctx, cfg, []string{query})
	if len(vectors) == 0 {
		return nil, ""
	}
	stored := s.store.Vectors(modelName)
	if len(stored) == 0 {
		return nil, ""
	}
	scores := map[string]float64{}
	for id, vector := range stored {
		scores[id] = Cosine(vectors[0], vector)
	}
	return scores, ""
}

// work: worker nen (mot viec mot luc, giu thu tu xep hang).
func (s *Service) work() {
	for {
		s.mu.Lock()
		for len(s.queue) == 0 && !s.closed {
			s.cond.Wait()
		}
		if s.closed && len(s.queue) == 0 {
			s.mu.Unlock()
			return
		}
		job := s.queue[0]
		s.queue = s.queue[1:]
		s.mu.Unlock()

		if err := job(s.stop); err != nil && s.stop.Err() == nil {
			corelog.Error("memory worker job failed: %v", err)
		}

		s.mu.Lock()
		s.pending--
		s.mu.Unlock()
	}
}

// Close: dung worker nen (Core dang tat).
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.cond.Broadcast()
	s.mu.Unlock()
	s.stopped()
}

func embeddingModelName(cfg config.Config) string {
	if cfg.EmbeddingModel == "" {
		return ""
	}
	return cfg.EmbeddingModel
}

// embeddingClient: embedding tuy chon (muc 8.5.8): {endpoint}/embeddings kieu OpenAI-compatible. Khong cau
// hinh / goi loi -> tra nil de chay chi keyword (FTS5), khong bao loi cho nguoi dung; loi chi log mot lan
// moi 10 phut.
type embeddingClient struct {
	http      *http.Client
	mu        sync.Mutex
	mutedTill time.Time
}

func newEmbeddingClient(httpClient *http.Client) *embeddingClient {
	return &embeddingClient{http: httpClient}
}

func (e *embeddingClient) embed(ctx context.Context, cfg config.Config, texts []string) [][]float32 {
	endpoint := cfg.EmbeddingEndpoint
	if endpoint == "" {
		endpoint = cfg.LlmEndpoint
	}
	endpoint = model.NormalizeEndpoint(endpoint)

	e.mu.Lock()
	muted := time.Now().Before(e.mutedTill)
	e.mu.Unlock()
	if cfg.EmbeddingModel == "" || endpoint == "" || len(texts) == 0 || muted {
		return nil
	}

	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	body, err := json.Marshal(map[string]any{"model": cfg.EmbeddingModel, "input": texts})
	if err != nil {
		return nil
	}
	request, err := http.NewRequestWithContext(callCtx, http.MethodPost, model.BuildURL(endpoint, "/embeddings"), bytes.NewReader(body))
	if err != nil {
		return e.mute(err.Error())
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	if cfg.LlmApiKey != "" {
		request.Header.Set("Authorization", "Bearer "+cfg.LlmApiKey)
	}
	response, err := e.http.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return e.mute(err.Error())
	}
	defer response.Body.Close()
	payload, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return e.mute("HTTP " + response.Status + ": " + model.Truncate(string(payload), 200))
	}

	var parsed struct {
		Data []struct {
			Index     *int      `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if json.Unmarshal(payload, &parsed) != nil || len(parsed.Data) != len(texts) {
		return e.mute("unexpected embeddings response")
	}
	vectors := make([][]float32, len(texts))
	for position, item := range parsed.Data {
		index := position
		if item.Index != nil {
			index = *item.Index
		}
		if index < 0 || index >= len(vectors) {
			continue
		}
		vectors[index] = item.Embedding
	}
	return vectors
}

func (e *embeddingClient) mute(reason string) [][]float32 {
	e.mu.Lock()
	e.mutedTill = time.Now().Add(10 * time.Minute)
	e.mu.Unlock()
	corelog.Info("embeddings unavailable, keyword-only memory search for 10 minutes: %s", reason)
	return nil
}
