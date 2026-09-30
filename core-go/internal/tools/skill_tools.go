package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"axiomoffice/core/internal/skills"
)

// LoadSkillTool: load_skill {name} -> noi dung SKILL.md + danh sach file dinh kem (tang 2 cua chuan
// Agent Skills, muc 8.4.3).
type LoadSkillTool struct {
	index   *skills.Index
	appKind string
}

const MaxListedFiles = 50

func NewLoadSkillTool(index *skills.Index, appKind string) *LoadSkillTool {
	return &LoadSkillTool{index: index, appKind: appKind}
}

func (t *LoadSkillTool) Name() string { return "load_skill" }

func (t *LoadSkillTool) Description() string {
	return "Load the full instructions of one skill from the 'Available skills' list in the system prompt. " +
		"Call it before starting a task that matches a skill description (creating or redesigning a document, " +
		"slide deck, report or table). Do not load skills for small edits such as making one line bold."
}

func (t *LoadSkillTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"name":{"type":"string","description":"Skill name exactly as listed"}},"required":["name"]}`)
}

func (t *LoadSkillTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	name := stringArg(arguments, "name")
	skill := t.index.Find(name)
	if skill == nil || !skill.AppliesTo(t.appKind) {
		available := []string{}
		for _, item := range t.index.ForApp(t.appKind) {
			available = append(available, item.Name)
		}
		list := "(none)"
		if len(available) > 0 {
			list = strings.Join(available, ", ")
		}
		return Result{JSON: ErrorJSON("unknown skill '" + name + "'; available skills: " + list)}
	}

	if run.Event != nil {
		run.Event("skill.loaded", map[string]any{"name": skill.Name, "source": skill.Source})
	}
	files := skills.List(skill.Directory)
	if len(files) > MaxListedFiles {
		files = files[:MaxListedFiles]
	}
	return Result{OK: true, JSON: OKJSON(map[string]any{
		"name":          skill.Name,
		"instructions":  skill.Body,
		"files":         files,
		"readFilesWith": "read_skill_file {name, path}",
	})}
}

// ReadSkillFileTool: read_skill_file {name, path} -> text (<= 64KB) hoac duong dan tuyet doi cho file nhi phan.
// name co the la mot skill hoac goi tai nguyen dung chung (vd "_design" -> tokens.json).
type ReadSkillFileTool struct {
	index   *skills.Index
	appKind string
}

func NewReadSkillFileTool(index *skills.Index, appKind string) *ReadSkillFileTool {
	return &ReadSkillFileTool{index: index, appKind: appKind}
}

func (t *ReadSkillFileTool) Name() string { return "read_skill_file" }

func (t *ReadSkillFileTool) Description() string {
	return "Read a file that belongs to a skill (for example references/..., examples/..., or tokens.json of the shared " +
		"'_design' pack). Text files return their content; templates (.docx/.xlsx/.pptx) and images return an absolute path."
}

func (t *ReadSkillFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"name":{"type":"string","description":"Skill name, or a shared pack such as _design"},` +
		`"path":{"type":"string","description":"Relative path inside the skill, e.g. references/the-thuc.md"}},` +
		`"required":["name","path"]}`)
}

func (t *ReadSkillFileTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	name := stringArg(arguments, "name")
	path := stringArg(arguments, "path")

	root := ""
	if skill := t.index.Find(name); skill != nil && skill.AppliesTo(t.appKind) {
		root = skill.Directory
	} else if strings.HasPrefix(name, "_") {
		root = t.index.ResourceDirectory(name)
	}
	if root == "" {
		return Result{JSON: ErrorJSON("unknown skill '" + name + "'")}
	}

	file, problem := skills.Resolve(root, path)
	if file == "" {
		return Result{JSON: ErrorJSON(problem)}
	}
	if skills.IsBinary(file) {
		return Result{OK: true, JSON: OKJSON(map[string]any{
			"name": name, "path": path, "absolutePath": file, "binary": true,
		})}
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return Result{JSON: ErrorJSON("cannot read the file: " + err.Error())}
	}
	truncated := len(data) > skills.MaxTextBytes
	if truncated {
		data = data[:skills.MaxTextBytes]
	}
	result := map[string]any{
		"name": name, "path": path,
		"content": strings.TrimPrefix(string(data), utf8BOM),
	}
	if truncated {
		result["truncated"] = true
	}
	return Result{OK: true, JSON: OKJSON(result)}
}
