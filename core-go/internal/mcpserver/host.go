package mcpserver

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// Run chay MCP server qua stdio: `AxiomOffice.Core mcp [all|word|excel|ppt] [--list]`.
// args la cac tham so SAU chu "mcp". Cung hinh dang voi `AxiomOffice.Host.exe mcp ...` cua ban
// Windows, nen core-go/internal/mcp co the dung thang Args ["mcp"] (xem mcp.LoadConfigs).
func Run(args []string) int {
	group := "all"
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			group = strings.ToLower(arg)
			break
		}
	}
	tools, ok := Catalog(group)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: AxiomOffice.Core mcp [all|word|excel|ppt] [--list]")
		return 2
	}
	name := group + "-tools"
	if group == "all" {
		name = "office-tools"
	}
	server := NewServer(name, tools)

	if slices.Contains(args, "--list") {
		for _, tool := range server.Tools() {
			fmt.Println(tool.Name)
		}
		fmt.Printf("%d tools\n", server.ToolCount())
		return 0
	}
	return server.Run(os.Stdin, os.Stdout)
}
