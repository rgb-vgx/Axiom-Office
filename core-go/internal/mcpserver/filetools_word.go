package mcpserver

import (
	"axiomoffice/core/internal/docx"
)

// wordFileTools la lan file cua .docx (thay python-docx).
func wordFileTools() []*Tool {
	return []*Tool{
		NewTool("doc_profile",
			"Inspect a .docx file: paragraph/table/section counts, style usage, core properties.",
			[]Param{ParamStr("path", "", true, nil)},
			func(a *ToolArgs) (any, error) { return docx.Profile(a.Req("path")) }),
		NewTool("doc_get_text",
			"Extract all text from a .docx file (paragraphs + tables), optionally truncated.",
			[]Param{ParamStr("path", "", true, nil), ParamInt("max_chars", "", false, 0), ParamBool("include_tables", "", false, true)},
			func(a *ToolArgs) (any, error) {
				return docx.GetText(a.Req("path"), a.Int("max_chars", 0), a.Bool("include_tables", true))
			}),
		NewTool("doc_find_text",
			"Find paragraphs containing a query string in a .docx file.",
			[]Param{ParamStr("path", "", true, nil), ParamStr("query", "", true, nil), ParamInt("max_results", "", false, 50)},
			func(a *ToolArgs) (any, error) {
				return docx.FindText(a.Req("path"), a.Str("query", ""), a.Int("max_results", 50))
			}),
		NewTool("doc_extract_table",
			"Extract one table (by index) from a .docx file as a 2D array.",
			[]Param{ParamStr("path", "", true, nil), ParamInt("index", "", false, 0)},
			func(a *ToolArgs) (any, error) { return docx.ExtractTable(a.Req("path"), a.Int("index", 0)) }),
		NewTool("doc_create",
			"Create a .docx file. paragraphs items: \"text\" | {\"text\": \"...\", \"style\": \"Heading 1\"} | {\"table\": [[...]]}. Atomic save.",
			[]Param{ParamStr("path", "", true, nil), ParamArr("paragraphs", "", false, nil), ParamStr("title", "", false, nil), ParamStr("author", "", false, nil), ParamBool("overwrite", "", false, false)},
			func(a *ToolArgs) (any, error) {
				return docx.Create(a.Req("path"), a.Arr("paragraphs"), a.Str("title", ""), a.Str("author", ""), a.Bool("overwrite", false))
			}),
	}
}
