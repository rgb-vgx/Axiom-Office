package mcpserver

// Handler tra ve gia tri cua tool: chuoi thi dung nguyen van lam text, kieu khac thi duoc
// JSON hoa. Tra ve error -> ket qua la {"ok":false,"error":...} voi isError=true, dung nhu
// nhanh catch trong McpServer.CallTool cua ban C#.
type Handler func(*ToolArgs) (any, error)

type Tool struct {
	Name        string
	Description string
	Handler     Handler
	schema      *orderedMap
}

func NewTool(name, description string, parameters []Param, handler Handler) *Tool {
	return &Tool{
		Name:        name,
		Description: description,
		Handler:     handler,
		schema:      Schema(parameters),
	}
}

func (t *Tool) InputSchema() *orderedMap {
	return t.schema
}
