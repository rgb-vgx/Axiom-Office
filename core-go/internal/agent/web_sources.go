package agent

import (
	"os"
	"sync/atomic"

	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/store"
	"axiomoffice/core/internal/tools"
)

// webSourcesSaver: cau noi RunContext (tools) -> store.WebSources. Loi ghi kho khong bao gio noi len
// luot chay (nguon chi la du lieu phu). Dinh ky kiem tra tran dung luong: core.db phinh qua nguong thi
// don nguon cu nhat (Purge) - tran mac dinh 300MB (config WebSourceMaxMB).
type webSourcesSaver struct {
	sources  *store.WebSources
	dbPath   string
	maxBytes int64
	saves    atomic.Int64
}

func newWebSourcesSaver(sources *store.WebSources, dbPath string, maxMB int) *webSourcesSaver {
	saver := &webSourcesSaver{sources: sources, dbPath: dbPath}
	if maxMB > 0 {
		saver.maxBytes = int64(maxMB) << 20
	}
	return saver
}

func (s *webSourcesSaver) SaveWebSources(runID, conversationID, kind, query string, docs []tools.WebSourceDoc) int {
	converted := make([]store.WebSourceDoc, 0, len(docs))
	for _, doc := range docs {
		converted = append(converted, store.WebSourceDoc{
			URL: doc.URL, Title: doc.Title, Lang: doc.Lang, Mode: doc.Mode,
			Content: doc.Content, Snippet: doc.Snippet, Score: doc.Score, OK: doc.OK, Error: doc.Error,
		})
	}
	saved := s.sources.SaveDocs(runID, conversationID, kind, query, converted)
	// Kiem tra tran moi 250 lan luu (stat file re): chi khi core.db da vuot tran moi don.
	if s.maxBytes > 0 && s.saves.Add(1)%250 == 0 {
		s.purgeIfOversize()
	}
	return saved
}

func (s *webSourcesSaver) purgeIfOversize() {
	info, err := os.Stat(s.dbPath)
	if err != nil || info.Size() <= s.maxBytes {
		return
	}
	if purged := s.sources.Purge(0, s.maxBytes); purged > 0 {
		corelog.Info("web sources: store over %d MB - purged %d oldest source(s)", s.maxBytes>>20, purged)
	}
}
