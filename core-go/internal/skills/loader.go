package skills

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	SkillFileName        = "SKILL.md"
	MaxNameLength        = 64
	MaxDescriptionLength = 1024
)

var (
	namePattern = regexp.MustCompile(`^[a-z0-9-]+$`)
	xmlTag      = regexp.MustCompile(`<\s*/?\s*[A-Za-z][^>]*>`)
	reserved    = []string{"anthropic", "claude"}
)

// Load doc va kiem tra mot thu muc skill. Tra dung mot trong hai: skill hoac loi.
func Load(directory, source string) (*Definition, *Failure) {
	text, err := os.ReadFile(filepath.Join(directory, SkillFileName))
	if err != nil {
		return nil, &Failure{directory, source, "cannot read SKILL.md: " + err.Error()}
	}
	return Parse(string(text), directory, source)
}

func Parse(text, directory, source string) (*Definition, *Failure) {
	fail := func(message string) (*Definition, *Failure) {
		return nil, &Failure{directory, source, message}
	}

	frontmatter, body, ok := splitFrontmatter(text)
	if !ok {
		return fail("SKILL.md must start with a YAML frontmatter block delimited by '---'")
	}

	fields := parseFrontmatter(frontmatter)
	name := strings.TrimSpace(stringField(fields, "name"))
	description := strings.TrimSpace(stringField(fields, "description"))

	if problem := ValidateName(name); problem != "" {
		return fail(problem)
	}
	if problem := ValidateDescription(description); problem != "" {
		return fail(problem)
	}

	apps := []string{}
	switch value := fields["apps"].(type) {
	case []any:
		for _, item := range value {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				apps = append(apps, strings.TrimSpace(text))
			}
		}
	case []string:
		for _, item := range value {
			if strings.TrimSpace(item) != "" {
				apps = append(apps, strings.TrimSpace(item))
			}
		}
	case string:
		for _, part := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				apps = append(apps, trimmed)
			}
		}
	}

	return &Definition{
		Name: name, Description: description, Apps: apps, Directory: directory, Source: source,
		Body: strings.TrimSpace(body),
	}, nil
}

func ValidateName(name string) string {
	switch {
	case name == "":
		return "frontmatter 'name' is required"
	case len([]rune(name)) > MaxNameLength:
		return "'name' must be at most " + strconv.Itoa(MaxNameLength) + " characters"
	case !namePattern.MatchString(name):
		return "'name' may only contain lowercase letters, digits and '-'"
	}
	for _, word := range reserved {
		if strings.Contains(strings.ToLower(name), word) {
			return "'name' must not contain the reserved word '" + word + "'"
		}
	}
	return ""
}

func ValidateDescription(description string) string {
	switch {
	case description == "":
		return "frontmatter 'description' is required"
	case len([]rune(description)) > MaxDescriptionLength:
		return "'description' must be at most " + strconv.Itoa(MaxDescriptionLength) + " characters"
	case xmlTag.MatchString(description):
		return "'description' must not contain XML tags"
	}
	return ""
}

func splitFrontmatter(text string) (string, string, bool) {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.TrimPrefix(normalized, "\ufeff")
	if !strings.HasPrefix(normalized, "---\n") && normalized != "---" {
		return "", "", false
	}

	end := strings.Index(normalized[4:], "\n---")
	if end < 0 {
		return "", "", false
	}
	end += 4

	bodyStart := strings.Index(normalized[end+4:], "\n")
	if bodyStart < 0 {
		return normalized[4:end], "", true
	}
	return normalized[4:end], normalized[end+4+bodyStart+1:], true
}

// parseFrontmatter: tap con YAML du cho frontmatter cua skill - `key: value`, chuoi co nhay,
// khoi `>`/`>-`/`|`/`|-`, danh sach `[a, b]` va `- item`. Key long nhau / field la: bo qua.
func parseFrontmatter(text string) map[string]any {
	result := map[string]any{}
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); {
		line := strings.TrimRight(lines[i], "\r")
		if line == "" || strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") || isIndented(line) {
			i++
			continue
		}
		colon := strings.Index(line, ":")
		if colon <= 0 {
			i++
			continue
		}
		key := strings.TrimSpace(line[:colon])
		value := strings.TrimSpace(line[colon+1:])
		i++

		if value == ">" || value == ">-" || value == ">+" || value == "|" || value == "|-" || value == "|+" {
			block := []string{}
			for i < len(lines) && (strings.TrimSpace(lines[i]) == "" || isIndented(lines[i])) {
				block = append(block, strings.TrimSpace(lines[i]))
				i++
			}
			if strings.HasPrefix(value, ">") {
				kept := []string{}
				for _, item := range block {
					if item != "" {
						kept = append(kept, item)
					}
				}
				result[key] = strings.Join(kept, " ")
			} else {
				result[key] = strings.Trim(strings.Join(block, "\n"), "\n")
			}
			continue
		}

		if value == "" {
			// Danh sach "- item" hoac map long nhau (bo qua).
			items := []any{}
			isList := false
			for i < len(lines) && (strings.TrimSpace(lines[i]) == "" || isIndented(lines[i])) {
				inner := strings.TrimSpace(lines[i])
				if strings.HasPrefix(inner, "- ") || inner == "-" {
					isList = true
					items = append(items, unquote(strings.TrimSpace(strings.TrimPrefix(inner, "- "))))
				}
				i++
			}
			if isList {
				result[key] = items
			}
			continue
		}

		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			list := []any{}
			for _, part := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"), ",") {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					list = append(list, unquote(trimmed))
				}
			}
			result[key] = list
			continue
		}

		result[key] = unquote(value)
	}
	return result
}

func isIndented(line string) bool {
	return len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
}

func unquote(value string) string {
	if len(value) >= 2 {
		if value[0] == '"' && value[len(value)-1] == '"' {
			return strings.NewReplacer(`\"`, `"`, `\n`, "\n").Replace(value[1 : len(value)-1])
		}
		if value[0] == '\'' && value[len(value)-1] == '\'' {
			return strings.ReplaceAll(value[1:len(value)-1], "''", "'")
		}
	}
	return value
}

func stringField(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}
