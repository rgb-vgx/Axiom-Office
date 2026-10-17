package memory

import (
	"math"
	"sort"
	"strings"
	"time"

	"axiomoffice/core/internal/corelog"
)

// Hit: mot memory khop truy van kem diem thanh phan.
type Hit struct {
	Item     Item
	Score    float64
	Keyword  float64
	Semantic float64
	Entity   float64
}

// Context: ket qua dua vao prompt - cac dong "- [id ngan] text" + id da dung (tang hits sau run).
// OrgLines: chuan trinh bay (org-style cua don vi + doc-style nguoi dung da chon cho app nay) - prompt hien thanh
// nhom rieng (12-m3).
type Context struct {
	Lines    []string
	IDs      []string
	OrgLines []string
}

func EmptyContext() Context { return Context{Lines: []string{}, IDs: []string{}, OrgLines: []string{}} }

const (
	KeywordWeight       = 1.0
	SemanticWeight      = 1.0
	EntityWeight        = 0.5 // ENTITY_BOOST_WEIGHT cua mem0
	Threshold           = 0.1 // chan tin hieu chinh TRUOC khi cong
	MaxDocumentMemories = 20
	MaxUserMemories     = 8
	// MaxOrgStyleMemories: tran rieng cho chuan trinh bay (org-style + doc-style) - khong chiem het cho cua fact tai
	// lieu (12-m3).
	MaxOrgStyleMemories = 12
	MaxContextChars     = 6000 // ~1.500 token
	// RawPerMatchedTerm: diem keyword tho cong them cho moi tu truy van khop (ly do ghi trong ban .NET).
	RawPerMatchedTerm = 4.0
)

// Retriever: tim memory lien quan (muc 8.5.7) - cham diem cong don nhu score_and_rank cua mem0 2.2.1.
type Retriever struct{ store *Store }

func NewRetriever(store *Store) *Retriever { return &Retriever{store: store} }

// Bm25Params: tham so sigmoid theo do dai truy van (get_bm25_params cua mem0).
func Bm25Params(queryTerms int) (midpoint, steepness float64) {
	switch {
	case queryTerms <= 3:
		return 5, 0.7
	case queryTerms <= 6:
		return 7, 0.6
	case queryTerms <= 9:
		return 9, 0.5
	case queryTerms <= 15:
		return 10, 0.5
	}
	return 12, 0.5
}

func NormalizeBm25(rawScore float64, queryTerms int) float64 {
	midpoint, steepness := Bm25Params(queryTerms)
	return 1.0 / (1.0 + math.Exp(-steepness*(rawScore-midpoint)))
}

// EntityBoost: boost thuc the giam khi thuc the gan qua nhieu memory (memory_count_weight cua mem0).
func EntityBoost(memoriesWithEntity int) float64 {
	n := memoriesWithEntity
	if n < 1 {
		n = 1
	}
	return EntityWeight * (1.0 / (1.0 + 0.001*math.Pow(float64(n-1), 2)))
}

// Search: semantic = cosine theo memory id (nil = khong co embedding). app = app dang mo (`wps`/`et`/`wpp`).
func (r *Retriever) Search(query, app, documentKey string, today time.Time, semantic map[string]float64, scope string, limit int) []Hit {
	candidates := []Item{}
	for _, item := range r.store.Active(app, documentKey, today) {
		if scope != "" && item.Scope != scope {
			continue
		}
		candidates = append(candidates, item)
	}
	return r.Score(candidates, query, semantic, limit)
}

func (r *Retriever) Score(candidates []Item, query string, semantic map[string]float64, limit int) []Hit {
	if len(candidates) == 0 {
		return []Hit{}
	}

	terms := QueryTerms(query)
	fts := FtsQuery(terms)
	bm25 := map[int64]float64{}
	if fts != "" {
		bm25 = r.store.KeywordScores(fts)
	}

	normalizedQuery := " " + Normalize(query) + " "
	entityCounts := map[string]int{}
	hasEntities := false
	for _, item := range candidates {
		if len(item.Entities) > 0 {
			hasEntities = true
			break
		}
	}
	if hasEntities {
		entityCounts = r.store.EntityCounts()
	}

	entityScores := map[string]float64{}
	entityEnabled := false
	for _, item := range candidates {
		best := 0.0
		for _, entity := range item.Entities {
			normalized := Normalize(entity)
			if len([]rune(normalized)) <= 1 {
				continue
			}
			if strings.Contains(normalizedQuery, " "+normalized+" ") {
				best = math.Max(best, EntityBoost(entityCounts[normalized]))
			}
		}
		if best > 0 {
			entityEnabled = true
			entityScores[item.ID] = best
		}
	}

	hasSemantic := len(semantic) > 0
	maxPossible := KeywordWeight + (boolWeight(hasSemantic) * SemanticWeight) + (boolWeight(entityEnabled) * EntityWeight)

	termSet := map[string]bool{}
	for _, term := range terms {
		termSet[term] = true
	}

	hits := []Hit{}
	for _, item := range candidates {
		keyword := KeywordScore(item, termSet, bm25, len(terms))
		semanticScore := 0.0
		if hasSemantic {
			if cosine, ok := semantic[item.ID]; ok {
				semanticScore = math.Max(0, cosine)
			}
		}
		entity := entityScores[item.ID]

		// Nguong chan tin hieu chinh: co embedding -> cosine; khong -> keyword.
		// Tai lieu hien tai va ghim luon duoc giu.
		primary := keyword
		if hasSemantic {
			primary = semanticScore
		}
		if !(item.Pinned || item.Scope == ScopeDocument || primary >= Threshold || entity > 0) {
			continue
		}

		combined := (keyword*KeywordWeight + semanticScore*SemanticWeight + entity) / maxPossible
		hits = append(hits, Hit{Item: item, Score: combined, Keyword: keyword, Semantic: semanticScore, Entity: entity})
	}

	// Bang diem thi uu tien ban MOI hon (ban ghi chuyen doi thang ban cu).
	sort.SliceStable(hits, func(a, b int) bool {
		left, right := math.Round(hits[a].Score*1e6)/1e6, math.Round(hits[b].Score*1e6)/1e6
		if left != right {
			return left > right
		}
		return hits[a].Item.CreatedAt > hits[b].Item.CreatedAt
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// KeywordScore: diem keyword tho = -bm25 (FTS5) + 4 x so tu truy van khop, roi sigmoid theo do dai truy van.
// Ly do cong so tu khop: IDF cua bm25 trong FTS5 bi kep khi kho it memory (may moi dung) nen mot memory khop
// hoan toan cung chi duoc ~0 diem va khong bao gio qua nguong 0.1 (ghi chu day du o ban .NET).
func KeywordScore(item Item, terms map[string]bool, bm25 map[int64]float64, termCount int) float64 {
	if len(terms) == 0 {
		return 0
	}
	matched := 0
	seen := map[string]bool{}
	for _, word := range strings.Fields(Normalize(item.Text)) {
		if seen[word] {
			continue
		}
		seen[word] = true
		if terms[word] {
			matched++
		}
	}
	raw := -bm25[item.RowID] + RawPerMatchedTerm*float64(matched)
	if raw <= 0 {
		return 0
	}
	return NormalizeBm25(raw, termCount)
}

// ScopeFilter: scope nao duoc dua vao ngu canh (M4-04: tat rieng memory tai lieu / ca nhan, co hieu luc ngay,
// khong xoa du lieu). org-style (chuan chung) luon duoc giu.
type ScopeFilter struct {
	Document bool
	User     bool
}

// AllScopes: mac dinh - bat ca hai.
func AllScopes() ScopeFilter { return ScopeFilter{Document: true, User: true} }

// BuildContext: ngu canh cho prompt - moi memory ghim + chuan trinh bay (org-style/doc-style cua app nay); <= 20
// memory tai lieu; <= 8 memory user tren nguong; <= ~1.500 token. filter bo trong = AllScopes().
func (r *Retriever) BuildContext(prompt, app, documentKey string, today time.Time, semantic map[string]float64, filter ...ScopeFilter) Context {
	allowed := AllScopes()
	if len(filter) > 0 {
		allowed = filter[0]
	}
	candidates := r.store.Active(app, documentKey, today)
	filtered := candidates[:0]
	for _, item := range candidates {
		if item.Scope == ScopeDocument && !allowed.Document {
			continue
		}
		if item.Scope == ScopeUser && !allowed.User {
			continue
		}
		filtered = append(filtered, item)
	}
	candidates = filtered
	if len(candidates) == 0 {
		return EmptyContext()
	}

	ranked := r.Score(candidates, prompt, semantic, 0)

	// Chuan trinh bay (org-style + doc-style): luon vao ngu canh (nhu ghim), co tran rieng, xep truoc de duoc uu
	// tien ngan sach.
	orgs := []Item{}
	for _, item := range candidates {
		if item.Scope == ScopeOrgStyle || item.Scope == ScopeDocStyle {
			orgs = append(orgs, item)
		}
	}
	sort.SliceStable(orgs, func(a, b int) bool { return orgs[a].CreatedAt > orgs[b].CreatedAt })
	if len(orgs) > MaxOrgStyleMemories {
		orgs = orgs[:MaxOrgStyleMemories]
	}

	chosen := []Item{}
	chosen = append(chosen, orgs...)
	for _, item := range candidates {
		if item.Pinned {
			chosen = append(chosen, item)
		}
	}
	sort.SliceStable(chosen, func(a, b int) bool { return chosen[a].CreatedAt > chosen[b].CreatedAt })

	documentCount := 0
	for _, hit := range ranked {
		if hit.Item.Pinned || hit.Item.Scope != ScopeDocument {
			continue
		}
		if documentCount >= MaxDocumentMemories {
			break
		}
		documentCount++
		chosen = append(chosen, hit.Item)
	}

	userCount := 0
	for _, hit := range ranked {
		if hit.Item.Pinned || hit.Item.Scope != ScopeUser {
			continue
		}
		if hit.Keyword < Threshold && hit.Semantic < Threshold && hit.Entity <= 0 {
			continue
		}
		if userCount >= MaxUserMemories {
			break
		}
		userCount++
		chosen = append(chosen, hit.Item)
	}

	lines, orgLines, ids, used, seen := []string{}, []string{}, []string{}, 0, map[string]bool{}
	cut := 0
	for _, item := range chosen {
		if seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		line := "[" + ShortID(item.ID) + "] " + item.Text
		if item.Scope == ScopeDocument {
			line += " (tài liệu này)"
		}
		if used+len([]rune(line)) > MaxContextChars {
			cut++
			continue
		}
		used += len([]rune(line))
		if item.Scope == ScopeOrgStyle || item.Scope == ScopeDocStyle {
			orgLines = append(orgLines, line)
		} else {
			lines = append(lines, line)
		}
		ids = append(ids, item.ID)
	}
	// M2-04: ngân sách token có log - biết khi nào phải chỉnh MaxContextChars/MaxUserMemories.
	if cut > 0 {
		corelog.Info("memory context: %d fact vao prompt (%d chuan chung), %d bi cat (ngan sach %d ky tu, da dung %d)",
			len(lines)+len(orgLines), len(orgLines), cut, MaxContextChars, used)
	}
	return Context{Lines: lines, IDs: ids, OrgLines: orgLines}
}

func ShortID(id string) string {
	runes := []rune(id)
	if len(runes) > 6 {
		return string(runes[len(runes)-6:])
	}
	return id
}

func boolWeight(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
