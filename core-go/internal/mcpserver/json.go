// Package mcpserver: MCP server qua stdio (JSON-RPC 2.0, moi message mot dong).
//
// Port cua ban C# src/AxiomOffice.Host/Mcp/McpServer.cs + McpHost.cs cho ban Linux. Ban Windows
// van dung AxiomOffice.Host.exe mcp (net48) - xem core-go/README.md.
//
// Lan file (doc/ghi docx, xlsx, pptx, csv tren dia, khong can app dang mo) nam o filetools_word.go,
// filetools_excel.go, filetools_ppt.go; lan live (gui lenh toi bridge trong app dang mo) nam o livetools.go.
package mcpserver

import (
	"bytes"
	"encoding/json"
	"strings"
)

// decode doc JSON va giu so nguyen o dang json.Number thay vi ep het ve float64.
// JavaScriptSerializer ben C# phan biet long/double, ToolArgs dua vao do de tra ve dung kieu.
func decode(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

// encode khong escape HTML (<, >, &) va khong escape ky tu ngoai ASCII, giong
// JavaScriptEncoder.UnsafeRelaxedJsonEscaping ma ban .NET 10 dung (PortableJson.cs, da bo 01/10/2026).
func encode(value any) string {
	return string(encodeBytes(value))
}

func encodeBytes(value any) []byte {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return []byte("null")
	}
	// json.Encoder luon them '\n'; JSON-RPC tu them newline cua no.
	return bytes.TrimRight(buffer.Bytes(), "\n")
}
