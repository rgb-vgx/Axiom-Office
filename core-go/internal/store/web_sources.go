package store

import (
	"database/sql"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"axiomoffice/core/internal/textutil"
)

// WebSources: kho nguon web tai ve trong cac luot truoc (trich dan lai, khoi tai lai). web_sources giu
// mot dong / URL (url_key chong trung), web_source_refs ghi lich su dung theo luot, web_sources_fts
// de tim lai bang tu khoa (bo dau). Chi Core ghi; moi loi ghi kho deu bi bo qua (khong lam hong luot chay).
type WebSources struct{ db *DB }

func (d *DB) WebSources() *WebSources { return &WebSources{db: d} }

// Path: duong dan file core.db (agent dung de kiem tra tran dung luong kho).
func (w *WebSources) Path() string { return w.db.Path }

// WebSourceDoc: mot nguon web trong ket qua tool (parse tu trailer AXIOM_WEB hoac tu tool tavily_search).
type WebSourceDoc struct {
	URL     string
	Title   string
	Lang    string // "vi" | "en" | ""
	Mode    string // search | lead | full | fetch | fail
	Content string // markdown da doc (co the rong khi chi co snippet)
	Snippet string // doan trich khi chua doc noi dung
	Score   int
	OK      bool
	Error   string
}

// WebSourceRow: mot dong trong kho (khong kem content).
type WebSourceRow struct {
	ID           int64
	URL          string
	URLKey       string
	Domain       string
	Title        string
	Lang         string
	ContentChars int
	FetchCount   int
	FirstSeenAt  string
	FetchedAt    string
}

// WebSourceHit: ket qua tim trong kho (kem doan trich quanh tu khoa).
type WebSourceHit struct {
	WebSourceRow
	Snippet string
}

// SaveDocs: upsert tung nguon theo url_key roi ghi mot ref (lich su dung) - ca hai trong mot transaction.
// Quy tac gop: tieu de/lang moi nhat khong rong thang; content: ban day du moi thay the, snippet chi dien
// khi dong con trong (khong de ket qua search 500 ky tu de mat ban doc day du). Loi giua duong -> rollback
// toan bo (luot chay khong bao gio hong vi viec luu kho). Tra ve so nguon ghi duoc.
func (w *WebSources) SaveDocs(runID, conversationID, kind, query string, docs []WebSourceDoc) int {
	if w == nil || len(docs) == 0 {
		return 0
	}
	tx, err := w.db.BeginWrite()
	if err != nil {
		return 0
	}
	defer func() { _ = tx.Rollback() }()

	now := Now()
	saved := 0
	for _, doc := range docs {
		key := urlKey(doc.URL)
		if key == "" {
			continue
		}
		url := strings.TrimSpace(doc.URL)
		domain := domainOf(url)
		title := strings.TrimSpace(doc.Title)
		lang := strings.TrimSpace(doc.Lang)
		content := strings.TrimSpace(doc.Content)
		realContent := content != ""
		if !realContent {
			content = strings.TrimSpace(doc.Snippet)
		}

		var (
			id         int64
			oldTitle   sql.NullString
			oldLang    sql.NullString
			oldContent sql.NullString
		)
		err := tx.QueryRow("SELECT id, title, lang, content FROM web_sources WHERE url_key = ?", key).
			Scan(&id, &oldTitle, &oldLang, &oldContent)
		switch {
		case err == sql.ErrNoRows:
			if _, err := tx.Exec(`INSERT INTO web_sources
					(url, url_key, domain, title, lang, content, content_chars, search_text, fetch_count, first_seen_at, fetched_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
				url, key, domain, title, lang, content, len(content),
				textutil.Normalize(title+"\n"+content), now, now); err != nil {
				continue
			}
			if err := tx.QueryRow("SELECT id FROM web_sources WHERE url_key = ?", key).Scan(&id); err != nil {
				continue
			}
		case err == nil:
			if title == "" {
				title = oldTitle.String
			}
			if lang == "" {
				lang = oldLang.String
			}
			if !realContent && strings.TrimSpace(oldContent.String) != "" {
				content = oldContent.String
			}
			if _, err := tx.Exec(`UPDATE web_sources SET url = ?, domain = ?, title = ?, lang = ?,
					content = ?, content_chars = ?, search_text = ?, fetch_count = fetch_count + 1, fetched_at = ?
				WHERE id = ?`,
				url, domain, title, lang, content, len(content),
				textutil.Normalize(title+"\n"+content), now, id); err != nil {
				continue
			}
		default:
			continue
		}

		okFlag := 1
		if !doc.OK && strings.TrimSpace(doc.Error) != "" {
			okFlag = 0
		}
		if _, err := tx.Exec(`INSERT INTO web_source_refs
				(source_id, conversation_id, run_id, kind, query, mode, score, ok, error, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, nullIfEmpty(conversationID), nullIfEmpty(runID), nullIfEmpty(kind),
			nullIfEmpty(cutRunes(query, 500)), nullIfEmpty(doc.Mode), doc.Score, okFlag,
			nullIfEmpty(cutRunes(doc.Error, 300)), now); err != nil {
			continue
		}
		saved++
	}
	if err := tx.Commit(); err != nil {
		return 0
	}
	return saved
}

// Search: tim trong kho bang tu khoa (FTS5 tren cot search_text da bo dau - dung textutil.Normalize,
// SQLite unicode61 khong tu bo dau tieng Viet). Truy van rong -> tra nguon moi nhat. limit <= 0 -> 8.
func (w *WebSources) Search(query string, limit int) []WebSourceHit {
	if w == nil {
		return nil
	}
	if limit <= 0 || limit > 50 {
		limit = 8
	}
	terms := ftsTerms(textutil.Normalize(query))
	if len(terms) == 0 {
		return w.Recent(limit)
	}
	// AND tat ca tu khoa (thua con hon nhieu); model tu thu lai voi tu khoa khac khi khong thay.
	return w.searchQuery(ftsMatch(terms, " AND "), limit)
}

func (w *WebSources) searchQuery(match string, limit int) []WebSourceHit {
	rows, err := w.db.sql.Query(`SELECT ws.id, ws.url, ws.url_key, COALESCE(ws.domain,''), COALESCE(ws.title,''),
			COALESCE(ws.lang,''), ws.content_chars, ws.fetch_count, ws.first_seen_at, ws.fetched_at,
			substr(COALESCE(ws.content,''), 1, 300)
		FROM web_sources_fts JOIN web_sources ws ON ws.id = web_sources_fts.rowid
		WHERE web_sources_fts MATCH ? ORDER BY rank LIMIT ?`, match, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanHits(rows)
}

// RecentByConversation: cac nguon dung gan nhat cua mot hoi thoai (theo ref).
func (w *WebSources) RecentByConversation(conversationID string, limit int) []WebSourceHit {
	if w == nil || strings.TrimSpace(conversationID) == "" {
		return nil
	}
	if limit <= 0 || limit > 50 {
		limit = 8
	}
	rows, err := w.db.sql.Query(`SELECT ws.id, ws.url, ws.url_key, COALESCE(ws.domain,''), COALESCE(ws.title,''),
			COALESCE(ws.lang,''), ws.content_chars, ws.fetch_count, ws.first_seen_at, ws.fetched_at,
			substr(COALESCE(ws.content,''), 1, 300)
		FROM web_source_refs r JOIN web_sources ws ON ws.id = r.source_id
		WHERE r.conversation_id = ?
		GROUP BY ws.id ORDER BY MAX(r.id) DESC LIMIT ?`, conversationID, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanHits(rows)
}

// Recent: cac nguon moi nhat trong kho (moi hoi thoai).
func (w *WebSources) Recent(limit int) []WebSourceHit {
	if w == nil {
		return nil
	}
	if limit <= 0 || limit > 50 {
		limit = 8
	}
	rows, err := w.db.sql.Query(`SELECT ws.id, ws.url, ws.url_key, COALESCE(ws.domain,''), COALESCE(ws.title,''),
			COALESCE(ws.lang,''), ws.content_chars, ws.fetch_count, ws.first_seen_at, ws.fetched_at,
			substr(COALESCE(ws.content,''), 1, 300)
		FROM web_sources ws ORDER BY ws.fetched_at DESC, ws.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanHits(rows)
}

// Get: doc mot nguon theo id (so) hoac URL (chuan hoa url_key). Tra ve dong + content day du.
func (w *WebSources) Get(idOrURL string) (*WebSourceRow, string) {
	if w == nil {
		return nil, ""
	}
	value := strings.TrimSpace(idOrURL)
	if value == "" {
		return nil, ""
	}
	row := w.db.sql.QueryRow(`SELECT id, url, url_key, COALESCE(domain,''), COALESCE(title,''), COALESCE(lang,''),
			content_chars, fetch_count, first_seen_at, fetched_at, COALESCE(content,'')
		FROM web_sources WHERE id = ? OR url_key = ? LIMIT 1`, numericID(value), urlKey(value))
	var (
		result  WebSourceRow
		content string
	)
	if row.Scan(&result.ID, &result.URL, &result.URLKey, &result.Domain, &result.Title, &result.Lang,
		&result.ContentChars, &result.FetchCount, &result.FirstSeenAt, &result.FetchedAt, &content) != nil {
		return nil, ""
	}
	return &result, content
}

// Bytes: tong dung luong content trong kho (byte) - de chan tran kich thuoc.
func (w *WebSources) Bytes() int64 {
	if w == nil {
		return 0
	}
	var total sql.NullInt64
	if err := w.db.sql.QueryRow(
		`SELECT SUM(LENGTH(CAST(COALESCE(content,'') AS BLOB))) FROM web_sources`).Scan(&total); err != nil {
		return 0
	}
	return total.Int64
}

// Count: so nguon trong kho.
func (w *WebSources) Count() int {
	if w == nil {
		return 0
	}
	var total int
	if err := w.db.sql.QueryRow(`SELECT COUNT(*) FROM web_sources`).Scan(&total); err != nil {
		return 0
	}
	return total
}

// Purge: don kho. keepDays > 0: xoa nguon khong con duoc dung tu moc do (ref cu hon). maxBytes > 0:
// xoa nguon CU NHAT truoc cho den khi dung luong content ve duoi 90% tran. Tra ve so nguon da xoa.
func (w *WebSources) Purge(keepDays int, maxBytes int64) int {
	if w == nil {
		return 0
	}
	removed := 0
	if keepDays > 0 {
		cutoff := timeAgo(keepDays)
		result, err := w.db.sql.Exec(`DELETE FROM web_sources WHERE fetched_at < ? AND id NOT IN
			(SELECT source_id FROM web_source_refs WHERE created_at >= ?)`, cutoff, cutoff)
		if err == nil {
			if affected, err := result.RowsAffected(); err == nil {
				removed += int(affected)
			}
		}
	}
	if maxBytes > 0 {
		removed += w.purgeOldestUntil(maxBytes * 9 / 10)
	}
	return removed
}

// purgeOldestUntil: xoa nguon cu nhat dan cho den khi tong content <= target (0 xoa khi da duoi).
func (w *WebSources) purgeOldestUntil(target int64) int {
	type entry struct {
		id    int64
		bytes int64
	}
	rows, err := w.db.sql.Query(
		`SELECT id, LENGTH(CAST(COALESCE(content,'') AS BLOB)) FROM web_sources ORDER BY fetched_at ASC, id ASC`)
	if err != nil {
		return 0
	}
	total := int64(0)
	list := []entry{}
	for rows.Next() {
		var item entry
		if rows.Scan(&item.id, &item.bytes) == nil {
			list = append(list, item)
			total += item.bytes
		}
	}
	rows.Close()
	if total <= target {
		return 0
	}
	removed := 0
	for _, item := range list {
		if total <= target {
			break
		}
		if result, err := w.db.sql.Exec("DELETE FROM web_sources WHERE id = ?", item.id); err == nil {
			if affected, err := result.RowsAffected(); err == nil && affected > 0 {
				total -= item.bytes
				removed += int(affected)
			}
		}
	}
	return removed
}

func scanHits(rows *sql.Rows) []WebSourceHit {
	hits := []WebSourceHit{}
	for rows.Next() {
		var hit WebSourceHit
		if rows.Scan(&hit.ID, &hit.URL, &hit.URLKey, &hit.Domain, &hit.Title, &hit.Lang,
			&hit.ContentChars, &hit.FetchCount, &hit.FirstSeenAt, &hit.FetchedAt, &hit.Snippet) == nil {
			hits = append(hits, hit)
		}
	}
	return hits
}

// urlKey: khoa chong trung URL: bo fragment + tham so theo doi (utm_*, fbclid, gclid), bo scheme/www,
// host viet thuong, bo dau "/" cuoi. Manh hon tools.normalizeURL (dung rieng cho kiem tra nguon slide).
func urlKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		trimmed := strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
		return strings.ToLower(strings.TrimRight(trimmed, "/"))
	}
	parsed.Fragment = ""
	values := parsed.Query()
	for name := range values {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" {
			values.Del(name)
		}
	}
	parsed.RawQuery = values.Encode()
	key := strings.ToLower(strings.TrimPrefix(parsed.Host, "www.")) + strings.TrimRight(parsed.Path, "/")
	if parsed.RawQuery != "" {
		key += "?" + parsed.RawQuery
	}
	return key
}

func domainOf(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
}

// ftsTerms: tach tu khoa, bo ky tu dieu khien FTS (chi giu chu/so/_ - giu dau tieng Viet vi FTS bo dau khi index).
func ftsTerms(query string) []string {
	terms := []string{}
	for _, part := range strings.Fields(query) {
		cleaned := strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
				return r
			case r > 127:
				return r
			}
			return -1
		}, part)
		if cleaned != "" {
			terms = append(terms, cleaned)
		}
	}
	return terms
}

// ftsMatch: ghep cac tu thanh truy van FTS5 an toan (boc ngoac kep tung tu - khong phai cu phap FTS).
func ftsMatch(terms []string, join string) string {
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		quoted = append(quoted, `"`+strings.ReplaceAll(term, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, join)
}

func numericID(value string) int64 {
	var id int64
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0
		}
		id = id*10 + int64(r-'0')
	}
	return id
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

// cutRunes: cat chuoi theo so ky tu (rune) giu UTF-8 hop le.
func cutRunes(value string, max int) string {
	if len(value) <= max || utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}

// timeAgo: moc thoi gian cach day days ngay, cung dinh dang Now().
func timeAgo(days int) string {
	return time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02T15:04:05.000Z")
}
