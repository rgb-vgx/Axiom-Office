package setup

import (
	"strings"
	"testing"
)

// Cung bang ca voi SetupTests.cs cua ban .NET: moi loai loi ra dung kind.
func TestDescribeKinds(t *testing.T) {
	cases := map[string]string{
		"":                                   "internal",
		"Endpoint is not configured":         "config",
		"Model is not configured":            "config",
		"HTTP 401: invalid api key":          "auth",
		"HTTP 403: forbidden":                "forbidden",
		"HTTP 404: not found":                "notFound",
		"HTTP 404: model `x` does not exist": "model",
		"HTTP 400: unknown model":            "model",
		"HTTP 429: slow down":                "rate",
		"HTTP 503: overloaded":               "server",
		"HTTP 418: teapot":                   "server",
		"request timed out after 30s":        "timeout",
		"HttpRequestException: dial tcp: lookup nope.invalid: no such host":                                                                  "dns",
		"HttpRequestException: dial tcp 127.0.0.1:1: connectex: No connection could be made because the target machine actively refused it.": "network",
		"provider returned an empty reply": "reply",
		"invalid provider response: bad":   "reply",
		"cancelled":                        "cancelled",
		"something odd":                    "internal",
	}
	for input, want := range cases {
		if got := Describe(input, "openai", "", "").Kind; got != want {
			t.Errorf("Describe(%q).Kind = %q, want %q", input, got, want)
		}
	}
}

func TestDescribeAuthPointsToKeyURL(t *testing.T) {
	failure := Describe("HTTP 401: nope", "openai", "", "")
	if !strings.Contains(failure.Hint, "platform.openai.com") {
		t.Fatalf("hint should name the key page: %q", failure.Hint)
	}
	if !strings.Contains(Describe("HTTP 401: nope", "company", "", "").Hint, "quản trị viên") {
		t.Fatal("company provider should point to the administrator")
	}
}

func TestTimeoutSecondsInMessage(t *testing.T) {
	if got := Describe("request timed out after 30s", "", "", "").Message; !strings.Contains(got, "30 giây") {
		t.Fatalf("message = %q", got)
	}
}

func TestCatalog(t *testing.T) {
	if len(Data.Providers) < 4 || FindProvider("OPENAI") == nil || FindCheck("token") == nil {
		t.Fatal("catalog not loaded")
	}
	guesses := map[string]string{
		"":                             "company",
		"https://api.openai.com/v1":    "openai",
		"https://api.anthropic.com/v1": "anthropic",
		"https://generativelanguage.googleapis.com/v1beta/openai": "gemini",
		"http://10.0.0.5:8080/v1":                                 "company",
	}
	for endpoint, want := range guesses {
		if got := GuessProviderID(endpoint, "openai"); got != want {
			t.Errorf("GuessProviderID(%q) = %q, want %q", endpoint, got, want)
		}
	}
}
