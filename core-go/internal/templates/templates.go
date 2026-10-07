// Package templates: template OOXML nhung san (default.docx / default.pptx) cho cac tool
// doc_create / ppt_create.
//
// Day la ban DUY NHAT cua hai template (ban C# trong src/AxiomOffice.Host/Mcp/Templates da xoa cung MCP
// server C#, dcf76fb). Nguon goc va giay phep MIT: NOTICE.md canh day.
package templates

import _ "embed"

//go:embed default.docx
var docxTemplate []byte

//go:embed default.pptx
var pptxTemplate []byte

// Docx tra ve bytes cua default.docx (tuong duong Templates.Docx cua ban C#).
func Docx() []byte { return docxTemplate }

// Pptx tra ve bytes cua default.pptx (tuong duong Templates.Pptx cua ban C#).
func Pptx() []byte { return pptxTemplate }
