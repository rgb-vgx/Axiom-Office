package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxTextBytes: tran noi dung tra cho model khi doc file cua skill.
const MaxTextBytes = 64 * 1024

var textExtensions = map[string]bool{".md": true, ".txt": true, ".json": true, ".csv": true}

var binaryExtensions = map[string]bool{".docx": true, ".xlsx": true, ".pptx": true, ".png": true, ".jpg": true}

// AllowedExtensions liet ke cho thong bao loi (thu tu on dinh).
func AllowedExtensions() string {
	all := []string{}
	for _, extension := range []string{".md", ".txt", ".json", ".csv", ".docx", ".xlsx", ".pptx", ".png", ".jpg"} {
		all = append(all, extension)
	}
	return strings.Join(all, ", ")
}

func IsBinary(path string) bool {
	return binaryExtensions[strings.ToLower(filepath.Ext(path))]
}

// Resolve: chi doc file ben trong thu muc skill, chan '..', duong dan tuyet doi va symlink ra ngoai.
func Resolve(root, relative string) (string, string) {
	normalized := strings.TrimSpace(strings.ReplaceAll(relative, "\\", "/"))
	switch {
	case normalized == "":
		return "", "'path' is required"
	case strings.HasPrefix(normalized, "/") || strings.Contains(normalized, ":"):
		return "", "'path' must be a relative path inside the skill (no '..' or absolute paths)"
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return "", "'path' must be a relative path inside the skill (no '..' or absolute paths)"
		}
	}

	extension := strings.ToLower(filepath.Ext(normalized))
	if !textExtensions[extension] && !binaryExtensions[extension] {
		return "", "file type '" + extension + "' is not allowed (allowed: " + AllowedExtensions() + ")"
	}

	rootFull, err := filepath.Abs(root)
	if err != nil {
		return "", "cannot resolve the skill directory"
	}
	rootPrefix := strings.TrimRight(rootFull, string(filepath.Separator)) + string(filepath.Separator)
	full, err := filepath.Abs(filepath.Join(rootPrefix, filepath.FromSlash(normalized)))
	if err != nil || !within(full, rootPrefix) {
		return "", "'path' must stay inside the skill directory"
	}
	info, err := os.Lstat(full)
	if err != nil {
		return "", "file not found: " + normalized
	}

	// Symlink (file hoac thu muc tren duong dan) tro ra ngoai thu muc skill: tu choi.
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(full)
		if err != nil || !within(target, rootPrefix) {
			return "", "'path' is a link that points outside the skill directory"
		}
	}
	for folder := filepath.Dir(full); within(folder, rootPrefix) && folder != rootFull; folder = filepath.Dir(folder) {
		info, err := os.Lstat(folder)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", "'path' goes through a link to another directory"
		}
	}

	return full, ""
}

func within(path, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(filepath.Clean(path))+string(filepath.Separator), strings.ToLower(prefix))
}

// List: danh sach file cua skill (tru SKILL.md), duong dan kieu '/', chi phan mo rong doc duoc.
func List(directory string) []string {
	files := []string{}
	rootFull, err := filepath.Abs(directory)
	if err != nil {
		return files
	}
	prefix := strings.TrimRight(rootFull, string(filepath.Separator)) + string(filepath.Separator)

	var all []string
	_ = filepath.WalkDir(rootFull, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		all = append(all, path)
		return nil
	})
	sort.Slice(all, func(a, b int) bool { return strings.ToLower(all[a]) < strings.ToLower(all[b]) })

	for _, file := range all {
		relative := strings.ReplaceAll(strings.TrimPrefix(file, prefix), string(filepath.Separator), "/")
		extension := strings.ToLower(filepath.Ext(relative))
		if strings.EqualFold(relative, SkillFileName) || (!textExtensions[extension] && !binaryExtensions[extension]) {
			continue
		}
		files = append(files, relative)
	}
	return files
}
