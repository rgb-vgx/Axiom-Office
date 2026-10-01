package mcpserver

// Catalog dung danh sach tool theo nhom, dung thu tu dang ky cua McpHost.cs.
// office_sessions luon duoc them cuoi cung.
func Catalog(group string) ([]*Tool, bool) {
	tools := []*Tool{}
	switch group {
	case "word":
		tools = append(tools, wordFileTools()...)
		tools = append(tools, wordLiveTools()...)
	case "excel":
		tools = append(tools, excelFileTools()...)
		tools = append(tools, excelLiveTools()...)
	case "ppt", "powerpoint":
		tools = append(tools, pptFileTools()...)
		tools = append(tools, pptLiveTools()...)
	case "all":
		tools = append(tools, wordFileTools()...)
		tools = append(tools, wordLiveTools()...)
		tools = append(tools, excelFileTools()...)
		tools = append(tools, excelLiveTools()...)
		tools = append(tools, pptFileTools()...)
		tools = append(tools, pptLiveTools()...)
	default:
		return nil, false
	}
	tools = append(tools, sessionsTool())
	return tools, true
}
