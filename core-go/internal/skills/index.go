// Package skills: chi muc skill theo chuan Agent Skills (New_arch.md muc 8.4.4) - ban Go cua
// Skills/SkillIndex.cs, SkillLoader.cs, Tools/SkillTools.cs (phan file).
package skills

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"axiomoffice/core/internal/corelog"
)

// WatchInterval: .NET dung FileSystemWatcher + debounce 2s; Go khong co watcher trong stdlib nen quet lai
// theo nhip va chi Reload khi danh sach file thay doi (thu muc skill nho nen re).
const WatchInterval = 2 * time.Second

// Source: mot nguon skill (ten hien thi + thu muc).
type Source struct {
	Name string
	Dir  string
}

// Definition: mot skill hop le (thu muc chua SKILL.md co frontmatter name + description).
type Definition struct {
	Name        string
	Description string
	Apps        []string
	Directory   string
	Source      string
	Body        string
}

func (d Definition) AppliesTo(appKind string) bool {
	if len(d.Apps) == 0 {
		return true
	}
	for _, app := range d.Apps {
		if strings.EqualFold(app, appKind) {
			return true
		}
	}
	return false
}

// Failure: skill loi (frontmatter sai...): bo qua skill do, hien o GET /v1/skills, khong lam hong Core.
type Failure struct {
	Directory string
	Source    string
	Error     string
}

type snapshot struct {
	skills    map[string]Definition
	resources map[string]string
	errs      []Failure
}

// Index: quet cac nguon theo thu tu uu tien, nguon sau ghi de nguon truoc khi trung ten.
// Thu muc bat dau bang '_' (vd _design) la goi tai nguyen dung chung, khong phai skill.
type Index struct {
	sources []Source
	mu      sync.RWMutex
	current snapshot
}

func New(sources []Source) *Index {
	index := &Index{sources: sources, current: snapshot{skills: map[string]Definition{}, resources: map[string]string{}}}
	index.Reload()
	return index
}

// DefaultSources: builtin (skills/ canh binary) -> thu muc cua to chuc -> thu muc cua nguoi dung.
func DefaultSources(builtin string, organization []string, user string) []Source {
	sources := []Source{{Name: "builtin", Dir: builtin}}
	for _, dir := range organization {
		if strings.TrimSpace(dir) != "" {
			sources = append(sources, Source{Name: "org", Dir: strings.TrimSpace(dir)})
		}
	}
	return append(sources, Source{Name: "user", Dir: user})
}

func (i *Index) Sources() []Source { return i.sources }

func (i *Index) All() []Definition {
	i.mu.RLock()
	defer i.mu.RUnlock()
	list := make([]Definition, 0, len(i.current.skills))
	for _, skill := range i.current.skills {
		list = append(list, skill)
	}
	sort.Slice(list, func(a, b int) bool { return list[a].Name < list[b].Name })
	return list
}

func (i *Index) Errors() []Failure {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return append([]Failure{}, i.current.errs...)
}

func (i *Index) ForApp(appKind string) []Definition {
	list := []Definition{}
	for _, skill := range i.All() {
		if skill.AppliesTo(appKind) {
			list = append(list, skill)
		}
	}
	return list
}

func (i *Index) Find(name string) *Definition {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if skill, ok := i.current.skills[strings.TrimSpace(name)]; ok {
		return &skill
	}
	return nil
}

// ResourceDirectory: thu muc goi tai nguyen (vd "_design"), theo cung thu tu uu tien voi skill.
func (i *Index) ResourceDirectory(name string) string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.current.resources[strings.TrimSpace(name)]
}

func (i *Index) Reload() {
	skills := map[string]Definition{}
	resources := map[string]string{}
	errs := []Failure{}
	for _, source := range i.sources {
		entries, err := os.ReadDir(source.Dir)
		if err != nil {
			continue // nguon chua ton tai: bo qua, giong ban .NET
		}
		directories := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				directories = append(directories, entry.Name())
			}
		}
		sort.Slice(directories, func(a, b int) bool {
			return strings.ToLower(directories[a]) < strings.ToLower(directories[b])
		})

		for _, folder := range directories {
			directory := filepath.Join(source.Dir, folder)
			if strings.HasPrefix(folder, "_") {
				resources[folder] = directory
				continue
			}
			if _, err := os.Stat(filepath.Join(directory, SkillFileName)); err != nil {
				continue
			}
			skill, failure := Load(directory, source.Name)
			switch {
			case skill != nil:
				skills[skill.Name] = *skill
			case failure != nil:
				errs = append(errs, *failure)
			}
		}
	}

	i.mu.Lock()
	i.current = snapshot{skills: skills, resources: resources, errs: errs}
	i.mu.Unlock()

	message := "skills loaded: " + strconv.Itoa(len(skills)) + " ok, " + strconv.Itoa(len(errs)) + " error(s)"
	if len(errs) > 0 {
		parts := make([]string, 0, len(errs))
		for _, failure := range errs {
			parts = append(parts, filepath.Base(failure.Directory)+": "+failure.Error)
		}
		message += " - " + strings.Join(parts, "; ")
	}
	corelog.Info("%s", message)
}

// Watch quet lai khi thu muc doi (chi Reload khi co thay doi thuc su). Dung khi ctx ket thuc.
func (i *Index) Watch(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(WatchInterval)
		defer ticker.Stop()
		previous := i.signature()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if current := i.signature(); current != previous {
					previous = current
					i.Reload()
					previous = i.signature()
				}
			}
		}
	}()
}

// signature: ten + mtime + co file cua tung muc trong cac nguon (du nhan ra skill them/sua/xoa).
func (i *Index) signature() string {
	var builder strings.Builder
	for _, source := range i.sources {
		entries, err := os.ReadDir(source.Dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			builder.WriteString(source.Dir)
			builder.WriteString("|")
			builder.WriteString(entry.Name())
			builder.WriteString("|")
			if info, err := entry.Info(); err == nil {
				builder.WriteString(info.ModTime().UTC().Format(time.RFC3339Nano))
				builder.WriteString("|")
				builder.WriteString(strconv.FormatInt(info.Size(), 10))
			}
			builder.WriteString("\n")
			if !entry.IsDir() {
				continue
			}
			// File ben trong skill: them/sua reference hoac SKILL.md.
			inner, err := os.ReadDir(filepath.Join(source.Dir, entry.Name()))
			if err != nil {
				continue
			}
			for _, file := range inner {
				builder.WriteString(entry.Name())
				builder.WriteString("/")
				builder.WriteString(file.Name())
				builder.WriteString("|")
				if info, err := file.Info(); err == nil {
					builder.WriteString(info.ModTime().UTC().Format(time.RFC3339Nano))
					builder.WriteString("|")
					builder.WriteString(strconv.FormatInt(info.Size(), 10))
				}
				builder.WriteString("\n")
			}
		}
	}
	return builder.String()
}
