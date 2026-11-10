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
	"sort"
	"strings"
	"time"

	"axiomoffice/core/internal/store"
)

// Pham vi memory (muc 8.5.2, 8.5.3).
const (
	ScopeUser     = "user"
	ScopeDocument = "document"
	ScopeSkill    = "skill"
	// ScopeOrgStyle: chuan chung cua don vi (font/le/cach xung ho da duyet). Luon vao ngu canh, khong tu het han
	// va khong gan theo tai lieu (12-m3). Chi quan tri vien ghi duoc (M3-04).
	//
	// Tu 17/10/2026 (12-m3 muc 7): tach theo APP bang scope_key (`et`/`wps`/`wpp`); scope_key rong = chuan dung
	// chung cho moi app. Khong doi schema nen ban .NET khong bi anh huong.
	ScopeOrgStyle = "org-style"
	// ScopeDocStyle: kieu trinh bay NGUOI DUNG da chon cho mot app (doc_design nho lua chon - M3-02), cung tach
	// theo app bang scope_key. Khac org-style: day la so thich ca nhan, khong can quyen quan tri.
	ScopeDocStyle = "doc-style"
)

// App kind chuan (trung Session.App cua bridge): Word = wps, Excel = et, PowerPoint = wpp.
const (
	AppWord       = "wps"
	AppExcel      = "et"
	AppPowerPoint = "wpp"
)

// NormalizeApp: app kind chuan hoa ve `wps`/`et`/`wpp`, nhan them ten tieng Anh/duoi tep cho API/nguoi go tay.
// "" = khong ro app (chuan chung dung chung moi app).
func NormalizeApp(app string) string {
	switch strings.ToLower(strings.TrimSpace(app)) {
	case "wps", "word", "winword", "writer", "docx":
		return AppWord
	case "et", "excel", "excel.exe", "xlsx", "spreadsheet":
		return AppExcel
	case "wpp", "ppt", "powerpoint", "powerpnt", "pptx", "presentation":
		return AppPowerPoint
	}
	return ""
}

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
	return scope == ScopeUser || scope == ScopeDocument || scope == ScopeSkill ||
		scope == ScopeOrgStyle || scope == ScopeDocStyle
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

// isBusyError: SQLite bao khoa ban (doi tuong ghi khac giu qua busy_timeout).
func isBusyError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "SQLITE_BUSY")
}

// isBusyErrorString: nhu isBusyError cho ly do loi da chuyen thanh chuoi.
func isBusyErrorString(reason string) bool {
	return strings.Contains(reason, "SQLITE_BUSY")
}

// AddBatch: ghi mot lo fact trong MOT transaction (muc 8.5.6) - bo trung hash trong lo va trong cung
// (scope, scope_key), gan-trung bang cosine >= 0.96 neu co vector, link chi toi memory ton tai, ghi history ADD.
//
// Ghi nen (API) va trich xuat nen (sau run) chay song song tren cung file nen INSERT co the gap khoa
// ban du busy_timeout 5s (log e2e: "database is locked (5) (SQLITE_BUSY)"). Chua commit thi rollback
// sach nen thu lai toi da 4 lan thay vi tra SKIPPED ngay - duong thanh cong khong doi gi.
func (s *Store) AddBatch(items []NewMemory, actor, runID string, vectors map[int][]float32, embeddingModel string) []OpResult {
	var results []OpResult
	for attempt := 0; attempt < 4; attempt++ {
		var busy bool
		results, busy = s.addBatchOnce(items, actor, runID, vectors, embeddingModel)
		if !busy || attempt == 3 {
			return results
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	return results
}

// addBatchOnce: mot lan thu ghi lo (xem AddBatch) - tra ket qua + co gap khoa ban ma chua commit.
func (s *Store) addBatchOnce(items []NewMemory, actor, runID string, vectors map[int][]float32, embeddingModel string) ([]OpResult, bool) {
	results := make([]OpResult, 0, len(items))
	tx, err := s.db.BeginWrite()
	if err != nil {
		for _, item := range items {
			results = append(results, OpResult{Event: "SKIPPED", Text: item.Text, Reason: "database: " + err.Error()})
		}
		return results, isBusyError(err)
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
		// user khong gan theo tai lieu; org-style/doc-style gan theo APP (rong = dung chung moi app).
		switch raw.Scope {
		case ScopeUser:
			scopeKey = ""
		case ScopeOrgStyle, ScopeDocStyle:
			scopeKey = NormalizeApp(scopeKey)
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
			// Lenh ghi loi -> bao SKIPPED chu khong bao DUPLICATE khi chua ghi (xem concurrency_test.go);
			// BUSY giua lo -> rollback (defer) roi bao thu lai nhu INSERT.
			if _, err := tx.Exec("UPDATE memories SET hits = hits + 1, last_used_at = ? WHERE id = ?", Now(), existing.ID); err != nil {
				if isBusyError(err) {
					results = append(results, OpResult{Event: "SKIPPED", Text: text, Reason: err.Error()})
					for _, rest := range items[index+1:] {
						results = append(results, OpResult{Event: "SKIPPED", Text: rest.Text, Reason: err.Error()})
					}
					return results, true
				}
				results = append(results, OpResult{Event: "SKIPPED", Text: text, Reason: err.Error()})
				continue
			}
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
		// SAVEPOINT tung muc: mot lenh phu (fts/link/history/vector) loi thi cuon lui RIENG muc do,
		// khong de lai memory thieu chi muc/lich su va khong lam mat cac muc khac trong lo.
		if _, err := tx.Exec("SAVEPOINT memory_item"); err != nil {
			if isBusyError(err) {
				results = append(results, OpResult{Event: "SKIPPED", Text: text, Reason: err.Error()})
				for _, rest := range items[index+1:] {
					results = append(results, OpResult{Event: "SKIPPED", Text: rest.Text, Reason: err.Error()})
				}
				return results, true
			}
			results = append(results, OpResult{Event: "SKIPPED", Text: text, Reason: err.Error()})
			continue
		}
		err = insertMemory(tx, raw, text, scopeKey, hash, actor, runID, id, now, expires, confidence, vector, embeddingModel)
		if err != nil {
			_, _ = tx.Exec("ROLLBACK TO memory_item")
			_, _ = tx.Exec("RELEASE memory_item")
			reason := err.Error()
			if isBusyErrorString(reason) {
				// Khoa ban giua lo: bo cac fact chua xu ly, rollback (defer) roi bao thu lai.
				results = append(results, OpResult{Event: "SKIPPED", Text: text, Reason: reason})
				for _, rest := range items[index+1:] {
					results = append(results, OpResult{Event: "SKIPPED", Text: rest.Text, Reason: reason})
				}
				return results, true
			}
			results = append(results, OpResult{Event: "SKIPPED", Text: text, Reason: reason})
			continue
		}
		if _, err := tx.Exec("RELEASE memory_item"); err != nil {
			results = append(results, OpResult{Event: "SKIPPED", Text: text, Reason: err.Error()})
			continue
		}
		results = append(results, OpResult{Event: "ADD", Text: text, Item: getItem(tx, id)})
	}

	if err := tx.Commit(); err != nil {
		for i := range results {
			results[i] = OpResult{Event: "SKIPPED", Text: results[i].Text, Reason: "commit: " + err.Error()}
		}
		// Da thu commit (trang thai chua ro) thi khong thu lai.
		return results, false
	}
	return results, false
}

// insertMemory ghi tron mot muc moi: memories + FTS + link + history + vector. Loi o bat ky lenh nao
// tra ve de caller ROLLBACK TO savepoint cua muc do.
func insertMemory(tx *sql.Tx, raw NewMemory, text, scopeKey, hash, actor, runID, id, now, expires string,
	confidence *float64, vector []float32, embeddingModel string) error {
	_, err := tx.Exec(`INSERT INTO memories (id, scope, scope_key, text, hash, category, entities_json, expires_at, source,
		  confidence, pinned, hits, created_at, updated_at, created_run_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?, ?)`,
		id, raw.Scope, nullText(scopeKey), text, hash, nullText(raw.Category), entitiesJSON(raw.Entities),
		nullText(expires), actor, confidence, now, now, nullText(runID))
	if err != nil {
		return err
	}

	var rowID int64
	if err := tx.QueryRow("SELECT last_insert_rowid()").Scan(&rowID); err != nil {
		return err
	}
	if err := indexText(tx, rowID, text); err != nil {
		return err
	}
	for _, linked := range distinct(raw.LinkedIDs) {
		if exists(tx, linked) {
			if _, err := tx.Exec("INSERT OR IGNORE INTO memory_links (memory_id, linked_memory_id, created_at) VALUES (?, ?, ?)",
				id, linked, now); err != nil {
				return err
			}
		}
	}
	if err := addHistory(tx, id, "ADD", "", text, actor, runID, ""); err != nil {
		return err
	}
	if len(vector) > 0 && embeddingModel != "" {
		if err := saveVector(tx, id, embeddingModel, vector, hash); err != nil {
			return err
		}
	}
	return nil
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

// Active: ung vien cho ngu canh - chua xoa, chua het han (expires_at > hom qua), gom: memory cua nguoi dung (user,
// dung chung moi app), chuan trinh bay cua APP nay (org-style + doc-style: scope_key = app, hoac rong = dung chung
// moi app), va memory cua TAI LIEU hien tai (document) (12-m3).
func (s *Store) Active(app, documentKey string, today time.Time) []Item {
	yesterday := today.AddDate(0, 0, -1).Format("2006-01-02")
	return readItems(s.db.SQL(), "SELECT "+memoryColumns+" FROM memories WHERE deleted_at IS NULL"+
		" AND (expires_at IS NULL OR expires_at > ?)"+
		" AND (scope = 'user'"+
		" OR ((scope = 'org-style' OR scope = 'doc-style') AND (scope_key IS NULL OR scope_key = ?))"+
		" OR (scope = 'document' AND scope_key = ?))",
		yesterday, NormalizeApp(app), nullText(documentKey))
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
	_ = saveVector(s.db.SQL(), id, model, vector, textHash)
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
	tx, err := s.db.BeginWrite()
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback() }()
	now := Now()
	for _, id := range list {
		// Lenh ghi loi -> rollback (defer), khong commit nua voi (xem concurrency_test.go).
		if _, err := tx.Exec("UPDATE memories SET hits = hits + 1, last_used_at = ? WHERE id = ?", now, id); err != nil {
			return
		}
	}
	_ = tx.Commit()
}

// ---- chi nguoi dung (form / API) ----

// Update: sua text/danh muc/han/ghim. Loi dinh dang -> error de API tra 400.
func (s *Store) Update(id, newText, category, expiresAt string, clearExpires bool, pinned *bool, reason string) (*Item, error) {
	tx, err := s.db.BeginWrite()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	item := getItem(tx, id)
	if item == nil || item.DeletedAt != nil {
		return nil, nil
	}

	// Moi lenh ghi loi -> rollback (defer) va bao loi, khong bao thanh cong khi chua ghi (xem concurrency_test.go).
	now := Now()
	if newText != "" {
		text := strings.TrimSpace(newText)
		if text == "" || len([]rune(text)) > MaxFactLength {
			return nil, fmt.Errorf("text must be 1-%d characters", MaxFactLength)
		}
		if text != item.Text {
			if _, err := tx.Exec("UPDATE memories SET text = ?, hash = ?, updated_at = ? WHERE id = ?", text, Hash(text), now, id); err != nil {
				return nil, err
			}
			if _, err := tx.Exec("DELETE FROM memories_fts WHERE rowid = ?", item.RowID); err != nil {
				return nil, err
			}
			if err := indexText(tx, item.RowID, text); err != nil {
				return nil, err
			}
			if _, err := tx.Exec("DELETE FROM memory_embeddings WHERE memory_id = ?", id); err != nil {
				return nil, err
			}
			if err := addHistory(tx, id, "UPDATE", item.Text, text, ActorUser, "", reason); err != nil {
				return nil, err
			}
		}
	}

	if category != "" {
		if _, err := tx.Exec("UPDATE memories SET category = ?, updated_at = ? WHERE id = ?", category, now, id); err != nil {
			return nil, err
		}
	}

	if clearExpires || expiresAt != "" {
		date := ""
		if !clearExpires {
			date = ValidDate(expiresAt)
			if date == "" {
				return nil, errors.New("expiresAt must be YYYY-MM-DD")
			}
		}
		if _, err := tx.Exec("UPDATE memories SET expires_at = ?, updated_at = ? WHERE id = ?", nullText(date), now, id); err != nil {
			return nil, err
		}
		if err := addHistory(tx, id, "EXPIRES", textOrEmpty(item.ExpiresAt), date, ActorUser, "", reason); err != nil {
			return nil, err
		}
	}

	if pinned != nil && *pinned != item.Pinned {
		if _, err := tx.Exec("UPDATE memories SET pinned = ?, updated_at = ? WHERE id = ?", boolInt(*pinned), now, id); err != nil {
			return nil, err
		}
		event := "UNPIN"
		if *pinned {
			event = "PIN"
		}
		if err := addHistory(tx, id, event, "", "", ActorUser, "", reason); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.Get(id), nil
}

func (s *Store) Delete(id, reason string) bool {
	tx, err := s.db.BeginWrite()
	if err != nil {
		return false
	}
	defer func() { _ = tx.Rollback() }()
	item := getItem(tx, id)
	if item == nil || item.DeletedAt != nil {
		return false
	}
	// Lenh ghi loi -> rollback (defer) va bao that bai, khong bao true khi chua ghi (xem concurrency_test.go).
	now := Now()
	if _, err := tx.Exec("UPDATE memories SET deleted_at = ?, updated_at = ? WHERE id = ?", now, now, id); err != nil {
		return false
	}
	if _, err := tx.Exec("DELETE FROM memories_fts WHERE rowid = ?", item.RowID); err != nil {
		return false
	}
	if err := addHistory(tx, id, "DELETE", item.Text, "", ActorUser, "", reason); err != nil {
		return false
	}
	return tx.Commit() == nil
}

func (s *Store) Restore(id string) bool {
	tx, err := s.db.BeginWrite()
	if err != nil {
		return false
	}
	defer func() { _ = tx.Rollback() }()
	item := getItem(tx, id)
	if item == nil || item.DeletedAt == nil {
		return false
	}
	// Lenh ghi loi -> rollback (defer) va bao that bai, khong bao true khi chua ghi (xem concurrency_test.go).
	if _, err := tx.Exec("UPDATE memories SET deleted_at = NULL, updated_at = ? WHERE id = ?", Now(), id); err != nil {
		return false
	}
	if err := indexText(tx, item.RowID, item.Text); err != nil {
		return false
	}
	if err := addHistory(tx, id, "RESTORE", "", item.Text, ActorUser, "", ""); err != nil {
		return false
	}
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
	tx, err := s.db.BeginWrite()
	if err != nil {
		return 0
	}
	defer func() { _ = tx.Rollback() }()
	filter, args := "", []any{}
	if scope != "" {
		filter, args = " WHERE scope = ?", []any{scope}
	}
	// Moi lenh ghi loi -> rollback (defer) va bao 0, khong bao so dong khi chua xoa het (xem concurrency_test.go).
	if _, err := tx.Exec("DELETE FROM memories_fts WHERE rowid IN (SELECT rowid FROM memories"+filter+")", args...); err != nil {
		return 0
	}
	if _, err := tx.Exec("DELETE FROM memory_history WHERE memory_id IN (SELECT id FROM memories"+filter+")", args...); err != nil {
		return 0
	}
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

// ExportRow: mot memory xuat ra JSON (M4-01) - giu id, ghim, han dung, lien ket de import lai nguyen trang.
type ExportRow struct {
	ID         string   `json:"id"`
	Scope      string   `json:"scope"`
	ScopeKey   string   `json:"scopeKey,omitempty"`
	Text       string   `json:"text"`
	Category   string   `json:"category,omitempty"`
	Entities   []string `json:"entities,omitempty"`
	ExpiresAt  string   `json:"expiresAt,omitempty"`
	Source     string   `json:"source,omitempty"`
	Confidence float64  `json:"confidence"`
	Pinned     bool     `json:"pinned"`
	Hits       int      `json:"hits"`
	CreatedAt  string   `json:"createdAt,omitempty"`
	UpdatedAt  string   `json:"updatedAt,omitempty"`
	Links      []string `json:"links,omitempty"`
}

// Export: toan bo memory chua xoa (M4-01).
func (s *Store) Export() []ExportRow {
	rows := []ExportRow{}
	for _, item := range s.List("", "", false, "", 1_000_000) {
		scopeKey := ""
		if item.ScopeKey != nil {
			scopeKey = *item.ScopeKey
		}
		confidence := 0.0
		if item.Confidence != nil {
			confidence = *item.Confidence
		}
		rows = append(rows, ExportRow{
			ID: item.ID, Scope: item.Scope, ScopeKey: scopeKey, Text: item.Text, Category: textOrEmpty(item.Category),
			Entities: item.Entities, ExpiresAt: textOrEmpty(item.ExpiresAt), Source: item.Source,
			Confidence: confidence, Pinned: item.Pinned, Hits: item.Hits, CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt, Links: item.Links,
		})
	}
	return rows
}

// ImportResult: so lieu mot lan import.
type ImportResult struct {
	Added   int
	Skipped int
	Links   int
}

// Import: ghi lai memory tu Export (M4-01), giu id/pinned/hits/han dung; chong trung theo hash; lien ket chi toi
// memory ton tai. Embedding tinh lai sau (Service.QueueEmbeddingBackfill). Ban trung/khong hop le bi bo qua.
func (s *Store) Import(rows []ExportRow) (ImportResult, error) {
	result := ImportResult{}
	tx, err := s.db.BeginWrite()
	if err != nil {
		return result, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	now := Now()
	for _, row := range rows {
		text := strings.TrimSpace(row.Text)
		if text == "" || len([]rune(text)) > MaxFactLength || !IsValidScope(row.Scope) {
			result.Skipped++
			continue
		}
		scopeKey := row.ScopeKey
		switch row.Scope {
		case ScopeUser:
			scopeKey = ""
		case ScopeOrgStyle, ScopeDocStyle:
			scopeKey = NormalizeApp(scopeKey)
		}
		hash := Hash(text)
		if findByHash(tx, row.Scope, scopeKey, hash) != nil {
			result.Skipped++
			continue
		}
		id := row.ID
		if id == "" {
			id = NewID()
		}
		created := row.CreatedAt
		if created == "" {
			created = now
		}
		updated := row.UpdatedAt
		if updated == "" {
			updated = created
		}
		source := row.Source
		if source == "" {
			source = "import"
		}
		if _, err := tx.Exec(`INSERT INTO memories (id, scope, scope_key, text, hash, category, entities_json, expires_at, source,
			  confidence, pinned, hits, created_at, updated_at, created_run_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
			id, row.Scope, nullText(scopeKey), text, hash, nullText(row.Category), entitiesJSON(row.Entities),
			nullText(ValidDate(row.ExpiresAt)), source, row.Confidence, boolInt(row.Pinned), row.Hits,
			created, updated); err != nil {
			if isBusyError(err) {
				return result, err
			}
			result.Skipped++
			continue
		}
		var rowID int64
		if err := tx.QueryRow("SELECT last_insert_rowid()").Scan(&rowID); err != nil {
			return result, err
		}
		if err := indexText(tx, rowID, text); err != nil {
			return result, err
		}
		for _, linked := range distinct(row.Links) {
			if !exists(tx, linked) {
				continue
			}
			if _, err := tx.Exec("INSERT OR IGNORE INTO memory_links (memory_id, linked_memory_id, created_at) VALUES (?, ?, ?)",
				id, linked, now); err != nil {
				return result, err
			}
			result.Links++
		}
		if err := addHistory(tx, id, "IMPORT", "", text, ActorUser, "", "import"); err != nil {
			return result, err
		}
		result.Added++
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	committed = true
	return result, nil
}

// SoftDeleteByScopeKey: xoa mem moi memory cua mot TAI LIEU (M4-02) - chi scope `document`, khong dung cham memory
// khac (org-style/doc-style khoa bang app cung khong bi dong toi du trung chuoi).
func (s *Store) SoftDeleteByScopeKey(scopeKey, reason string) int {
	tx, err := s.db.BeginWrite()
	if err != nil {
		return 0
	}
	defer func() { _ = tx.Rollback() }()
	items := readItems(tx, "SELECT "+memoryColumns+" FROM memories WHERE deleted_at IS NULL AND scope = ? AND scope_key = ?",
		ScopeDocument, nullText(scopeKey))
	now := Now()
	for _, item := range items {
		if _, err := tx.Exec("UPDATE memories SET deleted_at = ?, updated_at = ? WHERE id = ?", now, now, item.ID); err != nil {
			return 0
		}
		if _, err := tx.Exec("DELETE FROM memories_fts WHERE rowid = ?", item.RowID); err != nil {
			return 0
		}
		if err := addHistory(tx, item.ID, "DELETE", item.Text, "", ActorUser, "", reason); err != nil {
			return 0
		}
	}
	if tx.Commit() != nil {
		return 0
	}
	return len(items)
}

// CleanupStats: so lieu mot lan don dinh ky (M5-01).
type CleanupStats struct {
	Merged  int
	Expired int
	Purged  int
}

// Cleanup: don dinh ky - gop fact gan trung da lot qua luc ghi, xoa mem fact het han, don ban xoa mem cu.
// Khong dung cham org-style/doc-style (chuan chung + kieu trinh bay do nguoi dat - M5-02).
func (s *Store) Cleanup(now time.Time) CleanupStats {
	return CleanupStats{
		Merged:  s.mergeNearDuplicates(now),
		Expired: s.softDeleteExpired(now),
		Purged:  s.PurgeSoftDeleted(now),
	}
}

// vectors: embedding hien co cua moi memory chua xoa (de so cap trong cung nhom).
func (s *Store) vectors() map[string][]float32 {
	result := map[string][]float32{}
	rows, err := s.db.SQL().Query("SELECT e.memory_id, e.vector FROM memory_embeddings e " +
		"JOIN memories m ON m.id = e.memory_id WHERE m.deleted_at IS NULL")
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var blob []byte
		if rows.Scan(&id, &blob) == nil {
			result[id] = fromBlob(blob)
		}
	}
	return result
}

// mergeNearDuplicates: trong cung (scope, scope_key), fact giong cung hash hoac cosine >= NearDuplicateCosine thi
// gop vao ban CU hon (hoac ban ghim): giu ban cu, link ban sau vao ban cu, xoa mem ban sau (ly do "merge").
func (s *Store) mergeNearDuplicates(now time.Time) int {
	items := s.List("", "", false, "", 1_000_000)
	vectors := s.vectors()
	groups := map[string][]Item{}
	order := []string{}
	for _, item := range items {
		if item.Scope == ScopeOrgStyle || item.Scope == ScopeDocStyle {
			// chuan chung / kieu trinh bay do nguoi dat: khong gop tu dong.
			continue
		}
		key := item.Scope + "|" + textOrEmpty(item.ScopeKey)
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], item)
	}

	tx, err := s.db.BeginWrite()
	if err != nil {
		return 0
	}
	defer func() { _ = tx.Rollback() }()
	merged := 0
	for _, key := range order {
		group := groups[key]
		sort.SliceStable(group, func(a, b int) bool { return group[a].CreatedAt < group[b].CreatedAt })
		kept := []Item{}
		for _, item := range group {
			target := ""
			for _, old := range kept {
				if Normalize(old.Text) == Normalize(item.Text) {
					target = old.ID
					break
				}
				oldVector, hasOld := vectors[old.ID]
				vector, hasNew := vectors[item.ID]
				if hasOld && hasNew && Cosine(oldVector, vector) >= NearDuplicateCosine {
					target = old.ID
					break
				}
			}
			if target == "" {
				kept = append(kept, item)
				continue
			}
			if _, err := tx.Exec("INSERT OR IGNORE INTO memory_links (memory_id, linked_memory_id, created_at) VALUES (?, ?, ?)",
				item.ID, target, now); err != nil {
				return 0
			}
			if _, err := tx.Exec("UPDATE memories SET deleted_at = ?, updated_at = ? WHERE id = ?", now, now, item.ID); err != nil {
				return 0
			}
			if _, err := tx.Exec("DELETE FROM memories_fts WHERE rowid = ?", item.RowID); err != nil {
				return 0
			}
			if err := addHistory(tx, item.ID, "DELETE", item.Text, "", ActorAgent, "", "merge"); err != nil {
				return 0
			}
			merged++
		}
	}
	if tx.Commit() != nil {
		return 0
	}
	return merged
}

// softDeleteExpired: fact da qua han (expires_at <= hom qua) thi xoa mem; treo lai de cuu bang restore. Bo qua
// org-style/doc-style (chuan trinh bay do nguoi dat khong tu het han).
func (s *Store) softDeleteExpired(now time.Time) int {
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	items := readItems(s.db.SQL(), "SELECT "+memoryColumns+" FROM memories WHERE deleted_at IS NULL "+
		"AND scope NOT IN ('org-style','doc-style') AND expires_at IS NOT NULL AND expires_at <= ?", yesterday)
	if len(items) == 0 {
		return 0
	}
	tx, err := s.db.BeginWrite()
	if err != nil {
		return 0
	}
	defer func() { _ = tx.Rollback() }()
	stamp := Now()
	for _, item := range items {
		if _, err := tx.Exec("UPDATE memories SET deleted_at = ?, updated_at = ? WHERE id = ?", stamp, stamp, item.ID); err != nil {
			return 0
		}
		if _, err := tx.Exec("DELETE FROM memories_fts WHERE rowid = ?", item.RowID); err != nil {
			return 0
		}
		if err := addHistory(tx, item.ID, "DELETE", item.Text, "", ActorAgent, "", "expired"); err != nil {
			return 0
		}
	}
	if tx.Commit() != nil {
		return 0
	}
	return len(items)
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

func indexText(db dbHandle, rowID int64, text string) error {
	_, err := db.Exec("INSERT INTO memories_fts (rowid, text) VALUES (?, ?)", rowID, Normalize(text))
	return err
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

func saveVector(db dbHandle, id, model string, vector []float32, textHash string) error {
	_, err := db.Exec(`INSERT INTO memory_embeddings (memory_id, model, dim, vector, text_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(memory_id) DO UPDATE SET model = ?, dim = ?, vector = ?, text_hash = ?, created_at = ?`,
		id, model, len(vector), toBlob(vector), textHash, Now(), model, len(vector), toBlob(vector), textHash, Now())
	return err
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

func addHistory(db dbHandle, id, event, oldText, newText, actor, runID, reason string) error {
	_, err := db.Exec(`INSERT INTO memory_history (memory_id, event, old_text, new_text, actor, run_id, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, event, nullText(oldText), nullText(newText), actor, nullText(runID),
		nullText(reason), Now())
	return err
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
