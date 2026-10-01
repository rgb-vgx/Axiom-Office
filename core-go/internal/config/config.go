// Package config doc cau hinh Agent Core - ban Go cua src/AxiomOffice.Core/Config/CoreConfig.cs.
//
// Thu tu uu tien: bien moi truong AXIOM_* > nguon nen tang (HKCU\Software\AxiomOffice tren Windows,
// ~/.config/axiom-office/config.json tren Linux - cung ten khoa) > mac dinh. Hai ban Core (.NET va Go) doc
// cung nguon voi cung quy tac nen thay the nhau duoc.
package config

import (
	"os"
	"strconv"
	"strings"
)

const (
	DefaultPort      = 47840
	DefaultMutexName = `Local\AxiomOffice.Core`
	RegistryKeyPath  = `Software\AxiomOffice`
)

// Source la noi luu cau hinh cua nen tang (registry hoac config.json). Loi/thieu -> "" / false.
type Source interface {
	String(name string) (string, bool)
	Int(name string) (int, bool)
}

// Config la anh chup cau hinh tai mot thoi diem (Load lai moi luot de doi LLM co hieu luc ngay).
type Config struct {
	CorePort          int
	SingleInstance    bool
	MutexName         string
	Token             string
	CoreEnabled       bool
	MemoryEnabled     bool
	MemoryAutoExtract bool
	LlmProvider       string
	LlmEndpoint       string
	LlmModel          string
	// LlmRequestTimeoutSeconds: tran cho mot loi goi NGAN (kiem tra cau hinh, tom tat, trich xuat
	// memory). 0 = dung mac dinh 120s. Luot agent KHONG dung gia tri nay - no khong co tran thoi
	// gian, chi bi chan boi watchdog "khong nhan duoc byte nao" (300s).
	LlmRequestTimeoutSeconds int
	ConfirmTimeoutSeconds    int
	VisualQaEnabled          bool
	LlmApiKey                string
	MemoryModel              string
	EmbeddingModel           string
	EmbeddingEndpoint        string
	DataDirOverride          string
	SessionDirectoryOverride string
	SkillDirs                []string
}

// DynamicPort: CorePort = 0 -> de he dieu hanh cap port (chi dung cho test).
func (c Config) DynamicPort() bool { return c.CorePort == 0 }

// Env la nguon bien moi truong; tach ra de test khong dung vao moi truong that.
type Env func(key string) string

// Load doc cau hinh tu moi truong that + nguon mac dinh cua nen tang.
func Load() Config { return From(os.Getenv, DefaultSource()) }

// From ghep bien moi truong voi nguon nen tang.
func From(env Env, source Source) Config {
	port := intValue(env, "AXIOM_CORE_PORT", source, "CorePort", DefaultPort)
	if port < 0 || port > 65535 {
		port = DefaultPort
	}

	mutex := text(env, "AXIOM_CORE_MUTEX_NAME", nil, "")
	if mutex == "" {
		mutex = DefaultMutexName
	}
	provider := text(env, "AXIOM_LLM_PROVIDER", source, "LlmProvider")
	if provider == "" {
		provider = "openai"
	}

	return Config{
		CorePort:                 port,
		SingleInstance:           flag(env, "AXIOM_CORE_SINGLE_INSTANCE", nil, "", true),
		MutexName:                mutex,
		Token:                    text(env, "AXIOM_TOKEN", source, "Token"),
		CoreEnabled:              flag(env, "AXIOM_CORE_ENABLED", source, "CoreEnabled", true),
		MemoryEnabled:            flag(env, "AXIOM_MEMORY_ENABLED", source, "MemoryEnabled", true),
		MemoryAutoExtract:        flag(env, "AXIOM_MEMORY_AUTO_EXTRACT", source, "MemoryAutoExtract", true),
		LlmProvider:              provider,
		LlmEndpoint:              text(env, "AXIOM_LLM_ENDPOINT", source, "LlmEndpoint"),
		LlmModel:                 text(env, "AXIOM_LLM_MODEL", source, "LlmModel"),
		LlmRequestTimeoutSeconds: clamp(intValue(env, "AXIOM_LLM_REQUEST_TIMEOUT", source, "LlmRequestTimeoutSeconds", 0), 0, 600),
		ConfirmTimeoutSeconds:    clamp(intValue(env, "AXIOM_CONFIRM_TIMEOUT", source, "ConfirmTimeoutSeconds", 120), 1, 3600),
		VisualQaEnabled:          flag(env, "AXIOM_VISUAL_QA", source, "VisualQaEnabled", false),
		LlmApiKey:                Unprotect(text(env, "AXIOM_LLM_API_KEY", source, "LlmApiKey")),
		MemoryModel:              text(env, "AXIOM_MEMORY_MODEL", source, "MemoryModel"),
		EmbeddingModel:           text(env, "AXIOM_EMBEDDING_MODEL", source, "EmbeddingModel"),
		EmbeddingEndpoint:        text(env, "AXIOM_EMBEDDING_ENDPOINT", source, "EmbeddingEndpoint"),
		DataDirOverride:          text(env, "AXIOM_CORE_DATA_DIR", nil, ""),
		SessionDirectoryOverride: text(env, "AXIOM_SESSION_DIR", nil, ""),
		SkillDirs:                list(text(env, "AXIOM_SKILL_DIRS", source, "SkillDirs")),
	}
}

func text(env Env, envKey string, source Source, name string) string {
	if raw := strings.TrimSpace(env(envKey)); raw != "" {
		return raw
	}
	if source == nil || name == "" {
		return ""
	}
	value, _ := source.String(name)
	return strings.TrimSpace(value)
}

// "1"/"true"/"yes"/"on" = bat; "0"/"false"/"no"/"off" = tat; gia tri khac -> mac dinh.
func flag(env Env, envKey string, source Source, name string, fallback bool) bool {
	switch strings.ToLower(text(env, envKey, source, name)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func intValue(env Env, envKey string, source Source, name string, fallback int) int {
	if raw := strings.TrimSpace(env(envKey)); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			return value
		}
	}
	if source != nil {
		if value, ok := source.Int(name); ok {
			return value
		}
	}
	return fallback
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func list(raw string) []string {
	var items []string
	for _, part := range strings.Split(raw, ";") {
		if part = strings.TrimSpace(part); part != "" {
			items = append(items, part)
		}
	}
	return items
}
