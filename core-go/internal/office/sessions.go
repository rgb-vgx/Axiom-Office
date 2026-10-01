package office

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// healthProbeTimeout: tran thoi gian hoi /health cua tung session khi liet ke (giong ban C#).
const healthProbeTimeout = 1500 * time.Millisecond

// Sessions liet ke moi bridge dang song, them healthy/ageSeconds/file va don file cua bridge da chet.
// Sap xep theo family roi port - dung cho tool MCP `office_sessions`
// (port cua BridgeClient.Sessions trong src/AxiomOffice.Host/Mcp/BridgeClient.cs).
func (b *BridgeClient) Sessions(directory *Directory, prune bool) []map[string]any {
	found := []map[string]any{}
	entries, err := os.ReadDir(directory.Path)
	if err != nil {
		return found
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		file := filepath.Join(directory.Path, entry.Name())
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		session, ok := parseSession(data, file)
		if !ok {
			continue
		}
		alive := IsAlive(session.Pid)
		healthy := false
		if alive && session.Port > 0 {
			health := b.send(context.Background(), session.Port, "/health", nil, healthProbeTimeout)
			// /health tra {"ok":true,"result":{"pid":...}}; pid phai khop de tranh nham bridge khac.
			reportedPid, _ := health.Result["pid"].(float64)
			healthy = health.OK && int(reportedPid) == session.Pid
		}
		if !alive || (session.AgeSeconds > StaleSeconds && !healthy) {
			if prune {
				_ = os.Remove(file)
			}
			continue
		}
		found = append(found, map[string]any{
			"pid":          session.Pid,
			"app":          session.App,
			"family":       session.Family,
			"port":         session.Port,
			"host":         session.Host,
			"version":      session.Version,
			"ageSeconds":   session.AgeSeconds,
			"document":     nullable(session.Document),
			"documentPath": nullable(session.DocumentPath),
			"healthy":      healthy,
			"file":         session.File,
		})
	}
	sort.SliceStable(found, func(i, j int) bool {
		left, _ := found[i]["family"].(string)
		right, _ := found[j]["family"].(string)
		if left != right {
			return left < right
		}
		leftPort, _ := found[i]["port"].(int)
		rightPort, _ := found[j]["port"].(int)
		return leftPort < rightPort
	})
	return found
}

func nullable(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

// ageFromEpoch tinh tuoi theo giay, lam tron mot chu so thap phan nhu ban C#.
func ageFromEpoch(epoch float64, modified time.Time) float64 {
	if epoch <= 0 {
		epoch = float64(modified.UnixMilli()) / 1000.0
	}
	age := math.Max(0, float64(time.Now().UnixMilli())/1000.0-epoch)
	return math.Round(age*10) / 10
}
