package api

import (
	"net/http"
	"strings"

	"axiomoffice/core/internal/mcp"
)

// GET /v1/mcp: server da cau hinh (khong in env/headers vi co the chua key) + loi khoi dong (muc 7.3, 8.7).
func mapMCP(mux *http.ServeMux, deps *Deps) {
	mux.HandleFunc("GET /v1/mcp", func(w http.ResponseWriter, r *http.Request) {
		bound := deps.MCP.Tools(r.Context())
		errors := deps.MCP.Errors()

		servers := []any{}
		for _, config := range deps.MCP.Configs() {
			names := []string{}
			prefix := mcp.ToolName(config.Name, "")
			for _, tool := range bound {
				if strings.HasPrefix(tool.Name(), prefix) {
					names = append(names, tool.Name())
				}
			}
			transport := "stdio"
			if config.URL != "" {
				transport = "http"
			}
			var problem any
			if message, ok := errors[config.Name]; ok {
				problem = message
			}
			servers = append(servers, map[string]any{
				"name": config.Name, "transport": transport, "trusted": config.Trusted, "builtIn": config.BuiltIn,
				"tools": names, "error": problem,
			})
		}
		OK(w, map[string]any{"servers": servers})
	})
}
