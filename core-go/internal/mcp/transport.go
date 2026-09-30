// Package mcp: MCP client cua Core (New_arch.md muc 8.7) - ban Go cua Mcp/McpManager.cs, McpTransport.cs.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/model"
)

// ProtocolVersion: phien ban MCP ma Core noi.
const ProtocolVersion = "2025-06-18"

// Error: loi tu MCP server hoac tu ket noi.
type Error struct{ Message string }

func (e Error) Error() string { return e.Message }

func fail(format string, args ...any) error { return Error{Message: fmt.Sprintf(format, args...)} }

// transport: kenh JSON-RPC 2.0 toi mot MCP server.
type transport interface {
	Alive() bool
	Request(ctx context.Context, method string, params any) (json.RawMessage, error)
	Notify(ctx context.Context, method string, params any) error
	Close()
}

// rawMessageParams: params rong -> bo han truong (giong ban .NET).
func encodeMessage(id *int64, method string, params any) ([]byte, error) {
	message := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != nil {
		message["id"] = *id
	}
	if params != nil {
		message["params"] = params
	}
	return json.Marshal(message)
}

// rpcError: {"error": {...}} trong response.
func rpcError(raw json.RawMessage) error {
	var envelope struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Error == nil {
		return nil
	}
	if envelope.Error.Message != "" {
		return fail("%s", envelope.Error.Message)
	}
	return fail("%s", model.Truncate(string(raw), 300))
}

func rpcResult(raw json.RawMessage) json.RawMessage {
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return nil
	}
	return envelope.Result
}

// stdioTransport: moi thong diep JSON mot dong tren stdin/stdout cua tien trinh con (chuan MCP stdio).
type stdioTransport struct {
	name    string
	command *exec.Cmd
	stdin   io.WriteCloser
	writeMu sync.Mutex

	mu      sync.Mutex
	pending map[int64]chan json.RawMessage
	nextID  int64
	exited  chan struct{}
}

func newStdioTransport(name, command string, args []string, env map[string]string, workingDirectory string) (*stdioTransport, error) {
	cmd := exec.Command(command, args...)
	if workingDirectory != "" {
		cmd.Dir = workingDirectory
	}
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fail("cannot start %s: %v", command, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fail("cannot start %s: %v", command, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fail("cannot start %s: %v", command, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fail("cannot start %s: %v", command, err)
	}

	transport := &stdioTransport{name: name, command: cmd, stdin: stdin, pending: map[int64]chan json.RawMessage{},
		exited: make(chan struct{})}
	go transport.readLoop(stdout)
	go transport.drainStderr(stderr)
	go func() {
		if err := cmd.Wait(); err != nil {
			corelog.Info("mcp %s exited: %v", name, err)
		}
		close(transport.exited)
		transport.failPending()
	}()
	return transport, nil
}

func (t *stdioTransport) Alive() bool {
	select {
	case <-t.exited:
		return false
	default:
		return true
	}
}

func (t *stdioTransport) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	t.mu.Lock()
	t.nextID++
	id := t.nextID
	waiter := make(chan json.RawMessage, 1)
	t.pending[id] = waiter
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
	}()

	if err := t.write(&id, method, params); err != nil {
		return nil, err
	}
	select {
	case raw := <-waiter:
		if err := rpcError(raw); err != nil {
			return nil, err
		}
		return rpcResult(raw), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *stdioTransport) Notify(_ context.Context, method string, params any) error {
	return t.write(nil, method, params)
}

func (t *stdioTransport) write(id *int64, method string, params any) error {
	payload, err := encodeMessage(id, method, params)
	if err != nil {
		return err
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if !t.Alive() {
		return fail("mcp server '%s' has exited", t.name)
	}
	if _, err := t.stdin.Write(append(payload, '\n')); err != nil {
		return fail("mcp server '%s' is not accepting input: %v", t.name, err)
	}
	return nil
}

// readLoop: response -> danh thuc nguoi cho; server hoi nguoc (sampling, roots...) -> tra loi khong ho tro.
func (t *stdioTransport) readLoop(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var envelope struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if json.Unmarshal([]byte(line), &envelope) != nil {
			continue
		}
		switch {
		case envelope.ID != nil && envelope.Method == "":
			t.mu.Lock()
			waiter := t.pending[*envelope.ID]
			t.mu.Unlock()
			if waiter != nil {
				waiter <- json.RawMessage(line)
			}
		case envelope.ID != nil && envelope.Method != "":
			// Server hoi nguoc client (sampling, roots...): chua ho tro.
			t.writeRaw(map[string]any{"jsonrpc": "2.0", "id": *envelope.ID,
				"error": map[string]any{"code": -32601, "message": "method not supported by Axiom Office"}})
		}
	}
	if err := scanner.Err(); err != nil {
		corelog.Info("mcp %s read loop error: %v", t.name, err)
	}
}

func (t *stdioTransport) drainStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	for lines := 0; scanner.Scan() && lines < 20; lines++ {
		corelog.Info("mcp %s stderr: %s", t.name, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		corelog.Info("mcp %s stderr error: %v", t.name, err)
	}
}

func (t *stdioTransport) writeRaw(message map[string]any) {
	payload, err := json.Marshal(message)
	if err != nil {
		return
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	_, _ = t.stdin.Write(append(payload, '\n'))
}

// failPending: server da tat - bao loi cho moi nguoi dang cho thay vi de ho treo.
func (t *stdioTransport) failPending() {
	t.mu.Lock()
	pending := t.pending
	t.pending = map[int64]chan json.RawMessage{}
	t.mu.Unlock()
	for id, waiter := range pending {
		failure, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id,
			"error": map[string]any{"code": -32000, "message": "mcp server '" + t.name + "' closed the connection"}})
		if err != nil {
			continue
		}
		select {
		case waiter <- json.RawMessage(failure):
		default:
		}
	}
}

func (t *stdioTransport) Close() {
	_ = t.stdin.Close()
	select {
	case <-t.exited:
	case <-time.After(2 * time.Second):
		if t.command.Process != nil {
			_ = t.command.Process.Kill()
		}
	}
}

// httpTransport: Streamable HTTP - POST JSON-RPC, tra ve JSON hoac SSE (data: {...}); giu Mcp-Session-Id.
type httpTransport struct {
	name    string
	url     string
	headers map[string]string
	http    *http.Client

	mu        sync.Mutex
	nextID    int64
	sessionID string
}

func newHTTPTransport(name, url string, headers map[string]string, httpClient *http.Client) *httpTransport {
	return &httpTransport{name: name, url: url, headers: headers, http: httpClient}
}

func (t *httpTransport) Alive() bool { return true }

func (t *httpTransport) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	t.mu.Lock()
	t.nextID++
	id := t.nextID
	t.mu.Unlock()
	raw, err := t.post(ctx, &id, method, params)
	if err != nil {
		return nil, err
	}
	if problem := rpcError(raw); problem != nil {
		return nil, problem
	}
	return rpcResult(raw), nil
}

func (t *httpTransport) Notify(ctx context.Context, method string, params any) error {
	_, err := t.post(ctx, nil, method, params)
	return err
}

func (t *httpTransport) post(ctx context.Context, id *int64, method string, params any) (json.RawMessage, error) {
	payload, err := encodeMessage(id, method, params)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	t.mu.Lock()
	session := t.sessionID
	t.mu.Unlock()
	if session != "" {
		request.Header.Set("Mcp-Session-Id", session)
	}
	for key, value := range t.headers {
		request.Header.Set(key, value)
	}

	response, err := t.http.Do(request)
	if err != nil {
		return nil, fail("mcp server '%s' error: %v", t.name, err)
	}
	defer response.Body.Close()
	if header := response.Header.Get("Mcp-Session-Id"); header != "" {
		t.mu.Lock()
		t.sessionID = header
		t.mu.Unlock()
	}
	if id == nil {
		return nil, nil
	}

	body, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fail("mcp server '%s' HTTP %d: %s", t.name, response.StatusCode, model.Truncate(string(body), 200))
	}
	if strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		for _, line := range strings.Split(string(body), "\n") {
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			candidate := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var envelope struct {
				ID *int64 `json:"id"`
			}
			if json.Unmarshal([]byte(candidate), &envelope) == nil && envelope.ID != nil && *envelope.ID == *id {
				return json.RawMessage(candidate), nil
			}
		}
		return nil, fail("mcp server '%s' sent no response for request %d", t.name, *id)
	}
	return json.RawMessage(body), nil
}

func (t *httpTransport) Close() {}
