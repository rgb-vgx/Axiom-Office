package api

import (
	"math"
	"net/http"
	"time"

	"axiomoffice/core/internal/corelog"
)

func mapCore(mux *http.ServeMux, deps *Deps) {
	// Khong can token: add-in va script dung de kiem tra Core con song.
	health := func(w http.ResponseWriter, r *http.Request) {
		var memoryError any
		if deps.Stores.Error != "" {
			memoryError = deps.Stores.Error
		}
		OK(w, map[string]any{
			"app":           "core",
			"pid":           pid(),
			"port":          deps.Runtime.Port,
			"version":       deps.Runtime.Version,
			"protocol":      Protocol,
			"started":       deps.Runtime.StartedText(),
			"uptimeSeconds": math.Round(time.Since(deps.Runtime.Started).Seconds()*10) / 10,
			"dataDir":       deps.Paths.Root,
			"memory":        deps.Stores.Status(deps.Config.MemoryEnabled),
			"memoryError":   memoryError,
			"provider":      deps.Config.LlmProvider,
			"model":         deps.Config.LlmModel,
		})
	}
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /health/", health)

	// Dung boi build.ps1/uninstall.ps1/install.sh truoc khi ghi de file.
	mux.HandleFunc("POST /v1/admin/shutdown", func(w http.ResponseWriter, r *http.Request) {
		corelog.Info("shutdown requested via API")
		OK(w, map[string]any{"stopping": true})
		go func() {
			// Cho response di xong roi moi dung (dung ngay thi client co the nhan loi ket noi).
			time.Sleep(150 * time.Millisecond)
			if deps.Stop != nil {
				deps.Stop()
			}
		}()
	})
}
