package api

import (
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"axiomoffice/core/internal/config"
)

// Kenh quan tri cho pane Ask AI trong WPS (docs/architecture/wps-bridge.md muc 7, docs/architecture/ribbon-ui.md muc 2).
//
// Pane la trang do Core phuc vu: cung origin voi Core nhung KHONG co token, nen guard chung (moi
// request khong token -> 401) chan het /v1/*. Vi vay moi endpoint pane can duoc dang ky them duoi
// /wps/v1/* voi xac thuc kenh add-in (checkWPSAuth: Origin dung origin cua Core, hoac token khi
// khong co Origin) - cung quy tac voi /wps/v1/runs. Doi lai, /wps/v1/* khong doi token: mot tien
// trinh tren chinh may nay co the gia Origin. Do la mo hinh tin cay da co tu /wps/v1/runs (chay
// duoc agent thi cung sua duoc tai lieu), khong phai quyen moi.

// routeBoth dang ky mot handler duoi CA HAI tien to: /v1/* (guard chung: token) va /wps/v1/* (xac
// thuc kenh add-in WPS, de pane trong WPS goi duoc khi khong co token).
func routeBoth(mux *http.ServeMux, deps *Deps, pattern string, handler http.HandlerFunc) {
	mux.HandleFunc(pattern, handler)
	mux.HandleFunc(strings.Replace(pattern, "/v1/", "/wps/v1/", 1), func(w http.ResponseWriter, r *http.Request) {
		if !checkWPSAuth(w, r, deps) {
			return
		}
		handler(w, r)
	})
}

// mapWPSAdmin: cac duong chi pane/ribbon WPS can ma /v1/* khong co (hoac khong dung duoc vi thieu token).
func mapWPSAdmin(mux *http.ServeMux, deps *Deps) {
	// GET /wps/v1/status: nut "Trang thai" tren ribbon (ban Windows: RibbonUi.StatusReport).
	//
	// Pane khong tu hoi duoc: bridge WPS nam o CONG RIENG, va /health cua Core khong biet gi ve add-in.
	// Mot loi goi nay tra ca hai tang, nen pane chi can mot request va khong phai doan Origin cho
	// /health (guard chung chan /health khi request co Origin - xem guard.go).
	//
	// Day la duong CHI DOC: khong xep hang lenh nao, khong doi trang thai nao.
	mux.HandleFunc("GET /wps/v1/status", func(w http.ResponseWriter, r *http.Request) {
		if !checkWPSAuth(w, r, deps) {
			return
		}
		OK(w, map[string]any{"addin": deps.WPS.Status(), "core": coreStatus(deps)})
	})

	// POST /wps/v1/exec {action, params}: goi thang mot lenh bridge, khong qua agent, khong ton token.
	// Dung cho nut "Soat bang"/"Soat bo cuc" tren ribbon va "Hoan tac luot nay"/"Chen tra loi" trong
	// pane - Windows lam viec nay bang CommandDispatcher.Execute ngay trong tien trinh add-in.
	mux.HandleFunc("POST /wps/v1/exec", func(w http.ResponseWriter, r *http.Request) {
		if !checkWPSAuth(w, r, deps) {
			return
		}
		body, parsed := readJSON(r)
		if !parsed {
			Error(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		action := strings.TrimSpace(textOf(body["action"]))
		if action == "" {
			Error(w, http.StatusBadRequest, "missing 'action' field")
			return
		}
		params := map[string]any{}
		if raw, ok := body["params"].(map[string]any); ok {
			params = raw
		}
		// Luot chay dang chay: lenh ngoai se chen vao giua cac buoc cua agent (add-in chay tuan tu nen
		// khong hong du lieu, nhung agent mat dung trang thai tai lieu no dang theo doi).
		//
		// Hai ca 409 duoi day mang ma may doc duoc: pane hien cau tieng Viet rieng cho tung ca
		// (pane.html: execError) thay vi do chuoi tieng Anh cua Core.
		if deps.Manager != nil && deps.Manager.Running() > 0 {
			ErrorCode(w, http.StatusConflict, "run-in-progress", "a run is in progress: wait for it to finish")
			return
		}
		if !deps.WPS.Connected() {
			ErrorCode(w, http.StatusConflict, "addin-disconnected", "WPS add-in is not connected: open a document in WPS to connect the add-in")
			return
		}
		reply, ok := deps.WPS.Enqueue(action, params)
		if !ok {
			ErrorCode(w, http.StatusConflict, "addin-disconnected", "WPS add-in is not connected: open a document in WPS to connect the add-in")
			return
		}
		// reply da la phong bi cua bridge ({"ok":true,"result":...} / {"ok":false,"error":...}) - tra nguyen.
		writeJSON(w, http.StatusOK, reply)
	})

	// POST /wps/v1/config {TenKhoa: gia tri}: ghi cau hinh nen tang (Linux: config.json, Windows: HKCU).
	// Truoc 19/10/2026 Core chi doc cau hinh nen moi nut doi cau hinh tren giao dien WPS khong co duong
	// that dung sau (docs/architecture/ribbon-ui.md 5.1: toggle "Nhin tai lieu" bi chan).
	mux.HandleFunc("POST /wps/v1/config", func(w http.ResponseWriter, r *http.Request) {
		if !checkWPSAuth(w, r, deps) {
			return
		}
		body, parsed := readJSON(r)
		if !parsed {
			Error(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if len(body) == 0 {
			Error(w, http.StatusBadRequest, "body must contain at least one setting")
			return
		}
		// Thu tu on dinh: thong bao loi khong doi theo thu tu key cua JSON.
		names := make([]string, 0, len(body))
		for name := range body {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if !settableSettings[name] {
				Error(w, http.StatusBadRequest, "setting cannot be changed from the pane: "+name)
				return
			}
		}

		source := config.DefaultSource()
		if deps.ConfigSource != nil {
			// Nguon inject (test): ghi vao file tam, khong dung vao config that.
			source = deps.ConfigSource
		}
		for _, name := range names {
			value := body[name]
			// Khoa API: ma hoa truoc khi ghi (Windows DPAPI; Linux giu nguyen, file 0600) - giong
			// config.py:protect_secret cua extension LibreOffice.
			if name == "LlmApiKey" || name == "TavilyApiKey" {
				value = config.Protect(textOf(value))
			}
			if err := config.SetValue(source, name, value); err != nil {
				Error(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		cfg := deps.Live()
		OK(w, map[string]any{
			"saved": names,
			// ignored: khoa da ghi nhung gia tri doc lai KHONG doi - bien moi truong AXIOM_* de len
			// cau hinh (config.From: env > nguon nen tang). Noi ro de nguoi dung khong tuong nut hong.
			"ignored": overriddenSettings(cfg, body),
			"current": currentJSON(cfg),
		})
	})
}

// coreStatus: tang Core cua bao cao "Trang thai" (ban Windows doc tu /health cua Core + /health cua
// bridge trong add-in). Cung truong voi /health, cong them logFile va so ky nang - hai thu nut "Mo nhat
// ky" va man hinh "Ky nang" can, de pane khoi phai goi ba noi.
//
// endpoint + configured: nut "Kiem tra mo hinh" trong pane phai biet da chon AI chua (ban Windows:
// RibbonUi.TestModel doc Config.LlmEndpoint/LlmModel). Thieu chung thi pane phai doan, ma doan sai thi
// nut bao "chua cau hinh" ngay ca khi da cau hinh.
func coreStatus(deps *Deps) map[string]any {
	var memoryError any
	if deps.Stores.Error != "" {
		memoryError = deps.Stores.Error
	}
	cfg := deps.Live()
	skillCount, skillErrors := 0, 0
	if deps.Skills != nil {
		skillCount, skillErrors = len(deps.Skills.All()), len(deps.Skills.Errors())
	}
	return map[string]any{
		"version":       deps.Runtime.Version,
		"pid":           pid(),
		"port":          deps.Runtime.Port,
		"started":       deps.Runtime.StartedText(),
		"uptimeSeconds": math.Round(time.Since(deps.Runtime.Started).Seconds()*10) / 10,
		"dataDir":       deps.Paths.Root,
		"logFile":       deps.Paths.LogFile,
		"provider":      cfg.LlmProvider,
		"endpoint":      cfg.LlmEndpoint,
		"model":         cfg.LlmModel,
		"configured":    cfg.LlmEndpoint != "" && cfg.LlmModel != "",
		"memory":        deps.Stores.Status(cfg.MemoryEnabled),
		"memoryError":   memoryError,
		"skills":        skillCount,
		"skillErrors":   skillErrors,
	}
}

// settableSettings: khoa cau hinh pane/ribbon duoc phep doi. Co y KHONG cho ghi Token va CorePort:
// doi hai thu do tu UI lam Core dang chay mat ket noi voi add-in (token doi -> moi request 401, cong
// doi -> session file cu tro sai cho).
var settableSettings = map[string]bool{
	"VisualQaEnabled":       true,
	"TrackChangesEnabled":   true,
	"LlmShowReasoning":      true,
	"MemoryEnabled":         true,
	"MemoryAutoExtract":     true,
	"VerifyWorkEnabled":     true,
	"WebSourceStore":        true,
	"TavilyEnabled":         true,
	"TavilyApiKey":          true,
	"LlmProvider":           true,
	"LlmEndpoint":           true,
	"LlmModel":              true,
	"LlmApiKey":             true,
	"LlmFallbackModels":     true,
	"LlmContextWindow":      true,
	"ConfirmTimeoutSeconds": true,
}

// overriddenSettings: cac khoa vua ghi ma gia tri song van khac (bi bien moi truong AXIOM_* de).
// Khoa bi mat (LlmApiKey/TavilyApiKey) va danh sach (LlmFallbackModels) khong so sanh duoc: bo qua.
func overriddenSettings(cfg config.Config, written map[string]any) []string {
	ignored := []string{}
	for name, value := range written {
		live := liveSetting(cfg, name)
		if live == nil {
			continue
		}
		want, wantOK := config.SettingText(value)
		got, gotOK := config.SettingText(live)
		if wantOK && gotOK && want != got {
			ignored = append(ignored, name)
		}
	}
	sort.Strings(ignored)
	return ignored
}

// liveSetting: gia tri hien tai cua mot khoa cau hinh (nil = khong so sanh duoc).
func liveSetting(cfg config.Config, name string) any {
	switch name {
	case "VisualQaEnabled":
		return cfg.VisualQaEnabled
	case "TrackChangesEnabled":
		return cfg.TrackChangesEnabled
	case "LlmShowReasoning":
		return cfg.LlmShowReasoning
	case "MemoryEnabled":
		return cfg.MemoryEnabled
	case "MemoryAutoExtract":
		return cfg.MemoryAutoExtract
	case "VerifyWorkEnabled":
		return cfg.VerifyWorkEnabled
	case "WebSourceStore":
		return cfg.WebSourceStore
	case "TavilyEnabled":
		return cfg.TavilyEnabled
	case "LlmProvider":
		return cfg.LlmProvider
	case "LlmEndpoint":
		return cfg.LlmEndpoint
	case "LlmModel":
		return cfg.LlmModel
	case "LlmContextWindow":
		return cfg.LlmContextWindow
	case "ConfirmTimeoutSeconds":
		return cfg.ConfirmTimeoutSeconds
	default:
		return nil
	}
}
