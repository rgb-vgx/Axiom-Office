package ooxml

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"axiomoffice/core/internal/filesafe"
)

// Namespace dung trong goi OOXML.
const (
	RelNamespace     = "http://schemas.openxmlformats.org/package/2006/relationships"
	ContentTypeNS    = "http://schemas.openxmlformats.org/package/2006/content-types"
	RelTypeBase      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
	ContentTypesPart = "[Content_Types].xml"
	RootRelsPart     = "_rels/.rels"
)

// Namespace cua tung dinh dang con. Chu y: OfficeRelNamespace KHONG co "/" cuoi, con RelTypeBase
// thi co - chung la hai chuoi khac nhau (namespace cua tien to r: so voi tien to cua loai quan he).
const (
	OfficeRelNamespace = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	SpreadsheetNS      = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	WordprocessingNS   = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	PresentationNS     = "http://schemas.openxmlformats.org/presentationml/2006/main"
	DrawingNS          = "http://schemas.openxmlformats.org/drawingml/2006/main"
)

// Rel la mot quan he trong part .rels; TargetPart la ten part tuyet doi (nil khi External).
type Rel struct {
	ID         string
	Type       string
	Target     string
	TargetPart string
	External   bool
}

// Package la mot goi OOXML dang mo. Ten part dang "xl/workbook.xml" (khong co "/" dau).
type Package struct {
	order []string          // khoa (chu thuong) theo thu tu xuat hien trong zip
	names map[string]string // khoa -> ten part nguyen ban
	raw   map[string][]byte // part chua doc
	docs  map[string]*Document
	dirty map[string]bool
}

// ---- mo goi ----

// OpenRead mo goi OOXML de doc.
func OpenRead(path string) (*Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pkg, err := OpenBytes(data)
	if err != nil {
		return nil, fmt.Errorf("'%s' is not a valid Office Open XML file (zip)", path)
	}
	return pkg, nil
}

// OpenBytes mo goi tu bytes (dung cho template nhung san).
func OpenBytes(data []byte) (*Package, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	pkg := &Package{names: map[string]string{}, raw: map[string][]byte{}, docs: map[string]*Document{}, dirty: map[string]bool{}}
	for _, file := range reader.File {
		name := Normalize(file.Name)
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		content, err := readEntry(file)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(name)
		if _, exists := pkg.names[key]; !exists {
			pkg.order = append(pkg.order, key)
			pkg.names[key] = name
		}
		pkg.raw[key] = content
	}
	return pkg, nil
}

func readEntry(file *zip.File) ([]byte, error) {
	handle, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	return io.ReadAll(handle)
}

// NewPackage tao mot goi trong chua co part nao, de dung dan cac part roi Archive.
func NewPackage() *Package {
	return &Package{names: map[string]string{}, raw: map[string][]byte{}, docs: map[string]*Document{}, dirty: map[string]bool{}}
}

// Normalize chuan hoa ten part: bo "/" dau, doi "\" thanh "/".
func Normalize(part string) string {
	return strings.TrimLeft(strings.ReplaceAll(part, "\\", "/"), "/")
}

func key(part string) string { return strings.ToLower(Normalize(part)) }

// ---- doc part ----

func (p *Package) PartNames() []string {
	names := make([]string, 0, len(p.order))
	for _, item := range p.order {
		names = append(names, p.names[item])
	}
	return names
}

func (p *Package) Exists(part string) bool {
	item := key(part)
	if _, ok := p.docs[item]; ok {
		return true
	}
	_, ok := p.raw[item]
	return ok
}

// XML doc mot part XML (cache lai). Loi khi part khong co trong goi.
func (p *Package) XML(part string) (*Document, error) {
	item := key(part)
	if doc, exists := p.docs[item]; exists {
		return doc, nil
	}
	data, exists := p.raw[item]
	if !exists {
		return nil, fmt.Errorf("part not found in package: %s", Normalize(part))
	}
	doc, err := ParseXML(data)
	if err != nil {
		return nil, err
	}
	p.docs[item] = doc
	return doc, nil
}

// XMLOrNil tra ve nil khi part khong ton tai (giong XmlOrNull cua ban C#).
func (p *Package) XMLOrNil(part string) *Document {
	if !p.Exists(part) {
		return nil
	}
	doc, err := p.XML(part)
	if err != nil {
		return nil
	}
	return doc
}

// Bytes doc tho mot part.
func (p *Package) Bytes(part string) ([]byte, error) {
	data, exists := p.raw[key(part)]
	if !exists {
		if doc, hasDoc := p.docs[key(part)]; hasDoc {
			return doc.Serialize(), nil
		}
		return nil, fmt.Errorf("part not found in package: %s", Normalize(part))
	}
	return data, nil
}

// Put danh dau part XML da sua (hoac tao moi) de ghi lai khi luu.
func (p *Package) Put(part string, doc *Document) {
	item := key(part)
	if _, exists := p.names[item]; !exists {
		p.order = append(p.order, item)
		p.names[item] = Normalize(part)
	}
	p.docs[item] = doc
	p.dirty[item] = true
}

// Touch danh dau part XML da doc la da sua.
func (p *Package) Touch(part string) error {
	doc, err := p.XML(part)
	if err != nil {
		return err
	}
	p.Put(part, doc)
	return nil
}

// Delete xoa mot part kem part .rels cua no va content-type override.
func (p *Package) Delete(part string) {
	item := key(part)
	delete(p.docs, item)
	delete(p.dirty, item)
	delete(p.raw, item)
	if _, named := p.names[item]; named {
		delete(p.names, item)
		for index, existing := range p.order {
			if existing == item {
				p.order = append(p.order[:index], p.order[index+1:]...)
				break
			}
		}
	}
	rels := RelsPartFor(part)
	if !strings.EqualFold(rels, Normalize(part)) && p.Exists(rels) {
		p.Delete(rels)
	}
	p.RemoveContentTypeOverride(part)
}

// ---- relationships ----

// RelsPartFor tra ve ten part .rels cua mot part ("xl/workbook.xml" -> "xl/_rels/workbook.xml.rels").
func RelsPartFor(part string) string {
	part = Normalize(part)
	slash := strings.LastIndex(part, "/")
	dir, name := "", part
	if slash >= 0 {
		dir, name = part[:slash+1], part[slash+1:]
	}
	return dir + "_rels/" + name + ".rels"
}

// Rels doc quan he cua mot part (sourcePart "" = goi goc: _rels/.rels).
func (p *Package) Rels(sourcePart string) []Rel {
	relsPart := RootRelsPart
	if sourcePart != "" {
		relsPart = RelsPartFor(sourcePart)
	}
	doc := p.XMLOrNil(relsPart)
	result := []Rel{}
	if doc == nil || doc.Root() == nil {
		return result
	}
	for _, rel := range doc.Root().Children(RelNamespace, "Relationship") {
		external := strings.EqualFold(rel.AttrValue("", "TargetMode"), "External")
		target := rel.AttrValue("", "Target")
		entry := Rel{
			ID:       rel.AttrValue("", "Id"),
			Type:     rel.AttrValue("", "Type"),
			Target:   target,
			External: external,
		}
		if !external {
			entry.TargetPart = ResolveTarget(sourcePart, target)
		}
		result = append(result, entry)
	}
	return result
}

// RelOfType tim quan he dau tien co Type ket thuc bang "/"+suffix.
func (p *Package) RelOfType(sourcePart, suffix string) *Rel {
	for _, rel := range p.Rels(sourcePart) {
		if strings.HasSuffix(rel.Type, "/"+suffix) {
			found := rel
			return &found
		}
	}
	return nil
}

// AddRel them quan he moi, tra ve Id vua cap.
func (p *Package) AddRel(sourcePart, relType, targetPart string) string {
	relsPart := RootRelsPart
	if sourcePart != "" {
		relsPart = RelsPartFor(sourcePart)
	}
	doc := p.XMLOrNil(relsPart)
	if doc == nil || doc.Root() == nil {
		root := NewElement(RelNamespace, "", "Relationships")
		root.Attrs = append(root.Attrs, Attr{Prefix: "", Local: "xmlns", Value: RelNamespace})
		doc = NewDocument(root)
	}
	used := map[string]bool{}
	for _, rel := range doc.Root().Children(RelNamespace, "Relationship") {
		used[rel.AttrValue("", "Id")] = true
	}
	number := len(used) + 1
	for used["rId"+strconv.Itoa(number)] {
		number++
	}
	id := "rId" + strconv.Itoa(number)
	fullType := relType
	if !strings.HasPrefix(relType, "http") {
		fullType = RelTypeBase + relType
	}
	entry := doc.Root().AppendElement(RelNamespace, "", "Relationship")
	entry.SetAttr("", "", "Id", id)
	entry.SetAttr("", "", "Type", fullType)
	entry.SetAttr("", "", "Target", RelativeTarget(sourcePart, targetPart))
	p.Put(relsPart, doc)
	return id
}

// RemoveRel xoa quan he theo Id.
func (p *Package) RemoveRel(sourcePart, id string) {
	relsPart := RootRelsPart
	if sourcePart != "" {
		relsPart = RelsPartFor(sourcePart)
	}
	doc := p.XMLOrNil(relsPart)
	if doc == nil || doc.Root() == nil {
		return
	}
	for _, rel := range doc.Root().Children(RelNamespace, "Relationship") {
		if rel.AttrValue("", "Id") == id {
			doc.Root().RemoveChild(rel)
		}
	}
	p.Put(relsPart, doc)
}

// ResolveTarget quy Target (tuong doi) ve ten part tuyet doi trong goi.
func ResolveTarget(sourcePart, target string) string {
	if strings.HasPrefix(target, "/") {
		return Normalize(target)
	}
	normalized := Normalize(sourcePart)
	baseDir := ""
	if slash := strings.LastIndex(normalized, "/"); slash >= 0 {
		baseDir = normalized[:slash]
	}
	segments := []string{}
	if baseDir != "" {
		segments = strings.Split(baseDir, "/")
	}
	for _, segment := range strings.Split(strings.ReplaceAll(target, "\\", "/"), "/") {
		switch {
		case segment == "..":
			if len(segments) > 0 {
				segments = segments[:len(segments)-1]
			}
		case segment != "" && segment != ".":
			segments = append(segments, segment)
		}
	}
	return strings.Join(segments, "/")
}

// RelativeTarget tinh Target tuong doi tu sourcePart toi targetPart.
func RelativeTarget(sourcePart, targetPart string) string {
	from := []string{}
	if sourcePart != "" {
		from = strings.Split(Normalize(sourcePart), "/")
	}
	to := strings.Split(Normalize(targetPart), "/")
	common := 0
	for common < len(from)-1 && common < len(to)-1 && strings.EqualFold(from[common], to[common]) {
		common++
	}
	parts := []string{}
	for index := common; index < len(from)-1; index++ {
		parts = append(parts, "..")
	}
	parts = append(parts, to[common:]...)
	return strings.Join(parts, "/")
}

// ---- content types ----

func (p *Package) SetContentTypeOverride(part, contentType string) error {
	doc, err := p.XML(ContentTypesPart)
	if err != nil {
		return err
	}
	name := "/" + Normalize(part)
	for _, override := range doc.Root().Children(ContentTypeNS, "Override") {
		if strings.EqualFold(override.AttrValue("", "PartName"), name) {
			override.SetAttr("", "", "ContentType", contentType)
			p.Put(ContentTypesPart, doc)
			return nil
		}
	}
	override := doc.Root().AppendElement(ContentTypeNS, "", "Override")
	override.SetAttr("", "", "PartName", name)
	override.SetAttr("", "", "ContentType", contentType)
	p.Put(ContentTypesPart, doc)
	return nil
}

func (p *Package) RemoveContentTypeOverride(part string) {
	if !p.Exists(ContentTypesPart) {
		return
	}
	doc := p.XMLOrNil(ContentTypesPart)
	if doc == nil || doc.Root() == nil {
		return
	}
	name := "/" + Normalize(part)
	removed := false
	for _, override := range doc.Root().Children(ContentTypeNS, "Override") {
		if strings.EqualFold(override.AttrValue("", "PartName"), name) {
			doc.Root().RemoveChild(override)
			removed = true
		}
	}
	if removed {
		p.Put(ContentTypesPart, doc)
	}
}

// ContentTypeOf tra ve content type cua part, uu tien Override roi den Default theo duoi file.
func (p *Package) ContentTypeOf(part string) string {
	doc := p.XMLOrNil(ContentTypesPart)
	if doc == nil || doc.Root() == nil {
		return ""
	}
	name := "/" + Normalize(part)
	for _, override := range doc.Root().Children(ContentTypeNS, "Override") {
		if strings.EqualFold(override.AttrValue("", "PartName"), name) {
			return override.AttrValue("", "ContentType")
		}
	}
	extension := strings.TrimPrefix(filepath.Ext(Normalize(part)), ".")
	for _, def := range doc.Root().Children(ContentTypeNS, "Default") {
		if strings.EqualFold(def.AttrValue("", "Extension"), extension) {
			return def.AttrValue("", "ContentType")
		}
	}
	return ""
}

// NextPartName tim ten part chua dung theo mau kieu "xl/worksheets/sheet%d.xml".
func (p *Package) NextPartName(pattern string) string {
	for number := 1; ; number++ {
		candidate := fmt.Sprintf(pattern, number)
		if !p.Exists(candidate) {
			return candidate
		}
	}
}

// ---- luu ----

// Archive dong goi lai thanh file zip. Chi part bi sua moi duoc ghi lai tu cay XML; phan con lai
// giu nguyen byte goc nen chart/anh/pivot/macro khong bi mat.
func (p *Package) Archive() ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, item := range p.order {
		name := p.names[item]
		var content []byte
		if p.dirty[item] {
			doc := p.docs[item]
			if doc == nil {
				continue
			}
			content = doc.Serialize()
		} else if data, exists := p.raw[item]; exists {
			content = data
		} else if doc, exists := p.docs[item]; exists {
			content = doc.Serialize()
		} else {
			continue
		}
		entry, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(content); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// ---- luu atomic ----

// Edit sua file tai cho: ghi ra file tam roi thay the, de file goc khong bi hong neu loi giua chung.
func Edit(path string, edit func(*Package) error) error {
	if !filesafe.Exists(path) {
		return fmt.Errorf("file not found: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	pkg, err := OpenBytes(data)
	if err != nil {
		return fmt.Errorf("'%s' is not a valid Office Open XML file (zip)", path)
	}
	if err := edit(pkg); err != nil {
		return err
	}
	output, err := pkg.Archive()
	if err != nil {
		return err
	}
	return filesafe.WriteAtomic(path, output)
}

// CreateFromTemplate tao file moi tu template nhung san.
func CreateFromTemplate(path string, template []byte, edit func(*Package) error) error {
	pkg, err := OpenBytes(template)
	if err != nil {
		return err
	}
	if err := edit(pkg); err != nil {
		return err
	}
	output, err := pkg.Archive()
	if err != nil {
		return err
	}
	return filesafe.WriteAtomic(path, output)
}

// EmptyZip tra ve mot file zip rong.
func EmptyZip() []byte {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	_ = writer.Close()
	return buffer.Bytes()
}
