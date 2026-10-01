// Package templates: template OOXML nhung san (default.docx / default.pptx) cho cac tool
// doc_create / ppt_create.
//
// Ban chinh nam o src/AxiomOffice.Host/Mcp/Templates (ban Windows net48 nhung tu do). Thu muc nay
// la BAN SAO do scripts/generate_templates.py sinh ra, vi go:embed khong doc duoc file ngoai module.
// CI chay `python scripts/generate_templates.py --check` de hai ben khong bi lech.
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
