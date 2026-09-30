// Package setup: du lieu cho wizard thiet lap (catalog/setup.json, sinh vao catalog_gen.go) va dich loi
// LLM sang tieng Viet - ban Go cua SetupCatalog.cs + Setup/LlmErrors.cs.
package setup

import (
	"encoding/json"
	"strings"
)

type Step struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
}

type Provider struct {
	ID              string   `json:"id"`
	Group           string   `json:"group"`
	Label           string   `json:"label"`
	Description     string   `json:"description"`
	Endpoint        string   `json:"endpoint"`
	NeedsKey        bool     `json:"needsKey"`
	KeyURL          string   `json:"keyUrl"`
	Codec           string   `json:"codec"`
	SuggestedModels []string `json:"suggestedModels"`
}

type Feature struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Recommended bool   `json:"recommended"`
}

type Check struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Fixable  bool   `json:"fixable"`
	FixLabel string `json:"fixLabel"`
	Help     string `json:"help"`
}

type Catalog struct {
	Version   int        `json:"version"`
	Steps     []Step     `json:"steps"`
	Providers []Provider `json:"providers"`
	Features  []Feature  `json:"features"`
	Checks    []Check    `json:"checks"`
}

// Data giai ma mot lan luc khoi dong; catalog sai dinh dang la loi build (generator) nen panic ngay.
var Data = mustLoad()

func mustLoad() Catalog {
	var catalog Catalog
	if err := json.Unmarshal([]byte(catalogJSON), &catalog); err != nil {
		panic("setup catalog: " + err.Error())
	}
	for i := range catalog.Providers {
		if catalog.Providers[i].SuggestedModels == nil {
			catalog.Providers[i].SuggestedModels = []string{}
		}
	}
	return catalog
}

func FindProvider(id string) *Provider {
	wanted := strings.TrimSpace(id)
	if wanted == "" {
		return nil
	}
	for i := range Data.Providers {
		if strings.EqualFold(Data.Providers[i].ID, wanted) {
			return &Data.Providers[i]
		}
	}
	return nil
}

func FindCheck(id string) *Check {
	for i := range Data.Checks {
		if strings.EqualFold(Data.Checks[i].ID, id) {
			return &Data.Checks[i]
		}
	}
	return nil
}

// GuessProviderID doan nha cung cap tu dia chi dang cau hinh (mo lai wizard thi chon dung lua chon cu).
func GuessProviderID(endpoint, codec string) string {
	value := strings.ToLower(strings.TrimSpace(endpoint))
	switch {
	case value == "":
		return "company"
	case strings.Contains(value, "api.openai.com"):
		return "openai"
	case strings.Contains(value, "anthropic.com"):
		return "anthropic"
	case strings.Contains(value, "generativelanguage.googleapis.com"):
		return "gemini"
	}
	return "company"
}
