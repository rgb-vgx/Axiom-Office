package mcpserver

import (
	"axiomoffice/core/internal/pptx"
)

// pptFileTools la lan file cua .pptx (thay python-pptx).
func pptFileTools() []*Tool {
	return []*Tool{
		NewTool("ppt_profile",
			"Inspect a .pptx file: slide count, slide size and aspect (16:9, 4:3), per-slide layout, shapes, texts, "+
				"speaker notes, and its style (theme colors/fonts plus the colors and fonts actually used on the slides - "+
				"follow usedColors/usedFonts when the file is a template), and for a template its slide kinds: per kind the "+
				"text slots (shape name, size, maxChars), repeated items, the fixed background, the safe area and the box for "+
				"a picture - use them with wpp.addSlideFromTemplate / wpp.setShapeText / wpp.copyPicture.",
			[]Param{ParamStr("path", "", true, nil)},
			func(a *ToolArgs) (any, error) { return pptx.Profile(a.Req("path")) }),
		NewTool("ppt_get_text",
			"Extract all text (slides + notes) from a .pptx file.",
			[]Param{ParamStr("path", "", true, nil), ParamInt("max_chars", "", false, 0)},
			func(a *ToolArgs) (any, error) { return pptx.GetText(a.Req("path"), a.Int("max_chars", 0)) }),
		NewTool("ppt_create",
			"Create a .pptx file. slides items: {\"title\": \"...\", \"bullets\": [\"...\", \"...\"], \"layout\": 1}. Atomic save.",
			[]Param{ParamStr("path", "", true, nil), ParamArr("slides", "", false, nil), ParamBool("overwrite", "", false, false)},
			func(a *ToolArgs) (any, error) {
				return pptx.Create(a.Req("path"), a.Arr("slides"), a.Bool("overwrite", false))
			}),
		NewTool("ppt_add_slide_file",
			"Add a slide to a .pptx file on disk (offline mode).",
			[]Param{ParamStr("path", "", true, nil), ParamStr("title", "", false, nil), ParamArr("bullets", "", false, nil), ParamInt("layout", "", false, 1)},
			func(a *ToolArgs) (any, error) {
				return pptx.AddSlideFile(a.Req("path"), a.Str("title", ""), a.Arr("bullets"), a.Int("layout", 1))
			}),
	}
}
