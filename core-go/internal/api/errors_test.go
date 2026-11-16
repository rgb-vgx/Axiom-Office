package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"axiomoffice/core/internal/agent"
	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/store"
)

func TestErrorEnvelopeDefaultCodes(t *testing.T) {
	cases := []struct {
		status int
		code   string
	}{
		{http.StatusBadRequest, CodeInvalidRequest},
		{http.StatusUnsupportedMediaType, CodeInvalidRequest},
		{http.StatusUnauthorized, CodeUnauthorized},
		{http.StatusForbidden, CodeForbidden},
		{http.StatusNotFound, CodeNotFound},
		{http.StatusConflict, CodeBusy},
		{http.StatusServiceUnavailable, CodeCoreUnavailable},
		{http.StatusInternalServerError, CodeInternal},
	}
	for _, c := range cases {
		recorder := httptest.NewRecorder()
		Error(recorder, c.status, "loi")
		payload := decode(t, recorder.Body.String())
		if payload["ok"] != false || payload["error"] != "loi" || payload["code"] != c.code {
			t.Errorf("status %d -> %v, want code %q", c.status, payload, c.code)
		}
	}
}

func TestErrorCodeKeepsExplicitCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	ErrorCode(recorder, http.StatusConflict, CodeUncertain, "chua chac")
	if payload := decode(t, recorder.Body.String()); payload["code"] != CodeUncertain {
		t.Fatalf("payload = %v", payload)
	}
	// code rong -> ma mac dinh theo status, khong co truong code rong.
	recorder = httptest.NewRecorder()
	ErrorCode(recorder, http.StatusConflict, "", "busy")
	if payload := decode(t, recorder.Body.String()); payload["code"] != CodeBusy {
		t.Fatalf("payload = %v", payload)
	}
}

func newCodeDeps(t *testing.T) *Deps {
	t.Helper()
	root := t.TempDir()
	stores := store.OpenStores(filepath.Join(root, "core.db"))
	t.Cleanup(func() { _ = stores.DB.Close() })
	return &Deps{
		Config:  config.Config{Token: "tok", MemoryEnabled: true},
		Paths:   config.NewPaths(root),
		Runtime: &Runtime{Port: 1234, Started: time.Now(), Version: "1.0.0"},
		HTTP:    http.DefaultClient,
		Stores:  stores,
		Manager: agent.NewManager(),
	}
}

func TestAPIStableCodesAtCallSites(t *testing.T) {
	jsonHeaders := map[string]string{"X-Auth-Token": "tok", "Content-Type": "application/json"}

	t.Run("invalid JSON", func(t *testing.T) {
		handler := Handler(newCodeDeps(t), http.NewServeMux())
		reply := call(handler, "POST", "/v1/runs", `{bad`, jsonHeaders)
		if reply.Code != 400 || decode(t, reply.Body.String())["code"] != CodeInvalidRequest {
			t.Fatalf("%d %s", reply.Code, reply.Body.String())
		}
	})

	t.Run("unknown run", func(t *testing.T) {
		handler := Handler(newCodeDeps(t), http.NewServeMux())
		reply := call(handler, "GET", "/v1/runs/khong-co", "", map[string]string{"X-Auth-Token": "tok"})
		if reply.Code != 404 || decode(t, reply.Body.String())["code"] != CodeNotFound {
			t.Fatalf("%d %s", reply.Code, reply.Body.String())
		}
	})

	t.Run("busy port", func(t *testing.T) {
		handler := Handler(newCodeDeps(t), http.NewServeMux())
		body := `{"prompt":"hi","office":{"port":1234},"clientRequestId":"a"}`
		if reply := call(handler, "POST", "/v1/runs", body, jsonHeaders); reply.Code != 200 {
			t.Fatalf("luot dau: %d %s", reply.Code, reply.Body.String())
		}
		body = `{"prompt":"hi","office":{"port":1234},"clientRequestId":"b"}`
		reply := call(handler, "POST", "/v1/runs", body, jsonHeaders)
		if reply.Code != 409 || decode(t, reply.Body.String())["code"] != CodeBusy {
			t.Fatalf("%d %s", reply.Code, reply.Body.String())
		}
	})

	t.Run("stores unavailable", func(t *testing.T) {
		root := t.TempDir()
		deps := newCodeDeps(t)
		deps.Stores = &store.Stores{Available: false, Error: "khong mo duoc"}
		deps.Paths = config.NewPaths(root)
		handler := Handler(deps, http.NewServeMux())
		reply := call(handler, "POST", "/v1/runs", `{"prompt":"hi","office":{"port":1234}}`, jsonHeaders)
		if reply.Code != 503 || decode(t, reply.Body.String())["code"] != CodeCoreUnavailable {
			t.Fatalf("%d %s", reply.Code, reply.Body.String())
		}
	})

	t.Run("confirmation not found", func(t *testing.T) {
		deps := newCodeDeps(t)
		run, _, _ := deps.Manager.TryStartRequest(1234, "seed")
		handler := Handler(deps, http.NewServeMux())
		reply := call(handler, "POST", "/v1/runs/"+run.ID+"/confirm",
			`{"confirmationId":"khong-co","approved":true}`, jsonHeaders)
		if reply.Code != 404 || decode(t, reply.Body.String())["code"] != CodeNotFound {
			t.Fatalf("%d %s", reply.Code, reply.Body.String())
		}
	})
}

func TestWPSAuthErrorCodes(t *testing.T) {
	handler := newWPSHandler(t)
	reply := call(handler, "GET", "/wps/v1/setup", "", nil)
	if reply.Code != 401 || decode(t, reply.Body.String())["code"] != CodeUnauthorized {
		t.Fatalf("thieu xac thuc: %d %s", reply.Code, reply.Body.String())
	}
	reply = call(handler, "GET", "/wps/v1/setup", "", map[string]string{"Origin": "http://evil"})
	if reply.Code != 403 || decode(t, reply.Body.String())["code"] != CodeForbidden {
		t.Fatalf("Origin la: %d %s", reply.Code, reply.Body.String())
	}
}
