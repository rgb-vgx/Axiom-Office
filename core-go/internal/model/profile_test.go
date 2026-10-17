package model

import (
	"net/http"
	"testing"
)

// P4-01: them nha cung cap moi chi bang khai bao profile - khong sua client.go/types.go.
func TestProfileRegistryDrivesCodecAndAuth(t *testing.T) {
	RegisterProfile(Profile{
		Name: "khach-anthropic", Family: FamilyAnthropic,
		AuthHeader: "x-api-key", AuthPrefix: "",
		ExtraHeaders:     map[string]string{"anthropic-version": "2023-06-01"},
		SupportsThinking: false, SupportsTemperature: true,
	})
	if got := NewCodec("khach-anthropic").Name(); got != "anthropic" {
		t.Fatalf("codec = %q, want anthropic", got)
	}
	request, _ := http.NewRequest("POST", "https://x", nil)
	ApplyAuth(request, "khach-anthropic", "secret")
	if request.Header.Get("x-api-key") != "secret" || request.Header.Get("anthropic-version") != "2023-06-01" {
		t.Fatalf("header = %v", request.Header)
	}
	if request.Header.Get("Authorization") != "" {
		t.Fatal("nha cung cap x-api-key khong duoc gui Authorization")
	}
}

func TestProfileDefaultsAndThinking(t *testing.T) {
	if ProfileFor("khong-biet").Family != FamilyOpenAI || NewCodec("khong-biet").Name() != "openai" {
		t.Fatal("ten la phai mac dinh OpenAI-compatible")
	}
	if !ProfileFor("openai").SupportsThinking || ProfileFor("anthropic").SupportsThinking {
		t.Fatal("co nhan thinking phai theo profile")
	}
	request, _ := http.NewRequest("POST", "https://x", nil)
	ApplyAuth(request, "openai", "k")
	if request.Header.Get("Authorization") != "Bearer k" {
		t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
	}
	// Khong co khoa thi khong them header nao.
	empty, _ := http.NewRequest("POST", "https://x", nil)
	ApplyAuth(empty, "anthropic", "")
	if empty.Header.Get("x-api-key") != "" || empty.Header.Get("Authorization") != "" {
		t.Fatal("khong co khoa thi khong gui header xac thuc")
	}
}
