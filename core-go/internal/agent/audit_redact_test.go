package agent

import (
	"strings"
	"testing"
)

// P6-01: params ghi vao tool_calls/SSE phai khong con khoa API va PII.
func TestRedactedParamsHidesSecrets(t *testing.T) {
	raw := `{"apiKey":"sk-abcdefghijklmnopqrstuv","note":"gọi 0912 345 678","cccd":"012345678901"}`
	clean := redactedParams(raw)
	for _, secret := range []string{"sk-abcdefghijklmnopqrstuv", "0912 345 678", "012345678901"} {
		if strings.Contains(clean, secret) {
			t.Errorf("redactedParams con lo %q trong %q", secret, clean)
		}
	}
	if !strings.Contains(clean, "[ẩn]") {
		t.Errorf("redactedParams khong danh dau [ẩn]: %q", clean)
	}
}
