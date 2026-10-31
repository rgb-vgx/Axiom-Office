package skills

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/filesafe"
)

// Store: ghi skill vao thu muc CA NHAN (nguon "user") - nhap tu ngoai, tao, sua, xoa. Man hinh "Ky nang" cua pane
// goi qua api/skills.go.
//
// Moi thay doi chi o thu muc ca nhan: sua skill san co / cua to chuc thi chep ca thu muc sang thu muc ca nhan roi
// sua ban chep (ban ca nhan che ban goc vi nguon sau ghi de nguon truoc - Index.Reload). Ban goc giu nguyen nen
// cai lai/nang cap khong xoa chinh sua, va "Khoi phuc ban goc" = xoa ban ca nhan. Xoa = chuyen vao thung rac
// (skills-trash canh thu muc ca nhan), khong xoa han.
type Store struct {
	Index *Index
	// Now: dong ho cho ten thu muc thung rac (test thay duoc).
	Now func() time.Time
}

// Gioi han khi nhap: goi skill la tai lieu + vai file tham chieu, khong phai kho du lieu.
const (
	importMaxFiles     = 300
	importMaxFileBytes = 5 << 20
	importMaxBytes     = 30 << 20
)

// StoreError: loi cho giao dien - Code may doc duoc, Message tieng Viet hien thang cho nguoi dung.
type StoreError struct {
	Code    string
	Message string
}

func (e *StoreError) Error() string { return e.Message }

func storeError(code, message string) *StoreError { return &StoreError{Code: code, Message: message} }

// FileInfo: mot file trong thu muc skill (tru SKILL.md).
type FileInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Text bool   `json:"text"`
}

// Detail: du lieu man hinh sua mot skill.
type Detail struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Apps        []string   `json:"apps"`
	Body        string     `json:"body"`
	Source      string     `json:"source"`
	Directory   string     `json:"directory"`
	Files       []FileInfo `json:"files"`
	// Overrides: day la ban ca nhan dang che ban goc (OriginalSource) - xoa ban ca nhan thi ban goc quay lai.
	Overrides      bool   `json:"overrides"`
	OriginalSource string `json:"originalSource,omitempty"`
}

// Edit: noi dung moi cua mot skill (tao hoac sua). Files: file van ban them/sua trong thu muc skill.
type Edit struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Apps        []string          `json:"apps"`
	Body        string            `json:"body"`
	Files       map[string]string `json:"files"`
}

func (s *Store) userDir() (string, *StoreError) {
	dir := strings.TrimSpace(s.Index.SourceDir("user"))
	if dir == "" {
		return "", storeError("no-user-dir", "Core chưa có thư mục kỹ năng cá nhân.")
	}
	return dir, nil
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Get: chi tiet mot skill dang nap.
func (s *Store) Get(name string) (*Detail, *StoreError) {
	definition := s.Index.Find(name)
	if definition == nil {
		return nil, storeError("not-found", "Không có kỹ năng \""+strings.TrimSpace(name)+"\".")
	}
	apps := append([]string{}, definition.Apps...)
	detail := &Detail{
		Name: definition.Name, Description: definition.Description, Apps: apps, Body: definition.Body,
		Source: definition.Source, Directory: definition.Directory, Files: listFiles(definition.Directory),
	}
	if original, ok := s.Index.Shadowed(definition.Name); ok && definition.Source == "user" {
		detail.Overrides = true
		detail.OriginalSource = original.Source
	}
	return detail, nil
}

// ReadFile: noi dung mot file van ban cua skill (de sua tren giao dien).
func (s *Store) ReadFile(name, relative string) (string, *StoreError) {
	definition := s.Index.Find(name)
	if definition == nil {
		return "", storeError("not-found", "Không có kỹ năng \""+strings.TrimSpace(name)+"\".")
	}
	full, problem := Resolve(definition.Directory, relative)
	if problem != "" {
		return "", storeError("bad-path", "Không đọc được tệp: "+problem)
	}
	if IsBinary(full) {
		return "", storeError("binary", "Tệp này không phải văn bản, không sửa trên giao diện được.")
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", storeError("bad-path", "Không đọc được tệp: "+err.Error())
	}
	if len(data) > MaxTextBytes {
		return "", storeError("too-large", "Tệp lớn hơn "+strconv.Itoa(MaxTextBytes/1024)+" KB, không sửa trên giao diện được.")
	}
	return string(data), nil
}

// Create: kỹ năng moi trong thu muc ca nhan. Trung ten mot skill dang co (o bat ky nguon nao) thi tu choi - muon
// sua skill do thi dung Save.
func (s *Store) Create(edit Edit) (*Detail, *StoreError) {
	dir, problem := s.userDir()
	if problem != nil {
		return nil, problem
	}
	name := strings.TrimSpace(edit.Name)
	if message := vietnameseProblem(ValidateName(name)); message != "" {
		return nil, storeError("invalid", message)
	}
	if s.Index.Find(name) != nil {
		return nil, storeError("exists", "Đã có kỹ năng tên \""+name+"\" — mở nó và bấm Sửa.")
	}
	target := filepath.Join(dir, name)
	if pathExists(target) {
		return nil, storeError("exists", "Thư mục "+target+" đã có sẵn — đổi tên khác.")
	}
	text, failure := composeSkill("", name, edit)
	if failure != nil {
		return nil, failure
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, storeError("io", "Không tạo được thư mục kỹ năng: "+err.Error())
	}
	if err := filesafe.WriteAtomic(filepath.Join(target, SkillFileName), []byte(text)); err != nil {
		_ = os.RemoveAll(target)
		return nil, storeError("io", "Không ghi được SKILL.md: "+err.Error())
	}
	if failure := writeFiles(target, edit.Files); failure != nil {
		return nil, failure
	}
	s.Index.Reload()
	return s.Get(name)
}

// Save: sua skill `name`. Skill khong phai cua nguoi dung thi chep sang thu muc ca nhan truoc (ban goc giu nguyen).
func (s *Store) Save(name string, edit Edit) (*Detail, *StoreError) {
	definition := s.Index.Find(name)
	if definition == nil {
		return nil, storeError("not-found", "Không có kỹ năng \""+strings.TrimSpace(name)+"\".")
	}
	if strings.TrimSpace(edit.Name) != "" && strings.TrimSpace(edit.Name) != definition.Name {
		return nil, storeError("rename", "Không đổi tên kỹ năng khi sửa — tạo kỹ năng mới với tên khác rồi xoá bản cũ.")
	}
	original, err := os.ReadFile(filepath.Join(definition.Directory, SkillFileName))
	if err != nil {
		return nil, storeError("io", "Không đọc được SKILL.md hiện tại: "+err.Error())
	}
	// Kiem noi dung moi TRUOC khi dung toi dia: loi thi khong de lai ban chep do dang.
	text, failure := composeSkill(string(original), definition.Name, edit)
	if failure != nil {
		return nil, failure
	}
	for relative := range edit.Files {
		if problem := checkEditablePath(relative); problem != nil {
			return nil, problem
		}
	}

	directory := definition.Directory
	if definition.Source != "user" {
		dir, problem := s.userDir()
		if problem != nil {
			return nil, problem
		}
		directory = filepath.Join(dir, definition.Name)
		if pathExists(directory) {
			// Thu muc ca nhan cung ten nhung khong nap duoc (SKILL.md hong): khong ghi de mu.
			return nil, storeError("exists", "Thư mục "+directory+" đã có nhưng không nạp được — sửa hoặc xoá nó trước.")
		}
		if failure := copySkill(definition.Directory, directory); failure != nil {
			_ = os.RemoveAll(directory)
			return nil, failure
		}
	}
	if err := filesafe.WriteAtomic(filepath.Join(directory, SkillFileName), []byte(text)); err != nil {
		return nil, storeError("io", "Không ghi được SKILL.md: "+err.Error())
	}
	if failure := writeFiles(directory, edit.Files); failure != nil {
		return nil, failure
	}
	s.Index.Reload()
	return s.Get(definition.Name)
}

// Delete: chuyen ban ca nhan vao thung rac. Skill khong thuoc nguon ca nhan thi tu choi. restored = ban goc (san co /
// to chuc) quay lai sau khi xoa.
func (s *Store) Delete(name string) (trash string, restored bool, failure *StoreError) {
	definition := s.Index.Find(name)
	if definition == nil {
		return "", false, storeError("not-found", "Không có kỹ năng \""+strings.TrimSpace(name)+"\".")
	}
	if definition.Source != "user" {
		return "", false, storeError("not-user", "Chỉ xoá được kỹ năng cá nhân; kỹ năng \""+sourceName(definition.Source)+"\" đi kèm bộ cài.")
	}
	_, restored = s.Index.Shadowed(definition.Name)
	dir, problem := s.userDir()
	if problem != nil {
		return "", false, problem
	}
	trashRoot := filepath.Join(filepath.Dir(filepath.Clean(dir)), "skills-trash")
	if err := os.MkdirAll(trashRoot, 0o755); err != nil {
		return "", false, storeError("io", "Không tạo được thùng rác: "+err.Error())
	}
	trash = filepath.Join(trashRoot, definition.Name+"-"+s.now().Format("20060102-150405"))
	if err := os.Rename(definition.Directory, trash); err != nil {
		return "", false, storeError("io", "Không chuyển được vào thùng rác: "+err.Error())
	}
	s.Index.Reload()
	return trash, restored, nil
}

// ---- Nhap tu ngoai ----

// ImportItem: mot skill tim thay trong goi nhap.
type ImportItem struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Apps        []string `json:"apps"`
	Files       int      `json:"files"`
	// Skipped: file khong dung duoc (duoi la, vd script .py) - bo qua khi nhap.
	Skipped []string `json:"skipped"`
	// Conflict: "" | "user" (da co ban ca nhan cung ten - can overwrite) | "override" (se che ban san co/to chuc).
	Conflict string `json:"conflict,omitempty"`
	Error    string `json:"error,omitempty"`
	Imported bool   `json:"imported"`
}

// ImportResult: ket qua xem truoc / nhap.
type ImportResult struct {
	Source string       `json:"source"`
	Items  []ImportItem `json:"items"`
}

// importEntry: mot file trong goi (thu muc hoac zip), duong dan tuong doi tu goc skill.
type importEntry struct {
	relative string
	size     int64
	open     func() (io.ReadCloser, error)
}

type importCandidate struct {
	root    string
	entries []importEntry
}

// Import: nhap skill tu mot thu muc (co SKILL.md, hoac chua cac thu muc skill) hoac tep .zip/.skill. dryRun chi
// xem truoc. overwrite: thay ban ca nhan cung ten (ban cu vao thung rac).
func (s *Store) Import(path string, dryRun, overwrite bool) (*ImportResult, *StoreError) {
	path = expandHome(strings.TrimSpace(path))
	if path == "" {
		return nil, storeError("bad-path", "Nhập đường dẫn tới thư mục kỹ năng hoặc tệp .zip.")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, storeError("bad-path", "Không tìm thấy "+path+".")
	}
	var candidates []importCandidate
	var failure *StoreError
	var closer io.Closer
	switch {
	case info.IsDir():
		candidates, failure = folderCandidates(path)
	case strings.EqualFold(filepath.Ext(path), ".zip") || strings.EqualFold(filepath.Ext(path), ".skill"):
		var reader *zip.ReadCloser
		reader, err = zip.OpenReader(path)
		if err != nil {
			return nil, storeError("bad-zip", "Không mở được tệp nén: "+err.Error())
		}
		closer = reader
		candidates, failure = zipCandidates(&reader.Reader)
	default:
		return nil, storeError("bad-path", "Chỉ nhập được thư mục, tệp .zip hoặc .skill.")
	}
	if closer != nil {
		defer closer.Close()
	}
	if failure != nil {
		return nil, failure
	}
	if len(candidates) == 0 {
		return nil, storeError("empty", "Không thấy SKILL.md nào trong "+path+".")
	}

	dir, problem := s.userDir()
	if problem != nil {
		return nil, problem
	}
	result := &ImportResult{Source: path}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		item := s.inspect(candidate, dir)
		if item.Error == "" && seen[item.Name] {
			item.Error = "Gói có hai kỹ năng cùng tên \"" + item.Name + "\"."
		}
		seen[item.Name] = true
		if !dryRun && item.Error == "" && (item.Conflict != "user" || overwrite) {
			if message := s.install(candidate, item.Name, dir); message != "" {
				item.Error = message
			} else {
				item.Imported = true
			}
		}
		result.Items = append(result.Items, item)
	}
	if !dryRun {
		s.Index.Reload()
	}
	return result, nil
}

func (s *Store) inspect(candidate importCandidate, userDir string) ImportItem {
	item := ImportItem{Apps: []string{}, Skipped: []string{}}
	var skillText string
	for _, entry := range candidate.entries {
		if entry.relative == SkillFileName {
			data, err := readEntry(entry)
			if err != nil {
				item.Error = "Không đọc được SKILL.md: " + err.Error()
				return item
			}
			skillText = string(data)
			continue
		}
		if usableFile(entry.relative) {
			item.Files++
		} else {
			item.Skipped = append(item.Skipped, entry.relative)
		}
	}
	definition, failure := Parse(skillText, candidate.root, "import")
	if failure != nil {
		item.Name = filepath.Base(candidate.root)
		item.Error = vietnameseProblem(failure.Error)
		return item
	}
	item.Name, item.Description = definition.Name, definition.Description
	item.Apps = append(item.Apps, definition.Apps...)
	if existing := s.Index.Find(definition.Name); existing != nil {
		if existing.Source == "user" {
			item.Conflict = "user"
		} else {
			item.Conflict = "override"
		}
	} else if pathExists(filepath.Join(userDir, definition.Name)) {
		item.Conflict = "user"
	}
	return item
}

// install: giai vao thu muc tam canh thu muc ca nhan roi doi ten - khong bao gio de lai skill do dang.
func (s *Store) install(candidate importCandidate, name, userDir string) string {
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return "Không tạo được thư mục kỹ năng cá nhân: " + err.Error()
	}
	staging, err := os.MkdirTemp(userDir, ".import-")
	if err != nil {
		return "Không tạo được thư mục tạm: " + err.Error()
	}
	defer os.RemoveAll(staging)
	for _, entry := range candidate.entries {
		if entry.relative != SkillFileName && !usableFile(entry.relative) {
			continue
		}
		target := filepath.Join(staging, filepath.FromSlash(entry.relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "Không ghi được " + entry.relative + ": " + err.Error()
		}
		data, err := readEntry(entry)
		if err != nil {
			return "Không đọc được " + entry.relative + ": " + err.Error()
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return "Không ghi được " + entry.relative + ": " + err.Error()
		}
	}
	target := filepath.Join(userDir, name)
	if pathExists(target) {
		trashRoot := filepath.Join(filepath.Dir(filepath.Clean(userDir)), "skills-trash")
		if err := os.MkdirAll(trashRoot, 0o755); err != nil {
			return "Không tạo được thùng rác: " + err.Error()
		}
		if err := os.Rename(target, filepath.Join(trashRoot, name+"-"+s.now().Format("20060102-150405"))); err != nil {
			return "Không chuyển bản cũ vào thùng rác: " + err.Error()
		}
	}
	if err := os.Rename(staging, target); err != nil {
		return "Không đặt được kỹ năng vào thư mục cá nhân: " + err.Error()
	}
	return ""
}

// folderCandidates: thu muc co SKILL.md la mot skill; khong thi tim thu muc skill o hai tang ben duoi.
func folderCandidates(root string) ([]importCandidate, *StoreError) {
	roots := []string{}
	if filesafe.Exists(filepath.Join(root, SkillFileName)) {
		roots = append(roots, root)
	} else {
		for _, pattern := range []string{"*", filepath.Join("*", "*")} {
			matches, _ := filepath.Glob(filepath.Join(root, pattern, SkillFileName))
			sort.Strings(matches)
			for _, match := range matches {
				candidate := filepath.Dir(match)
				if !insideAny(candidate, roots) && !strings.HasPrefix(filepath.Base(candidate), "_") {
					roots = append(roots, candidate)
				}
			}
		}
	}
	candidates := []importCandidate{}
	total := int64(0)
	count := 0
	for _, skillRoot := range roots {
		candidate := importCandidate{root: skillRoot}
		var failure *StoreError
		_ = filepath.WalkDir(skillRoot, func(path string, entry os.DirEntry, err error) error {
			if err != nil || failure != nil {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil // link: khong theo, khong chep
			}
			if entry.IsDir() {
				if path != skillRoot && filesafe.Exists(filepath.Join(path, SkillFileName)) {
					return filepath.SkipDir // skill long nhau: la mot ung vien rieng
				}
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			relative, _ := filepath.Rel(skillRoot, path)
			count++
			total += info.Size()
			if failure = checkLimits(count, info.Size(), total, filepath.ToSlash(relative)); failure != nil {
				return nil
			}
			full := path
			candidate.entries = append(candidate.entries, importEntry{relative: filepath.ToSlash(relative), size: info.Size(),
				open: func() (io.ReadCloser, error) { return os.Open(full) }})
			return nil
		})
		if failure != nil {
			return nil, failure
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

// zipCandidates: moi thu muc trong zip co SKILL.md la mot skill (thuong mot thu muc goc). Chan zip-slip, link.
func zipCandidates(reader *zip.Reader) ([]importCandidate, *StoreError) {
	type file struct {
		name string
		zf   *zip.File
	}
	files := []file{}
	total := int64(0)
	for _, zf := range reader.File {
		name := strings.ReplaceAll(zf.Name, "\\", "/")
		if strings.HasSuffix(name, "/") || zf.FileInfo().IsDir() {
			continue
		}
		if zf.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if strings.HasPrefix(name, "/") || strings.Contains(name, ":") || hasDotDot(name) {
			return nil, storeError("bad-zip", "Tệp nén có đường dẫn không an toàn: "+name)
		}
		if strings.HasPrefix(name, "__MACOSX/") || filepath.Base(name) == ".DS_Store" {
			continue
		}
		total += int64(zf.UncompressedSize64)
		if failure := checkLimits(len(files)+1, int64(zf.UncompressedSize64), total, name); failure != nil {
			return nil, failure
		}
		files = append(files, file{name, zf})
	}
	roots := []string{}
	for _, f := range files {
		if filepath.Base(f.name) == SkillFileName {
			roots = append(roots, strings.TrimSuffix(strings.TrimSuffix(f.name, SkillFileName), "/"))
		}
	}
	sort.Slice(roots, func(a, b int) bool {
		return len(roots[a]) < len(roots[b]) || (len(roots[a]) == len(roots[b]) && roots[a] < roots[b])
	})
	candidates := []importCandidate{}
	for _, root := range roots {
		candidate := importCandidate{root: root}
		for _, f := range files {
			var relative string
			switch {
			case root == "":
				relative = f.name
			case strings.HasPrefix(f.name, root+"/"):
				relative = strings.TrimPrefix(f.name, root+"/")
			default:
				continue
			}
			// File thuoc mot skill long nhau sau hon: de cho ung vien do.
			if belongsDeeper(f.name, root, roots) {
				continue
			}
			zf := f.zf
			candidate.entries = append(candidate.entries, importEntry{relative: relative, size: int64(zf.UncompressedSize64),
				open: func() (io.ReadCloser, error) { return zf.Open() }})
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func belongsDeeper(name, root string, roots []string) bool {
	for _, other := range roots {
		if len(other) > len(root) && (root == "" || strings.HasPrefix(other, root+"/")) && strings.HasPrefix(name, other+"/") {
			return true
		}
	}
	return false
}

func checkLimits(count int, size, total int64, name string) *StoreError {
	switch {
	case count > importMaxFiles:
		return storeError("too-large", "Gói có hơn "+strconv.Itoa(importMaxFiles)+" tệp — không giống một kỹ năng.")
	case size > importMaxFileBytes:
		return storeError("too-large", "Tệp "+name+" lớn hơn "+strconv.Itoa(importMaxFileBytes>>20)+" MB.")
	case total > importMaxBytes:
		return storeError("too-large", "Gói lớn hơn "+strconv.Itoa(importMaxBytes>>20)+" MB.")
	}
	return nil
}

func readEntry(entry importEntry) ([]byte, error) {
	reader, err := entry.open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	// Doc toi da kich thuoc khai bao + 1 byte: zip noi doi kich thuoc thi khong phinh vo han.
	data, err := io.ReadAll(io.LimitReader(reader, importMaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > importMaxFileBytes {
		return nil, storeError("too-large", entry.relative+" lớn hơn khai báo")
	}
	return data, nil
}

// usableFile: file Axiom doc duoc (read_skill_file) - con lai (script, file an) bo qua khi nhap.
func usableFile(relative string) bool {
	base := filepath.Base(relative)
	if strings.HasPrefix(base, ".") {
		return false
	}
	extension := strings.ToLower(filepath.Ext(relative))
	return textExtensions[extension] || binaryExtensions[extension]
}

// pathExists: duong dan ton tai (file, thu muc hay link) - filesafe.Exists chi nhan file thuong.
func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func hasDotDot(name string) bool {
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func insideAny(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

// ---- Ghi SKILL.md ----

// composeSkill: SKILL.md moi tu frontmatter cu (giu nguyen thu tu va cac khoa la nhu license, metadata) + noi dung sua.
// Kiem bang chinh bo doc Parse - ghi xong chac chan nap duoc.
func composeSkill(original, name string, edit Edit) (string, *StoreError) {
	description := strings.Join(strings.Fields(edit.Description), " ")
	if message := vietnameseProblem(ValidateDescription(description)); message != "" {
		return "", storeError("invalid", message)
	}
	apps := []string{}
	for _, app := range edit.Apps {
		app = strings.TrimSpace(app)
		switch app {
		case "":
		case "wps", "et", "wpp":
			apps = append(apps, app)
		default:
			return "", storeError("invalid", "Ứng dụng \""+app+"\" không hợp lệ (wps, et, wpp).")
		}
	}

	frontmatter, _, _ := splitFrontmatter(original)
	blocks := frontmatterBlocks(frontmatter)
	replace := map[string]string{
		"name":        "name: " + name,
		"description": foldedDescription(description),
		"apps":        "",
	}
	if len(apps) > 0 {
		replace["apps"] = "apps: [" + strings.Join(apps, ", ") + "]"
	}
	lines := []string{}
	done := map[string]bool{}
	for _, block := range blocks {
		if value, ok := replace[block.key]; ok {
			if !done[block.key] && value != "" {
				lines = append(lines, value)
			}
			done[block.key] = true
			continue
		}
		lines = append(lines, block.text)
	}
	for _, key := range []string{"name", "description", "apps"} {
		if !done[key] && replace[key] != "" {
			// Khoa moi: name, description len dau theo thu tu quen thuoc.
			if key == "apps" {
				lines = append(lines, replace[key])
			} else {
				lines = append([]string{replace[key]}, lines...)
			}
		}
	}
	// name truoc description khi ca hai deu moi them.
	if !done["name"] && !done["description"] && len(lines) >= 2 && strings.HasPrefix(lines[0], "description:") {
		lines[0], lines[1] = lines[1], lines[0]
	}
	body := strings.TrimSpace(strings.ReplaceAll(edit.Body, "\r\n", "\n"))
	text := "---\n" + strings.Join(lines, "\n") + "\n---\n\n" + body + "\n"
	if _, failure := Parse(text, "", "check"); failure != nil {
		return "", storeError("invalid", vietnameseProblem(failure.Error))
	}
	return text, nil
}

type frontmatterBlock struct {
	key  string
	text string
}

// frontmatterBlocks: chia frontmatter thanh khoi theo khoa cap mot (dong khong thut le "key:") kem cac dong thut le
// sau no. Dong dau tien khong thuoc khoa nao (comment...) giu nguyen voi key rong.
func frontmatterBlocks(frontmatter string) []frontmatterBlock {
	blocks := []frontmatterBlock{}
	if strings.TrimSpace(frontmatter) == "" {
		return blocks
	}
	for _, line := range strings.Split(strings.ReplaceAll(frontmatter, "\r\n", "\n"), "\n") {
		colon := strings.Index(line, ":")
		top := line != "" && !isIndented(line) && !strings.HasPrefix(strings.TrimSpace(line), "#") && colon > 0
		if top || len(blocks) == 0 {
			key := ""
			if top {
				key = strings.TrimSpace(line[:colon])
			}
			blocks = append(blocks, frontmatterBlock{key: key, text: line})
			continue
		}
		blocks[len(blocks)-1].text += "\n" + line
	}
	// Bo dong trong o cuoi tung khoi (de ghi lai gon).
	for i := range blocks {
		blocks[i].text = strings.TrimRight(blocks[i].text, "\n ")
	}
	return blocks
}

// foldedDescription: `description: >-` + cac dong thut le <= 100 ky tu - an toan voi moi ky tu (nhay, '\', ':').
func foldedDescription(description string) string {
	lines := []string{"description: >-"}
	current := ""
	for _, word := range strings.Fields(description) {
		if current != "" && len([]rune(current))+1+len([]rune(word)) > 100 {
			lines = append(lines, "  "+current)
			current = word
			continue
		}
		if current == "" {
			current = word
		} else {
			current += " " + word
		}
	}
	if current != "" {
		lines = append(lines, "  "+current)
	}
	return strings.Join(lines, "\n")
}

// ---- File cua skill ----

func listFiles(directory string) []FileInfo {
	files := []FileInfo{}
	for _, relative := range List(directory) {
		info, err := os.Stat(filepath.Join(directory, filepath.FromSlash(relative)))
		if err != nil {
			continue
		}
		files = append(files, FileInfo{Path: relative, Size: info.Size(), Text: !IsBinary(relative)})
	}
	return files
}

// checkEditablePath: file van ban sua tren giao dien: duong dan tuong doi, khong '..', duoi van ban, khong la SKILL.md.
func checkEditablePath(relative string) *StoreError {
	normalized := strings.TrimSpace(strings.ReplaceAll(relative, "\\", "/"))
	switch {
	case normalized == "" || strings.HasPrefix(normalized, "/") || strings.Contains(normalized, ":") || hasDotDot(normalized):
		return storeError("bad-path", "Đường dẫn tệp \""+relative+"\" không hợp lệ (phải nằm trong thư mục kỹ năng).")
	case strings.EqualFold(normalized, SkillFileName):
		return storeError("bad-path", "Sửa SKILL.md qua các ô Mô tả / Nội dung.")
	case !textExtensions[strings.ToLower(filepath.Ext(normalized))]:
		return storeError("bad-path", "Chỉ sửa được tệp văn bản (.md, .txt, .json, .csv): "+relative)
	}
	return nil
}

func writeFiles(directory string, files map[string]string) *StoreError {
	names := make([]string, 0, len(files))
	for relative := range files {
		names = append(names, relative)
	}
	sort.Strings(names)
	for _, relative := range names {
		if problem := checkEditablePath(relative); problem != nil {
			return problem
		}
		content := files[relative]
		if len(content) > MaxTextBytes {
			return storeError("too-large", relative+" lớn hơn "+strconv.Itoa(MaxTextBytes/1024)+" KB.")
		}
		target := filepath.Join(directory, filepath.FromSlash(strings.TrimSpace(strings.ReplaceAll(relative, "\\", "/"))))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return storeError("io", "Không ghi được "+relative+": "+err.Error())
		}
		if err := filesafe.WriteAtomic(target, []byte(content)); err != nil {
			return storeError("io", "Không ghi được "+relative+": "+err.Error())
		}
	}
	return nil
}

// copySkill: chep thu muc skill (chi file doc duoc, khong theo link) sang thu muc moi.
func copySkill(from, to string) *StoreError {
	var failure *StoreError
	_ = filepath.WalkDir(from, func(path string, entry os.DirEntry, err error) error {
		if err != nil || failure != nil || entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil
		}
		relative, _ := filepath.Rel(from, path)
		if filepath.Base(relative) != SkillFileName && !usableFile(relative) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			failure = storeError("io", "Không chép được "+relative+": "+err.Error())
			return nil
		}
		target := filepath.Join(to, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			failure = storeError("io", "Không chép được "+relative+": "+err.Error())
			return nil
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			failure = storeError("io", "Không chép được "+relative+": "+err.Error())
		}
		return nil
	})
	return failure
}

func sourceName(source string) string {
	switch source {
	case "builtin":
		return "sẵn có"
	case "org":
		return "tổ chức"
	case "user":
		return "cá nhân"
	}
	return source
}

// vietnameseProblem: cau loi cua bo kiem skill (ValidateName/ValidateDescription/Parse - tieng Anh vi con dung cho log
// va read_skill_file) dich sang cau cho nguoi dung. Cau chua biet: giu nguyen.
func vietnameseProblem(problem string) string {
	translations := map[string]string{
		"frontmatter 'name' is required":                                       "Thiếu tên kỹ năng.",
		"'name' may only contain lowercase letters, digits and '-'":            "Tên kỹ năng chỉ gồm chữ thường không dấu, số và dấu '-' (vd: bao-cao-quy).",
		"frontmatter 'description' is required":                                "Thiếu mô tả — AI dựa vào mô tả để biết khi nào dùng kỹ năng.",
		"'description' must not contain XML tags":                              "Mô tả không được chứa thẻ dạng <...>.",
		"SKILL.md must start with a YAML frontmatter block delimited by '---'": "SKILL.md phải bắt đầu bằng khối '---' (frontmatter) có name và description.",
	}
	if problem == "" {
		return ""
	}
	if message, ok := translations[problem]; ok {
		return message
	}
	switch {
	case strings.HasPrefix(problem, "'name' must be at most"):
		return "Tên kỹ năng dài quá " + strconv.Itoa(MaxNameLength) + " ký tự."
	case strings.HasPrefix(problem, "'description' must be at most"):
		return "Mô tả dài quá " + strconv.Itoa(MaxDescriptionLength) + " ký tự."
	case strings.HasPrefix(problem, "'name' must not contain the reserved word"):
		return "Tên kỹ năng không được chứa từ dành riêng (" + strings.Join(reserved, ", ") + ")."
	}
	return problem
}
