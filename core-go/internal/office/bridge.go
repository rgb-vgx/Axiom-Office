package office

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/model"
)

const (
	// DefaultCommandTimeout: wpp.exportPdf / writer.exportPdf co the lau, va mot lenh ghi vao bang lon
	// lam LibreOffice tinh lai CA so (SumProduct tren 50.000 dong x 10 cong thuc mat hang phut).
	//
	// Phai lon hon gioi han cua chinh bridge, khong thi ben goi cat truoc va thong bao ro rang cua
	// bridge ("Busy: LibreOffice did not answer within Ns") bi thay bang mot loi het gio mo ho:
	//   gate cua extension LibreOffice 300s (axiom/gate.py)  <  330s nay  =  MCP live commandTimeout
	// Do ngay 02/10/2026 tren de Inventory Optimization: ban dau la 90s, va 60s o phia extension.
	DefaultCommandTimeout = 330 * time.Second
	HealthTimeout         = 5 * time.Second
	commandsTimeout       = 15 * time.Second
	catalogTTL            = 5 * time.Minute
)

// BridgeResult: ket qua mot lan goi bridge. RawJSON la body nguyen van - dung lam ket qua tra cho model
// (model doc duoc ca ok lan error).
type BridgeResult struct {
	OK      bool
	RawJSON string
	Result  map[string]any
	Error   string
	Status  int
	Ms      int64
}

// CommandParam / Command / CommandCatalog: registry lenh cua DLL dang chay (GET /commands).
type CommandParam struct {
	Name     string
	Required bool
	Hint     string
}

type Command struct {
	Name    string
	Kind    string
	Agent   bool
	Summary string
	Params  []CommandParam
}

type CommandCatalog struct {
	Version  string
	Commands []Command
}

type cachedCatalog struct {
	catalog   *CommandCatalog
	fetchedAt time.Time
}

// BridgeClient goi HTTP vao bridge trong app (127.0.0.1:port). Token chi gui trong header.
type BridgeClient struct {
	http  *http.Client
	token string
	// CommandTimeout: tran thoi gian cho POST /cmd. Mac dinh DefaultCommandTimeout; MCP server dat
	// 330s vi lenh ai.ask co the chay toi 5 phut.
	CommandTimeout time.Duration
	mu             sync.Mutex
	cache          map[int]cachedCatalog
}

func NewBridgeClient(httpClient *http.Client, token string) *BridgeClient {
	return &BridgeClient{http: httpClient, token: token, cache: map[int]cachedCatalog{}}
}

func (b *BridgeClient) commandTimeout() time.Duration {
	if b.CommandTimeout > 0 {
		return b.CommandTimeout
	}
	return DefaultCommandTimeout
}

// Command gui mot lenh POST /cmd. Khong tra loi loi giao thuc: OK=false kem RawJSON de model tu xu ly.
func (b *BridgeClient) Command(ctx context.Context, port int, action string, params any) BridgeResult {
	return b.send(ctx, port, "/cmd", commandBody(action, params), b.commandTimeout())
}

// CommandRaw tra nguyen van body cua POST /cmd - dung cho cac tool MCP live (khong dien giai gi).
// Loi ket noi cung duoc boc san thanh {"ok":false,"error":...} nen ben goi chi viec tra tiep.
func (b *BridgeClient) CommandRaw(ctx context.Context, port int, action string, params any) string {
	return b.send(ctx, port, "/cmd", commandBody(action, params), b.commandTimeout()).RawJSON
}

// HealthRaw tra nguyen van body cua GET /health.
func (b *BridgeClient) HealthRaw(ctx context.Context, port int) string {
	return b.send(ctx, port, "/health", nil, HealthTimeout).RawJSON
}

func (b *BridgeClient) Health(ctx context.Context, port int) map[string]any {
	result := b.send(ctx, port, "/health", nil, HealthTimeout)
	if !result.OK {
		return nil
	}
	return result.Result
}

func commandBody(action string, params any) map[string]any {
	body := map[string]any{"action": action}
	if params == nil {
		body["params"] = map[string]any{}
	} else {
		body["params"] = params
	}
	return body
}

// GetCommands bo lenh cua DLL dang chay, cache theo port (muc 7.5).
func (b *BridgeClient) GetCommands(ctx context.Context, port int, force bool) *CommandCatalog {
	b.mu.Lock()
	cached, found := b.cache[port]
	b.mu.Unlock()
	if !force && found && time.Since(cached.fetchedAt) < catalogTTL {
		return cached.catalog
	}

	result := b.send(ctx, port, "/commands", nil, commandsTimeout)
	if !result.OK || result.Result == nil {
		corelog.Info("GET /commands port %d failed: %s", port, result.Error)
		return nil
	}
	version, _ := result.Result["version"].(string)
	catalog := &CommandCatalog{Version: version, Commands: []Command{}}
	if list, ok := result.Result["commands"].([]any); ok {
		for _, item := range list {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			params := []CommandParam{}
			if paramList, ok := entry["params"].([]any); ok {
				for _, p := range paramList {
					param, ok := p.(map[string]any)
					if !ok {
						continue
					}
					name, _ := param["name"].(string)
					required, _ := param["required"].(bool)
					hint, _ := param["hint"].(string)
					params = append(params, CommandParam{Name: name, Required: required, Hint: hint})
				}
			}
			name, _ := entry["name"].(string)
			kind, _ := entry["kind"].(string)
			agent, _ := entry["agent"].(bool)
			summary, _ := entry["summary"].(string)
			catalog.Commands = append(catalog.Commands, Command{Name: name, Kind: kind, Agent: agent, Summary: summary, Params: params})
		}
	}

	b.mu.Lock()
	b.cache[port] = cachedCatalog{catalog: catalog, fetchedAt: time.Now()}
	b.mu.Unlock()
	return catalog
}

func (b *BridgeClient) send(ctx context.Context, port int, path string, body map[string]any, timeout time.Duration) BridgeResult {
	started := time.Now()
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	method := http.MethodGet
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return BridgeResult{RawJSON: errorJSON(err.Error()), Error: err.Error()}
		}
		method, reader = http.MethodPost, bytes.NewReader(payload)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	request, err := http.NewRequestWithContext(callCtx, method, url, reader)
	if err != nil {
		return BridgeResult{RawJSON: errorJSON("HttpRequestException: " + err.Error()), Error: err.Error()}
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	if b.token != "" {
		request.Header.Set("X-Auth-Token", b.token)
	}

	response, err := b.http.Do(request)
	if err != nil {
		ms := time.Since(started).Milliseconds()
		if callCtx.Err() != nil && ctx.Err() == nil {
			corelog.Info("Bridge call %s port %d timed out after %dms", path, port, timeout.Milliseconds())
			message := fmt.Sprintf("bridge did not answer within %ds", int(timeout.Seconds()))
			return BridgeResult{RawJSON: errorJSON(message), Error: "bridge timeout", Ms: ms}
		}
		message := "HttpRequestException: " + err.Error()
		if ctx.Err() != nil {
			message = "cancelled"
		}
		return BridgeResult{RawJSON: errorJSON(message), Error: message, Ms: ms}
	}
	defer response.Body.Close()
	text, err := io.ReadAll(response.Body)
	if err != nil {
		return BridgeResult{RawJSON: errorJSON("cancelled"), Error: "cancelled", Ms: time.Since(started).Milliseconds()}
	}
	ms := time.Since(started).Milliseconds()

	// Khong phai JSON: tra nguyen van de con chan doan.
	var parsed map[string]any
	if json.Unmarshal(text, &parsed) != nil {
		parsed = nil
	}
	ok, _ := parsed["ok"].(bool)
	var result map[string]any
	if parsed != nil {
		result, _ = parsed["result"].(map[string]any)
	}
	errorText, _ := parsed["error"].(string)
	if !ok && errorText == "" {
		errorText = fmt.Sprintf("HTTP %d", response.StatusCode)
	}
	return BridgeResult{OK: ok, RawJSON: strings.TrimRight(string(text), "\n"), Result: result, Error: errorText,
		Status: response.StatusCode, Ms: ms}
}

func errorJSON(message string) string {
	return model.MarshalRelaxed(map[string]any{"ok": false, "error": message})
}
