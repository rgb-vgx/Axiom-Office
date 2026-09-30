package api

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/setup"
)

// API cho wizard thiet lap (SetupWizardForm.cs cua add-in Windows va setupwizard.py cua extension LibreOffice):
//
//	GET  /v1/setup       preset nha cung cap + mo ta tinh nang + tinh trang hien tai cua Core
//	POST /v1/llm/test    thu ket noi voi gia tri NGUOI DUNG VUA NHAP (chua luu) -> loi da dich sang tieng Viet
//	GET  /v1/llm/models  danh sach model cua may chu de nguoi dung chon thay vi tu go ten
//
// KHONG ghi khoa API vao log.
const (
	testSystemPrompt = "Bạn là trợ lý kiểm tra kết nối. Trả lời cực ngắn."
	testUserPrompt   = "Trả lời đúng một từ: OK"
	testTimeout      = 30 * time.Second
	testMaxTokens    = 24
)

func mapSetup(mux *http.ServeMux, deps *Deps) {
	mux.HandleFunc("GET /v1/setup", func(w http.ResponseWriter, r *http.Request) {
		OK(w, setupPayload(deps))
	})

	mux.HandleFunc("POST /v1/llm/test", func(w http.ResponseWriter, r *http.Request) {
		body, parsed := readJSON(r)
		if !parsed {
			Error(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		request := adHocFrom(func(name string) string {
			if value, ok := body[name].(string); ok {
				return value
			}
			return ""
		})
		request.needsModel = true
		if missing := request.missing(); missing != "" {
			OK(w, errorJSON(setup.Describe(missing, request.providerID, request.endpoint, request.model)))
			return
		}

		apiKey := request.apiKey
		if apiKey == "" {
			apiKey = deps.Config.LlmApiKey
		}
		client := model.NewClient(deps.HTTP, request.codec, request.endpoint, apiKey, request.model)
		client.RequestTimeout = testTimeout
		client.RetryDelays = nil

		started := time.Now()
		text, errText, ok := client.Chat(r.Context(), testSystemPrompt, testUserPrompt, testMaxTokens)
		seconds := time.Since(started).Seconds()
		if !ok {
			failure := setup.Describe(errText, request.providerID, request.endpoint, request.model)
			corelog.Info("setup: llm test failed kind=%s provider=%s model=%s detail=%s", failure.Kind, request.codec, request.model, failure.Detail)
			OK(w, errorJSON(failure))
			return
		}
		reply := strings.TrimSpace(text)
		if reply == "" {
			reply = "OK"
		}
		// Ban .NET lam tron 1 chu so roi moi tra 2 chu so: giu y nguyen.
		rounded := math.Round(seconds*10) / 10
		corelog.Info("setup: llm test ok provider=%s model=%s seconds=%s", request.codec, request.model, strconv.FormatFloat(rounded, 'f', 1, 64))
		if runes := []rune(reply); len(runes) > 200 {
			reply = string(runes[:200])
		}
		OK(w, map[string]any{"seconds": rounded, "model": request.model, "provider": request.codec, "reply": reply})
	})

	mux.HandleFunc("GET /v1/llm/models", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		request := adHocFrom(query.Get)
		if missing := request.missing(); missing != "" {
			OK(w, errorJSON(setup.Describe(missing, request.providerID, request.endpoint, request.model)))
			return
		}
		models, errText := model.ListModels(r.Context(), deps.HTTP, request.codec, request.endpoint, request.apiKey, 0)
		if models == nil {
			failure := setup.Describe(errText, request.providerID, request.endpoint, request.model)
			corelog.Info("setup: list models failed kind=%s detail=%s", failure.Kind, failure.Detail)
			OK(w, errorJSON(failure))
			return
		}
		corelog.Info("setup: list models ok provider=%s count=%d", request.codec, len(models))
		OK(w, map[string]any{"models": models, "count": len(models)})
	})
}

func errorJSON(failure setup.LlmError) map[string]any {
	return map[string]any{"kind": failure.Kind, "message": failure.Message, "hint": failure.Hint, "detail": failure.Detail}
}

// adHoc: tham so mot lan thu (body JSON cua POST hoac query cua GET). apiKey rong = dung khoa da luu.
type adHoc struct {
	providerID, codec, endpoint, model, apiKey string
	needsModel                                 bool
}

func adHocFrom(read func(string) string) adHoc {
	endpoint := strings.TrimSpace(read("endpoint"))
	codec := strings.TrimSpace(read("provider"))
	providerID := strings.TrimSpace(read("providerId"))
	if providerID == "" {
		providerID = setup.GuessProviderID(endpoint, codec)
	}
	if codec == "" {
		codec = "openai"
		if provider := setup.FindProvider(providerID); provider != nil {
			codec = provider.Codec
		}
	}
	return adHoc{
		providerID: providerID,
		codec:      codec,
		endpoint:   endpoint,
		model:      strings.TrimSpace(read("model")),
		apiKey:     strings.TrimSpace(read("apiKey")),
	}
}

func (a adHoc) missing() string {
	switch {
	case a.endpoint == "":
		return "Endpoint is not configured"
	case a.needsModel && a.model == "":
		return "Model is not configured"
	}
	return ""
}

func setupPayload(deps *Deps) map[string]any {
	cfg := deps.Config
	presets := make([]any, 0, len(setup.Data.Providers))
	for _, provider := range setup.Data.Providers {
		presets = append(presets, map[string]any{
			"id":              provider.ID,
			"label":           provider.Label,
			"description":     provider.Description,
			"endpoint":        provider.Endpoint,
			"needsKey":        provider.NeedsKey,
			"keyUrl":          provider.KeyURL,
			"codec":           provider.Codec,
			"suggestedModels": provider.SuggestedModels,
		})
	}
	steps := make([]any, 0, len(setup.Data.Steps))
	for _, step := range setup.Data.Steps {
		steps = append(steps, map[string]any{"id": step.ID, "title": step.Title, "subtitle": step.Subtitle})
	}
	features := make([]any, 0, len(setup.Data.Features))
	for _, feature := range setup.Data.Features {
		features = append(features, map[string]any{
			"key": feature.Key, "label": feature.Label, "description": feature.Description, "recommended": feature.Recommended,
		})
	}
	checks := make([]string, 0, len(setup.Data.Checks))
	for _, check := range setup.Data.Checks {
		checks = append(checks, check.ID)
	}

	// problems: viec can lam, wizard hien o buoc "Kiem tra may" (id khop catalog/setup.json).
	problems := []any{}
	if cfg.LlmEndpoint == "" || cfg.LlmModel == "" {
		problems = append(problems, problem("config", "error",
			"Chưa cấu hình AI", "Mở bước \"Kết nối máy chủ AI\" để nhập địa chỉ máy chủ và model."))
	}
	if cfg.Token == "" {
		problems = append(problems, problem("token", "warning",
			"Chưa có khoá bảo vệ giữa ứng dụng và Agent Core", "Bấm \"Tạo khoá mới\" rồi khởi động lại Core."))
	}
	if !deps.Stores.Available {
		hint := deps.Stores.Error
		if hint == "" {
			hint = "Kiểm tra ổ đĩa và thư mục dữ liệu."
		}
		problems = append(problems, problem("memory", "warning", "Không mở được dữ liệu ghi nhớ", hint))
	}

	skills, skillErrors := 0, 0
	if deps.Skills != nil {
		skills, skillErrors = deps.Skills()
	}
	return map[string]any{
		"version":    deps.Runtime.Version,
		"problems":   problems,
		"presets":    presets,
		"steps":      steps,
		"features":   features,
		"checks":     checks,
		"providerId": setup.GuessProviderID(cfg.LlmEndpoint, cfg.LlmProvider),
		"current":    currentJSON(cfg),
		"core": map[string]any{
			"pid":         pid(),
			"port":        deps.Runtime.Port,
			"dataDir":     deps.Paths.Root,
			"logFile":     deps.Paths.LogFile,
			"skills":      skills,
			"skillErrors": skillErrors,
		},
	}
}

func currentJSON(cfg config.Config) map[string]any {
	return map[string]any{
		"provider":          cfg.LlmProvider,
		"endpoint":          cfg.LlmEndpoint,
		"model":             cfg.LlmModel,
		"hasKey":            cfg.LlmApiKey != "",
		"memoryEnabled":     cfg.MemoryEnabled,
		"memoryAutoExtract": cfg.MemoryAutoExtract,
		"visualQaEnabled":   cfg.VisualQaEnabled,
		"tokenSet":          cfg.Token != "",
		"configured":        cfg.LlmEndpoint != "" && cfg.LlmModel != "",
	}
}

func problem(id, severity, label, hint string) map[string]any {
	fixable, fixLabel := false, ""
	if check := setup.FindCheck(id); check != nil {
		fixable, fixLabel = check.Fixable, check.FixLabel
	}
	return map[string]any{"id": id, "severity": severity, "label": label, "hint": hint, "fixable": fixable, "fixLabel": fixLabel}
}
