package wpsbridge

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"axiomoffice/core/internal/office"
)

// Hop dong add-in WPS -> bridge -> nguoi doc trong Core, CA chuoi: body poll la file mau do tests/wps/test_contract_poll.mjs
// sinh tu main.js THAT (khong viet tay o day), day qua ServePoll, roi doc lai bang dung code Core dung khi chay:
// office.BridgeClient (LiveSession, Health, GetCommandsFor) va office.Directory (file session). Truoc 01/11/2026 moi
// dau co test rieng voi du lieu tu nghi ra, nen LiveSession doc `document` dang chuoi trong khi add-in gui object
// {name, fullName} ma khong test nao do (other_document khong bao gio thay tai lieu WPS khac).
func TestAddinPollContractReachesCoreReaders(t *testing.T) {
	cases := []struct{ app, file string }{{"wps", "poll-wps.json"}, {"et", "poll-et.json"}, {"wpp", "poll-wpp.json"}}
	b := startedBridge(t, nil)
	b.opts.PollSeconds = 0
	client := office.NewBridgeClient(&http.Client{Timeout: 5 * time.Second}, "tok")
	ctx := context.Background()

	ports := map[string]int{}
	bodies := map[string]map[string]any{}
	for _, c := range cases {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "wps", "contract", c.file))
		if err != nil {
			t.Fatalf("thieu file mau %s (sinh bang AXIOM_UPDATE_CONTRACT=1 node --test tests/wps/test_contract_poll.mjs): %v", c.file, err)
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		if body["app"] != c.app {
			t.Fatalf("%s: app %v, can %s", c.file, body["app"], c.app)
		}
		reply := pollAs(t, b, body)
		port := int(reply["port"].(float64))
		if port == 0 {
			t.Fatalf("%s: poll khong tra cong: %v", c.file, reply)
		}
		ports[c.app], bodies[c.app] = port, body
	}

	for _, c := range cases {
		body, port := bodies[c.app], ports[c.app]
		document := body["document"].(map[string]any)
		name, fullName := document["name"].(string), document["fullName"].(string)

		live := client.LiveSession(ctx, port)
		if live == nil || live.Document != name || live.DocumentPath != fullName || live.App != c.app || live.Port != port {
			t.Fatalf("%s: LiveSession doc sai hop dong: %+v (can document %q, path %q, app %s, port %d)", c.app, live, name, fullName, c.app, port)
		}
		health := client.Health(ctx, port)
		if health == nil || health["app"] != c.app {
			t.Fatalf("%s: /health %v", c.app, health)
		}
		catalog := client.GetCommandsFor(ctx, port, c.app, true)
		commands := body["commands"].([]any)
		if catalog == nil || len(catalog.Commands) != len(commands) {
			t.Fatalf("%s: catalog %+v, can %d lenh", c.app, catalog, len(commands))
		}
		for i, item := range commands {
			want := item.(map[string]any)
			got := catalog.Commands[i]
			if got.Name != want["name"] || len(got.Params) != len(want["params"].([]any)) {
				t.Fatalf("%s: lenh %d doc thanh %+v, can %v", c.app, i, got, want)
			}
			if !catalog.Has(want["name"].(string)) {
				t.Fatalf("%s: Has(%v) = false", c.app, want["name"])
			}
		}
	}

	b.tickSessions()
	sessions := office.NewDirectory(b.opts.SessionDir).List(true)
	if len(sessions) != len(cases) {
		t.Fatalf("Directory.List doc %d file session, can %d: %+v", len(sessions), len(cases), sessions)
	}
	for _, s := range sessions {
		body := bodies[s.App]
		if body == nil {
			t.Fatalf("file session app la %q: %+v", s.App, s)
		}
		document := body["document"].(map[string]any)
		if s.Port != ports[s.App] || s.Document != document["name"] || s.DocumentPath != document["fullName"] || s.Family != "wps-linux" {
			t.Fatalf("%s: file session doc sai hop dong: %+v (can %v)", s.App, s, document)
		}
	}
}
