package skills

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// MaxImportBytes: tran tong kich thuoc mot lan nhap skill (chong copy nham ca thu muc lon).
const MaxImportBytes = 64 << 20

// SourceNameExternal: nhan cua skill doc tu thu muc ngoai (chua nam trong thu muc skill nao cua Core).
const SourceNameExternal = "external"

// Candidate: mot skill tim thay trong thu muc ngoai, kem noi se copy toi.
type Candidate struct {
	Definition
	TargetDir string // noi skill se duoc copy toi (destRoot/<name>)
	Conflict  bool   // noi nhan da co thu muc cung ten
}

// ImportResult: ket qua mot lan nhap.
type ImportResult struct {
	Candidates []Candidate
	Imported   []string
	Replaced   []string // ten da ghi de (Conflict + overwrite)
}

// Inspect soi `path` - mot thu muc skill (co SKILL.md) hoac thu muc chua nhieu skill - va doi chieu voi
// `destRoot` (thuong la thu muc skill cua nguoi dung) de biet truoc se nhap gi va co trung ten khong.
// Loi tra ve (error) la loi duong dan; skill sai frontmatter tra trong failures, khong lam hong ca lan soi.
func Inspect(path, destRoot string) ([]Candidate, []Failure, error) {
	directories, err := skillDirectories(path)
	if err != nil {
		return nil, nil, err
	}

	candidates := []Candidate{}
	failures := []Failure{}
	seen := map[string]string{}
	for _, directory := range directories {
		skill, failure := Load(directory, SourceNameExternal)
		switch {
		case skill != nil:
			if other, duplicate := seen[skill.Name]; duplicate {
				failures = append(failures, Failure{Directory: directory, Source: SourceNameExternal,
					Error: "duplicate skill name '" + skill.Name + "' (also in " + filepath.Base(other) + ")"})
				continue
			}
			seen[skill.Name] = directory
			target := filepath.Join(destRoot, skill.Name)
			_, statErr := os.Stat(target)
			candidates = append(candidates, Candidate{Definition: *skill, TargetDir: target, Conflict: statErr == nil})
		case failure != nil:
			failures = append(failures, *failure)
		}
	}
	sort.Slice(candidates, func(a, b int) bool { return candidates[a].Name < candidates[b].Name })
	return candidates, failures, nil
}

// Import kiem roi copy skill tu `path` vao `destRoot`. Khong nhap mot phan: chi can mot skill sai frontmatter
// hoac mot ten da ton tai (khi overwrite=false) la ca lan nhap bi tu choi. Copy qua thu muc tam roi chuyen
// vao dich bang Rename, nen chi muc skill khong bao gio doc thay thu muc dang copy do.
func Import(path, destRoot string, overwrite bool) (ImportResult, error) {
	result := ImportResult{Candidates: []Candidate{}, Imported: []string{}, Replaced: []string{}}
	candidates, failures, err := Inspect(path, destRoot)
	if err != nil {
		return result, err
	}
	if len(failures) > 0 {
		return result, errors.New("skill folder has errors: " + failureText(failures) + " (nothing was imported)")
	}
	if len(candidates) == 0 {
		return result, errors.New("no skill to import")
	}
	conflicts := []string{}
	for _, candidate := range candidates {
		if candidate.Conflict && !overwrite {
			conflicts = append(conflicts, candidate.Name)
		}
	}
	if len(conflicts) > 0 {
		return result, errors.New("already exists in " + destRoot + ": " + strings.Join(conflicts, ", ") +
			" (set overwrite=true to replace)")
	}
	result.Candidates = candidates

	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return result, errors.New("cannot create the skills folder: " + err.Error())
	}
	// Thu muc tam nam canh `skills/` (khong nam trong do), nen chi muc khong quet thay.
	workRoot := filepath.Join(filepath.Dir(destRoot), ".import")
	if err := os.MkdirAll(workRoot, 0o755); err != nil {
		return result, errors.New("cannot create the import folder: " + err.Error())
	}

	for _, candidate := range candidates {
		staging, err := os.MkdirTemp(workRoot, "import-")
		if err != nil {
			return result, errors.New("cannot create a temporary folder: " + err.Error())
		}
		if _, err := copyTree(candidate.Directory, staging); err != nil {
			_ = os.RemoveAll(staging)
			return result, err
		}
		if candidate.Conflict {
			if err := os.RemoveAll(candidate.TargetDir); err != nil {
				_ = os.RemoveAll(staging)
				return result, errors.New("cannot replace skill '" + candidate.Name + "': " + err.Error())
			}
		}
		if err := os.Rename(staging, candidate.TargetDir); err != nil {
			_ = os.RemoveAll(staging)
			return result, errors.New("cannot move skill '" + candidate.Name + "' into place: " + err.Error())
		}
		if candidate.Conflict {
			result.Replaced = append(result.Replaced, candidate.Name)
		}
		result.Imported = append(result.Imported, candidate.Name)
	}
	_ = os.Remove(workRoot) // chi xoa duoc khi khong con gi ben trong
	return result, nil
}

// skillDirectories: `path` chinh la mot skill (co SKILL.md ngay trong do) hoac thu muc chua nhieu skill.
func skillDirectories(path string) ([]string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, errors.New("'path' is required")
	}
	info, err := os.Stat(trimmed)
	if err != nil {
		return nil, errors.New("folder not found: " + trimmed)
	}
	if !info.IsDir() {
		return nil, errors.New("'path' must be a folder: " + trimmed)
	}
	if isFile(filepath.Join(trimmed, SkillFileName)) {
		return []string{trimmed}, nil
	}

	entries, err := os.ReadDir(trimmed)
	if err != nil {
		return nil, errors.New("cannot read folder: " + trimmed)
	}
	directories := []string{}
	for _, entry := range entries {
		name := entry.Name()
		// Thu muc bat dau bang '_' la goi tai nguyen, '.' la thu muc an: khong phai skill.
		if !entry.IsDir() || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		directory := filepath.Join(trimmed, name)
		if isFile(filepath.Join(directory, SkillFileName)) {
			directories = append(directories, directory)
		}
	}
	sort.Slice(directories, func(a, b int) bool {
		return strings.ToLower(directories[a]) < strings.ToLower(directories[b])
	})
	if len(directories) == 0 {
		return nil, errors.New("no skill found in " + trimmed + " (a skill folder needs " + SkillFileName + ")")
	}
	return directories, nil
}

// copyTree copy cay thu muc; tu choi symlink va file khong phai file thuong; bo qua .git.
func copyTree(source, destination string) (int64, error) {
	var total int64
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(destination, relative), 0o755)
		}
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			return errors.New("symbolic links cannot be imported: " + filepath.ToSlash(relative))
		case !entry.Type().IsRegular():
			return errors.New("only regular files can be imported: " + filepath.ToSlash(relative))
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if total > MaxImportBytes {
			return errors.New("skill is larger than " + strconv.Itoa(MaxImportBytes>>20) + " MB")
		}
		return copyFile(path, filepath.Join(destination, relative))
	})
	return total, err
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// failureText: gop loi cua nhieu skill thanh mot dong (giong thong bao trong Reload).
func failureText(failures []Failure) string {
	parts := make([]string, 0, len(failures))
	for _, failure := range failures {
		parts = append(parts, filepath.Base(failure.Directory)+": "+failure.Error)
	}
	return strings.Join(parts, "; ")
}
