package memory

import (
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"axiomoffice/core/internal/store"
)

// Pham vi memory (muc 8.5.2, 8.5.3).
const (
	ScopeUser     = "user"
	ScopeDocument = "document"
	ScopeSkill    = "skill"
)

// Tac nhan ghi memory.
const (
	ActorUser    = "user"    // form / API
	ActorAgent   = "agent"   // tool remember
	ActorExtract = "extract" // tu trich xuat sau run
)

const (
	// NearDuplicateCosine: cosine >= nguong nay thi coi la cung mot y.
	NearDuplicateCosine = 0.96
	// SoftDeleteRetentionDays: giu ban xoa mem bao lau truoc khi don han.
	SoftDeleteRetentionDays = 30
)

func IsValidScope(scope string) bool {
	return scope == ScopeUser || scope == ScopeDocument || scope == ScopeSkill
}

// NewMemory: mot fact can ghi.
type NewMemory struct {
	Scope      string
	ScopeKey   string
	Text       string
	Category   string
	Entities   []string
	LinkedIDs  []string
	ExpiresAt  string
	Confidence *float64
}

// Item: mot memory da luu.
type Item struct {
	ID           string
	RowID        int64
	Scope        string
	ScopeKey     *string
	Text         string
	Category     *string
	Entities     []string
	ExpiresAt    *string
	Source       string
	Confidence   *float64
	Pinned       bool
	Hits         int
	LastUsedAt   *string
	CreatedAt    string
	UpdatedAt    string
	DeletedAt    *string
	CreatedRunID *string
	Links        []string
}

// OpResult: ket qua ghi mot fact. Event: ADD | DUPLICATE | SKIPPED (kem ly do).
type OpResult struct {
	Event  string
	Text   string
	Item   *Item
	Reason string
}

type HistoryEntry struct {
	ID        int64
	MemoryID  string
	Event     string
	OldText   *string
	NewText   *string
	Actor     string
	RunID     *string
	Reason    *string
	CreatedAt string
}

const memoryColumns = "rowid, id, scope, scope_key, text, category, entities_json, expires_at, source, confidence, pinned, hits, " +
	"last_used_at, created_at, updated_at, deleted_at, created_run_id"

// Store: kho memory SQLite (muc 8.5.3). Model chi ADD (qua AddBatch); sua/xoa/ghim/khoi phuc chi do nguoi dung.
type Store struct{ db *store.DB }

func NewStore(db *store.DB) *Store { return &Store{db: db} }

// NewID: "m_" + UUID kieu v7 (6 byte dau la moc thoi gian) - giong Guid.CreateVersion7 cua ban .NET.
func NewID() string {
	buffer := make([]byte, 16)
	ms := uint64(time.Now().UnixMilli())
	buffer[0] = byte(ms >> 40)
	buffer[1] = byte(ms >> 32)
	buffer[2] = byte(ms >> 24)
	buffer[3] = byte(ms >> 16)
	buffer[4] = byte(ms >> 8)
	buffer[5] = byte(ms)
	if _, err := rand.Read(buffer[6:]); err != nil {
		binary.BigEndian.PutUint64(buffer[8:], uint64(time.Now().UnixNano()))
	}
	buffer[6] = (buffer[6] & 0x0f) | 0x70 // version 7
	buffer[8] = (buffer[8] & 0x3f) | 0x80 // variant
	return "m_" + hex.EncodeToString(buffer)
}

type dbHandle interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// AddBatch: ghi mot lo fact trong MOT transaction (muc 8.5.6) - bo trung hash trong lo va trong cung
// (scope, scope_key), gan-trung bang cosine >= 0.96 neu co vector, link chi toi memory ton tai, ghi history ADD.
func (s *Store) AddBatch(items []NewMemory, actor, runID string, vectors map[int][]float32, embeddingModel string) []OpResult {
	results := make([]OpResult, 0, len(items))
	tx, err := s.db.SQL().Begin()
	if err != nil {
		for _, item := range items {
			results = append(results, OpResult{Event: "SKIPPED", Text: item.Text, Reason: "database: " + err.Error()})
		}
		return results
	}
	defer func() { _ = tx.Rollback() }()

	batchHashes := map[string]bool{}
	for index, raw := range items {
		text := strings.TrimSpace(raw.Text)
		switch {
		case text == "":
			results = append(results, OpResult{Event: "SKIPPED", Text: text, Reason: "empty"})
			continue
		case len([]rune(text)) > MaxFactLength:
			results = append(results, OpResult{Event: "SKIPPED", Text: text, Reason: "too long"})
			continue
		case !IsValidScope(raw.Scope):
			results = append(results, OpResult{Event: "SKIPPED", Text: text, Reason: "invalid scope"})
			continue
		case IsSensitive(text):
			results = append(results, OpResult{Event: "SKIPPED", Text: text, Reason: "sensitive"})
			continue
		}

		scopeKey := raw.ScopeKey
		if raw.Scope == ScopeUser {
			scopeKey = ""
		}
		hash := Hash(text)
		batchKey := raw.Scope + "|" + scopeKey + "|" + hash
		if batchHashes[batchKey] {
			results = append(results, OpResult{Event: "DUPLICATE", Text: text, Reason: "duplicate in batch"})
			continue
		}
		batchHashes[batchKey] = true

		existing := findByHash(tx, raw.Scope, scopeKey, hash)
		var vector []float32
		if vectors != nil {
			vector = vectors[index]
		}
		if existing == nil && len(vector) > 0 {
			existing = findNearDuplicate(tx, raw.Scope, scopeKey, vector)
		}
		if existing != nil {
			_, _ = tx.Exec("UPDATE memories SET hits = hits + 1, last_used_at = ? WHERE id = ?", Now(), existing.ID)
			results = append(results, OpResult{Event: "DUPLICATE", Text: text, Item: existing, Reason: "same as " + existing.ID})
			continue
		}

		id, now := NewID(), Now()
		confidence := raw.Confidence
		if actor == ActorUser {
			// Nguoi dung tu khai bao: tin tuyet doi.
			one := 1.0
			confidence = &one
		}
		expires := ValidDate(raw.ExpiresAt)
		_, err := tx.Exec(`INSERT INTO memories (id, scope, scope_key, text, hash, category, entities_json, expires_at, source,
			  confidence, pinned, hits, created_at, updated_at, created_run_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?, ?)`,
			id, raw.Scope, nullText(scopeKey), text, hash, nullText(raw.Category), entitiesJSON(raw.Entities),
			nullText(expires), actor, confidence, now, now, nullText(runID))
		if err != nil {
			results = append(results, OpResult{Event: "SKIPPED", Text: text, Reason: err.Error()})
			continue
		}

		var rowID int64
		if err := tx.QueryRow("SELECT last_insert_rowid()").Scan(&rowID); err != nil {
			rowID = 0
		}
		indexText(tx, rowID, text)

		for _, linked := range distinct(raw.LinkedIDs) {
			if exists(tx, linked) {
				_, _ = tx.Exec("INSERT OR IGNORE INTO memory_links (memory_id, linked_memory_id, created_at) VALUES (?, ?, ?)",
					id, linked, now)
			}
		}

		addHistory(tx, id, "ADD", "", text, actor, runID, "")
		if len(vector) > 0 && embeddingModel != "" {
			saveVector(tx, id, embeddingModel, vector, hash)
		}
		results = append(results, OpResult{Event: "ADD", Text: text, Item: getItem(tx, id)})
	}

	if err := tx.Commit(); err != nil {
		for i := range results {
			results[i] = OpResult{Event: "SKIPPED", Text: results[i].Text, Reason: "commit: " + err.Error()}
		}
	}
	return results
}

func (s *Store) Get(id string) *Item { return getItem(s.db.SQL(), id) }

func (s *Store) List(scope, scopeKey string, includeDeleted bool, runID string, limit int) []Item {
	where := []string{}
	args := []any{}
	if !includeDeleted {
		where = append(where, "deleted_at IS NULL")
	}
	if scope != "" {
		where = append(where, "scope = ?")
		args = append(args, scope)
	}
	if scopeKey != "" {
		where = append(where, "scope_key = ?")
		args = append(args, scopeKey)
	}
	if runID != "" {
		where = append(where, "created_run_id = ?")
		args = append(args, runID)
	}
	query := "SELECT " + memoryColumns + " FROM memories"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY pinned DESC, created_at DESC LIMIT ?"
	args = append(args, limit)
	return readItems(s.db.SQL(), query, args...)
}

// Active: ung vien cho ngu canh - chua xoa, chua het han (expires_at > hom qua), thuoc user + document hien tai.
func (s *Store) Active(documentKey string, today time.Time) []Item {
	yesterday := today.AddDate(0, 0, -1).Format("2006-01-02")
	return readItems(s.db.SQL(), "SELECT "+memoryColumns+" FROM memories WHERE deleted_at IS NULL"+
		" AND (expires_at IS NULL OR expires_at > ?)"+
		" AND (scope = 'user' OR (scope = 'document' AND scope_key = ?))",
		yesterday, nullText(documentKey))
}

// KeywordScores: bm25 cua FTS5 cho cac rowid khop truy van (diem cang THAP cang khop).
func (s *Store) KeywordScores(ftsQuery string) map[int64]float64 {
	scores := map[int64]float64{}
	rows, err := s.db.SQL().Query("SELECT rowid, bm25(memories_fts) FROM memories_fts WHERE memories_fts MATCH ?", ftsQuery)
	if err != nil {
		return scores
	}
	defer rows.Close()
	for rows.Next() {
		var rowID int64
		var score float64
		if rows.Scan(&rowID, &score) == nil {
			scores[rowID] = score
		}
	}
	return scores
}

func (s *Store) Vectors(model string) map[string][]float32 {
	vectors := map[string][]float32{}
	rows, err := s.db.SQL().Query("SELECT memory_id, vector FROM memory_embeddings WHERE model = ?", model)
	if err != nil {
		return vectors
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var blob []byte
		if rows.Scan(&id, &blob) == nil {
			vectors[id] = fromBlob(blob)
		}
	}
	return vectors
}

func (s *Store) SaveVector(id, model string, vector []float32, textHash string) {
	saveVector(s.db.SQL(), id, model, vector, textHash)
}

// MissingVectors: memory chua co vector cua model hien tai (tinh bu o nen khi doi EmbeddingModel).
func (s *Store) MissingVectors(model string, limit int) []Item {
	return readItems(s.db.SQL(), `SELECT `+memoryColumns+` FROM memories m WHERE deleted_at IS NULL AND NOT EXISTS
		(SELECT 1 FROM memory_embeddings e WHERE e.memory_id = m.id AND e.model = ? AND e.text_hash = m.hash) LIMIT ?`, model, limit)
}

// EntityCounts: so memory dang dung gan voi moi thuc the (cho entity boost giam dan).
func (s *Store) EntityCounts() map[string]int {
	counts := map[string]int{}
	for _, item := range s.List("", "", false, "", 100_000) {
		seen := map[string]bool{}
		for _, entity := range item.Entities {
			normalized := Normalize(entity)
			if normalized == "" || seen[normalized] {
				continue
			}
			seen[normalized] = true
			counts[normalized]++
		}
	}
	return counts
}

func (s *Store) TouchHits(ids []string) {
	list := distinct(ids)
	if len(list) == 0 {
		return
	}
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback() }()
	now := Now()
	for _, id := range list {
		_, _ = tx.Exec("UPDATE memories SET hits = hits + 1, last_used_at = ? WHERE id = ?", now, id)
	}
	_ = tx.Commit()
}

// ---- chi nguoi dung (form / API) ----

// Update: sua text/danh muc/han/ghim. Loi dinh dang -> error de API tra 400.
func (s *Store) Update(id, newText, category, expiresAt string, clearExpires bool, pinned *bool, reason string) (*Item, error) {
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	item := getItem(tx, id)
	if item == nil || item.DeletedAt != nil {
		return nil, nil
	}

	now := Now()
	if newText != "" {
		text := strings.TrimSpace(newText)
		if text == "" || len([]rune(text)) > MaxFactLength {
			return nil, fmt.Errorf("text must be 1-%d characters", MaxFactLength)
		}
		if text != item.Text {
			_, _ = tx.Exec("UPDATE memories SET text = ?, hash = ?, updated_at = ? WHERE id = ?", text, Hash(text), now, id)
			_, _ = tx.Exec("DELETE FROM memories_fts WHERE rowid = ?", item.RowID)
			indexText(tx, item.RowID, text)
			_, _ = tx.Exec("DELETE FROM memory_embeddings WHERE memory_id = ?", id)
			addHistory(tx, id, "UPDATE", item.Text, text, ActorUser, "", reason)
		}
	}

	if category != "" {
		_, _ = tx.Exec("UPDATE memories SET category = ?, updated_at = ? WHERE id = ?", category, now, id)
	}

	if clearExpires || expiresAt != "" {
		date := ""
		if !clearExpires {
			date = ValidDate(expiresAt)
			if date == "" {
				return nil, errors.New("expiresAt must be YYYY-MM-DD")
			}
		}
		_, _ = tx.Exec("UPDATE memories SET expires_at = ?, updated_at = ? WHERE id = ?", nullText(date), now, id)
		addHistory(tx, id, "EXPIRES", textOrEmpty(item.ExpiresAt), date, ActorUser, "", reason)
	}

	if pinned != nil && *pinned != item.Pinned {
		_, _ = tx.Exec("UPDATE memories SET pinned = ?, updated_at = ? WHERE id = ?", boolInt(*pinned), now, id)
		event := "UNPIN"
		if *pinned {
			event = "PIN"
		}
		addHistory(tx, id, event, "", "", ActorUser, "", reason)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.Get(id), nil
}

func (s *Store) Delete(id, reason string) bool {
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return false
	}
	defer func() { _ = tx.Rollback() }()
	item := getItem(tx, id)
	if item == nil || item.DeletedAt != nil {
		return false
	}
	now := Now()
	_, _ = tx.Exec("UPDATE memories SET deleted_at = ?, updated_at = ? WHERE id = ?", now, now, id)
	_, _ = tx.Exec("DELETE FROM memories_fts WHERE rowid = ?", item.RowID)
	addHistory(tx, id, "DELETE", item.Text, "", ActorUser, "", reason)
	return tx.Commit() == nil
}

func (s *Store) Restore(id string) bool {
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return false
	}
	defer func() { _ = tx.Rollback() }()
	item := getItem(tx, id)
	if item == nil || item.DeletedAt == nil {
		return false
	}
	_, _ = tx.Exec("UPDATE memories SET deleted_at = NULL, updated_at = ? WHERE id = ?", Now(), id)
	indexText(tx, item.RowID, item.Text)
	addHistory(tx, id, "RESTORE", "", item.Text, ActorUser, "", "")
	return tx.Commit() == nil
}

func (s *Store) History(id string) []HistoryEntry {
	entries := []HistoryEntry{}
	rows, err := s.db.SQL().Query("SELECT id, memory_id, event, old_text, new_text, actor, run_id, reason, created_at "+
		"FROM memory_history WHERE memory_id = ? ORDER BY id", id)
	if err != nil {
		return entries
	}
	defer rows.Close()
	for rows.Next() {
		var entry HistoryEntry
		var oldText, newText, runID, reason sql.NullString
		if rows.Scan(&entry.ID, &entry.MemoryID, &entry.Event, &oldText, &newText, &entry.Actor, &runID, &reason,
			&entry.CreatedAt) != nil {
			continue
		}
		entry.OldText, entry.NewText = nullable(oldText), nullable(newText)
		entry.RunID, entry.Reason = nullable(runID), nullable(reason)
		entries = append(entries, entry)
	}
	return entries
}

// Purge: xoa cung (scope rong = tat ca). Dung cho "Xoa toan bo ghi nho" va don xoa mem cu.
func (s *Store) Purge(scope string) int {
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return 0
	}
	defer func() { _ = tx.Rollback() }()
	filter, args := "", []any{}
	if scope != "" {
		filter, args = " WHERE scope = ?", []any{scope}
	}
	_, _ = tx.Exec("DELETE FROM memories_fts WHERE rowid IN (SELECT rowid FROM memories"+filter+")", args...)
	_, _ = tx.Exec("DELETE FROM memory_history WHERE memory_id IN (SELECT id FROM memories"+filter+")", args...)
	result, err := tx.Exec("DELETE FROM memories"+filter, args...)
	if err != nil {
		return 0
	}
	removed, _ := result.RowsAffected()
	if tx.Commit() != nil {
		return 0
	}
	return int(removed)
}

func (s *Store) PurgeSoftDeleted(now time.Time) int {
	cutoff := now.AddDate(0, 0, -SoftDeleteRetentionDays).UTC().Format("2006-01-02T15:04:05.000Z")
	result, err := s.db.SQL().Exec("DELETE FROM memories WHERE deleted_at IS NOT NULL AND deleted_at < ?", cutoff)
	if err != nil {
		return 0
	}
	removed, _ := result.RowsAffected()
	return int(removed)
}

// Cosine: do giong nhau giua hai vector (0 khi khac so chieu hoac vector rong).
func Cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// ---- noi bo ----

func indexText(db dbHandle, rowID int64, text string) {
	_, _ = db.Exec("INSERT INTO memories_fts (rowid, text) VALUES (?, ?)", rowID, Normalize(text))
}

func findByHash(db dbHandle, scope, scopeKey, hash string) *Item {
	items := readItems(db, "SELECT "+memoryColumns+" FROM memories WHERE deleted_at IS NULL AND scope = ? "+
		"AND scope_key IS ? AND hash = ? LIMIT 1", scope, nullText(scopeKey), hash)
	if len(items) == 0 {
		return nil
	}
	return &items[0]
}

func findNearDuplicate(db dbHandle, scope, scopeKey string, vector []float32) *Item {
	rows, err := db.Query("SELECT e.memory_id, e.vector FROM memory_embeddings e JOIN memories m ON m.id = e.memory_id "+
		"WHERE m.deleted_at IS NULL AND m.scope = ? AND m.scope_key IS ? AND e.dim = ?", scope, nullText(scopeKey), len(vector))
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var blob []byte
		if rows.Scan(&id, &blob) != nil {
			continue
		}
		if Cosine(vector, fromBlob(blob)) >= NearDuplicateCosine {
			return getItem(db, id)
		}
	}
	return nil
}

func saveVector(db dbHandle, id, model string, vector []float32, textHash string) {
	_, _ = db.Exec(`INSERT INTO memory_embeddings (memory_id, model, dim, vector, text_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(memory_id) DO UPDATE SET model = ?, dim = ?, vector = ?, text_hash = ?, created_at = ?`,
		id, model, len(vector), toBlob(vector), textHash, Now(), model, len(vector), toBlob(vector), textHash, Now())
}

func exists(db dbHandle, id string) bool {
	var one int
	return db.QueryRow("SELECT 1 FROM memories WHERE id = ? AND deleted_at IS NULL", id).Scan(&one) == nil
}

func getItem(db dbHandle, id string) *Item {
	items := readItems(db, "SELECT "+memoryColumns+" FROM memories WHERE id = ?", id)
	if len(items) == 0 {
		return nil
	}
	return &items[0]
}

func readItems(db dbHandle, query string, args ...any) []Item {
	rows, err := db.Query(query, args...)
	if err != nil {
		return []Item{}
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var (
			item                                                Item
			scopeKey, category, expires, lastUsed, deleted, run sql.NullString
			entitiesJSON                                        sql.NullString
			confidence                                          sql.NullFloat64
			pinned, hits                                        int64
		)
		if rows.Scan(&item.RowID, &item.ID, &item.Scope, &scopeKey, &item.Text, &category, &entitiesJSON, &expires,
			&item.Source, &confidence, &pinned, &hits, &lastUsed, &item.CreatedAt, &item.UpdatedAt, &deleted,
			&run) != nil {
			continue
		}
		item.ScopeKey = nullable(scopeKey)
		item.Category = nullable(category)
		item.ExpiresAt = nullable(expires)
		item.LastUsedAt = nullable(lastUsed)
		item.DeletedAt = nullable(deleted)
		item.CreatedRunID = nullable(run)
		if confidence.Valid {
			value := confidence.Float64
			item.Confidence = &value
		}
		item.Pinned = pinned != 0
		item.Hits = int(hits)
		item.Entities = decodeEntities(entitiesJSON)
		item.Links = []string{}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return []Item{}
	}

	// Lien ket (memory moi -> memory cu) cho tung memory.
	for index := range items {
		links, err := db.Query("SELECT linked_memory_id FROM memory_links WHERE memory_id = ?", items[index].ID)
		if err != nil {
			continue
		}
		ids := []string{}
		for links.Next() {
			var linked string
			if links.Scan(&linked) == nil {
				ids = append(ids, linked)
			}
		}
		links.Close()
		if len(ids) > 0 {
			items[index].Links = ids
		}
	}
	return items
}

func addHistory(db dbHandle, id, event, oldText, newText, actor, runID, reason string) {
	_, _ = db.Exec(`INSERT INTO memory_history (memory_id, event, old_text, new_text, actor, run_id, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, event, nullText(oldText), nullText(newText), actor, nullText(runID),
		nullText(reason), Now())
}

func nullText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullable(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}

func textOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func entitiesJSON(entities []string) string {
	cleaned := []string{}
	for _, entity := range entities {
		trimmed := strings.TrimSpace(entity)
		if trimmed == "" || len([]rune(trimmed)) > 100 {
			continue
		}
		duplicate := false
		for _, existing := range cleaned {
			if strings.EqualFold(existing, trimmed) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			cleaned = append(cleaned, trimmed)
		}
		if len(cleaned) >= 10 {
			break
		}
	}
	encoded, err := json.Marshal(cleaned)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func decodeEntities(value sql.NullString) []string {
	entities := []string{}
	if !value.Valid || value.String == "" {
		return entities
	}
	_ = json.Unmarshal([]byte(value.String), &entities)
	return entities
}

func toBlob(vector []float32) []byte {
	blob := make([]byte, len(vector)*4)
	for index, value := range vector {
		binary.LittleEndian.PutUint32(blob[index*4:], math.Float32bits(value))
	}
	return blob
}

func fromBlob(blob []byte) []float32 {
	vector := make([]float32, len(blob)/4)
	for index := range vector {
		vector[index] = math.Float32frombits(binary.LittleEndian.Uint32(blob[index*4:]))
	}
	return vector
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func distinct(values []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
