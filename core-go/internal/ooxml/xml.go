// Package ooxml: doc/ghi goi Office Open XML (docx/xlsx/pptx) thang tren archive/zip, khong
// dung thu vien ngoai. Port cua src/AxiomOffice.Host/Mcp/OoxmlPackage.cs.
//
// Chi nhung part bi sua moi duoc ghi lai, nen chart/anh/pivot/macro trong file giu nguyen.
package ooxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// XMLNamespace la namespace dung cho tien to "xml" co san trong moi tai lieu XML.
const XMLNamespace = "http://www.w3.org/XML/1998/namespace"

// Node la mot nut trong cay XML: *Element, CharData, Comment, ProcInst hoac Directive.
type Node interface{ isNode() }

// Element vua giu tien to goc (de ghi lai y nguyen) vua giu URI namespace da phan giai
// (de truy van giong XNamespace cua ban C#). Truy van luon theo URI + Local.
type Element struct {
	URI    string
	Prefix string
	Local  string
	Attrs  []Attr
	Kids   []Node
}

// Attr giu tien to goc va URI da phan giai. Thuoc tinh khong co tien to thuoc namespace rong,
// dung nhu XAttribute cua ban C#.
type Attr struct {
	URI    string
	Prefix string
	Local  string
	Value  string
}

type CharData struct{ Text string }

type Comment struct{ Text string }

type ProcInst struct{ Target, Inst string }

type Directive struct{ Text string }

func (*Element) isNode()  {}
func (CharData) isNode()  {}
func (Comment) isNode()   {}
func (ProcInst) isNode()  {}
func (Directive) isNode() {}

// Document la mot part XML da doc. Nodes gom ca phan truoc va sau phan tu goc.
type Document struct {
	Declaration string // noi dung trong <?xml ...?>; "" nghia la file goc khong co khai bao
	Nodes       []Node
}

func NewDocument(root *Element) *Document {
	return &Document{Nodes: []Node{root}}
}

// Root tra ve phan tu goc (phan tu dau tien o muc tai lieu).
func (d *Document) Root() *Element {
	for _, node := range d.Nodes {
		if element, ok := node.(*Element); ok {
			return element
		}
	}
	return nil
}

// ---- phan giai namespace ----

// nsScope la chuoi khai bao xmlns dang co hieu luc, dung trong luc parse.
type nsScope struct {
	parent *nsScope
	prefix string
	uri    string
}

func (s *nsScope) lookup(prefix string) (string, bool) {
	if prefix == "xml" {
		return XMLNamespace, true
	}
	for current := s; current != nil; current = current.parent {
		if current.prefix == prefix {
			return current.uri, true
		}
	}
	return "", false
}

// ParseXML doc mot part XML, giu nguyen tien to va phan giai URI namespace.
func ParseXML(data []byte) (*Document, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	document := &Document{}
	stack := []*Element{}
	scopes := []*nsScope{}
	for {
		token, err := decoder.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			parent := (*nsScope)(nil)
			if len(scopes) > 0 {
				parent = scopes[len(scopes)-1]
			}
			scope := declareScope(parent, typed.Attr)
			element := &Element{Prefix: typed.Name.Space, Local: typed.Name.Local}
			if uri, ok := scope.lookup(element.Prefix); ok {
				element.URI = uri
			}
			for _, attribute := range typed.Attr {
				if isNamespaceDeclaration(attribute.Name) {
					// Giu nguyen khai bao xmlns nhu mot thuoc tinh binh thuong.
					element.Attrs = append(element.Attrs, Attr{Prefix: attribute.Name.Space, Local: attribute.Name.Local, Value: attribute.Value})
					continue
				}
				resolved := ""
				if attribute.Name.Space != "" {
					if uri, ok := scope.lookup(attribute.Name.Space); ok {
						resolved = uri
					}
				}
				element.Attrs = append(element.Attrs, Attr{
					URI: resolved, Prefix: attribute.Name.Space, Local: attribute.Name.Local, Value: attribute.Value,
				})
			}
			if len(stack) == 0 {
				document.Nodes = append(document.Nodes, element)
			} else {
				stack[len(stack)-1].Kids = append(stack[len(stack)-1].Kids, element)
			}
			stack = append(stack, element)
			scopes = append(scopes, scope)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
				scopes = scopes[:len(scopes)-1]
			}
		case xml.CharData:
			// Ngoai phan tu goc chi duoc phep co comment/PI; bo qua khoang trang lai.
			if len(stack) == 0 {
				if strings.TrimSpace(string(typed)) != "" {
					return nil, fmt.Errorf("unexpected text outside the root element")
				}
				continue
			}
			parent := stack[len(stack)-1]
			parent.Kids = append(parent.Kids, CharData{Text: string(typed)})
		case xml.Comment:
			document.addNode(stack, Comment{Text: string(typed)})
		case xml.ProcInst:
			if typed.Target == "xml" && len(stack) == 0 {
				document.Declaration = string(typed.Inst)
				continue
			}
			document.addNode(stack, ProcInst{Target: typed.Target, Inst: string(typed.Inst)})
		case xml.Directive:
			document.addNode(stack, Directive{Text: string(typed)})
		}
	}
	if document.Root() == nil {
		return nil, fmt.Errorf("no root element")
	}
	return document, nil
}

func isNamespaceDeclaration(name xml.Name) bool {
	return name.Space == "xmlns" || (name.Space == "" && name.Local == "xmlns")
}

func declareScope(parent *nsScope, attributes []xml.Attr) *nsScope {
	scope := parent
	for _, attribute := range attributes {
		switch {
		case attribute.Name.Space == "xmlns":
			scope = &nsScope{parent: scope, prefix: attribute.Name.Local, uri: attribute.Value}
		case attribute.Name.Space == "" && attribute.Name.Local == "xmlns":
			scope = &nsScope{parent: scope, prefix: "", uri: attribute.Value}
		}
	}
	return scope
}

func (d *Document) addNode(stack []*Element, node Node) {
	if len(stack) == 0 {
		d.Nodes = append(d.Nodes, node)
		return
	}
	parent := stack[len(stack)-1]
	parent.Kids = append(parent.Kids, node)
}

// ---- ghi ra ----

// Serialize ghi document ra bytes: UTF-8 khong BOM, khong thut le, newline giu nguyen.
func (d *Document) Serialize() []byte {
	var builder strings.Builder
	declaration := d.Declaration
	if strings.TrimSpace(declaration) == "" {
		declaration = `version="1.0" encoding="utf-8"`
	}
	builder.WriteString("<?xml ")
	builder.WriteString(declaration)
	builder.WriteString("?>")
	for _, node := range d.Nodes {
		writeNode(&builder, node)
	}
	return []byte(builder.String())
}

func writeNode(builder *strings.Builder, node Node) {
	switch typed := node.(type) {
	case *Element:
		writeElement(builder, typed)
	case CharData:
		builder.WriteString(escapeText(typed.Text))
	case Comment:
		builder.WriteString("<!--")
		builder.WriteString(typed.Text)
		builder.WriteString("-->")
	case ProcInst:
		builder.WriteString("<?")
		builder.WriteString(typed.Target)
		if typed.Inst != "" {
			builder.WriteString(" ")
			builder.WriteString(typed.Inst)
		}
		builder.WriteString("?>")
	case Directive:
		builder.WriteString("<!")
		builder.WriteString(typed.Text)
		builder.WriteString(">")
	}
}

func writeElement(builder *strings.Builder, element *Element) {
	builder.WriteString("<")
	builder.WriteString(qualifiedName(element.Prefix, element.Local))
	for _, attribute := range element.Attrs {
		builder.WriteString(" ")
		builder.WriteString(qualifiedName(attribute.Prefix, attribute.Local))
		builder.WriteString(`="`)
		builder.WriteString(escapeAttr(attribute.Value))
		builder.WriteString(`"`)
	}
	if len(element.Kids) == 0 {
		builder.WriteString("/>")
		return
	}
	builder.WriteString(">")
	for _, kid := range element.Kids {
		writeNode(builder, kid)
	}
	builder.WriteString("</")
	builder.WriteString(qualifiedName(element.Prefix, element.Local))
	builder.WriteString(">")
}

func qualifiedName(prefix, local string) string {
	if prefix == "" {
		return local
	}
	return prefix + ":" + local
}

func escapeText(value string) string {
	var builder strings.Builder
	for _, character := range value {
		switch character {
		case '&':
			builder.WriteString("&amp;")
		case '<':
			builder.WriteString("&lt;")
		case '>':
			builder.WriteString("&gt;")
		case '\r':
			builder.WriteString("&#xD;")
		default:
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func escapeAttr(value string) string {
	var builder strings.Builder
	for _, character := range value {
		switch character {
		case '&':
			builder.WriteString("&amp;")
		case '<':
			builder.WriteString("&lt;")
		case '>':
			builder.WriteString("&gt;")
		case '"':
			builder.WriteString("&quot;")
		case '\r':
			builder.WriteString("&#xD;")
		case '\n':
			builder.WriteString("&#xA;")
		case '\t':
			builder.WriteString("&#x9;")
		default:
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

// ---- truy van cay (khop theo URI namespace + ten dia phuong, giong XNamespace cua C#) ----

// Child tra ve phan tu con truc tiep dau tien khop.
func (e *Element) Child(uri, local string) *Element {
	for _, kid := range e.Kids {
		if element, ok := kid.(*Element); ok && element.URI == uri && element.Local == local {
			return element
		}
	}
	return nil
}

// Children tra ve moi phan tu con truc tiep khop.
func (e *Element) Children(uri, local string) []*Element {
	found := []*Element{}
	for _, kid := range e.Kids {
		if element, ok := kid.(*Element); ok && element.URI == uri && element.Local == local {
			found = append(found, element)
		}
	}
	return found
}

// Elements tra ve moi phan tu con truc tiep.
func (e *Element) Elements() []*Element {
	found := []*Element{}
	for _, kid := range e.Kids {
		if element, ok := kid.(*Element); ok {
			found = append(found, element)
		}
	}
	return found
}

// ChildLocal khop chi theo ten cuc bo, khong quan tam namespace (giong helper Child cua ban C#
// trong XlsxBook). Dung cho cac part ma tien to co the doi giua cac thu vien.
func (e *Element) ChildLocal(local string) *Element {
	for _, kid := range e.Kids {
		if element, ok := kid.(*Element); ok && element.Local == local {
			return element
		}
	}
	return nil
}

// ChildrenLocal khop chi theo ten cuc bo.
func (e *Element) ChildrenLocal(local string) []*Element {
	found := []*Element{}
	for _, kid := range e.Kids {
		if element, ok := kid.(*Element); ok && element.Local == local {
			found = append(found, element)
		}
	}
	return found
}

// ElementAt tra ve phan tu con thu index (bo qua cac nut khong phai phan tu).
func (e *Element) ElementAt(index int) *Element {
	if index < 0 {
		return nil
	}
	elements := e.Elements()
	if index >= len(elements) {
		return nil
	}
	return elements[index]
}

// RemoveChildrenLocal bo moi phan tu con co ten cuc bo cho truoc.
func (e *Element) RemoveChildrenLocal(local string) {
	kept := e.Kids[:0]
	for _, kid := range e.Kids {
		if element, ok := kid.(*Element); ok && element.Local == local {
			continue
		}
		kept = append(kept, kid)
	}
	e.Kids = kept
}

// NewRoot tao phan tu goc co namespace mac dinh (tu them khai bao xmlns, vi ban Go khong tu sinh
// khai bao theo namespace nhu XElement cua C#).
func NewRoot(uri, local string) *Element {
	root := NewElement(uri, "", local)
	root.DeclareNamespace("", uri)
	return root
}

// HasPrefix cho biet phan tu da khai bao xmlns:<prefix> chua.
func (e *Element) HasPrefix(prefix string) bool {
	for _, attribute := range e.Attrs {
		if attribute.Prefix == "xmlns" && attribute.Local == prefix {
			return true
		}
	}
	return false
}

// DeclareNamespace khai bao xmlns tren phan tu; prefix "" la namespace mac dinh.
func (e *Element) DeclareNamespace(prefix, uri string) {
	for index := range e.Attrs {
		attribute := &e.Attrs[index]
		if prefix == "" {
			if attribute.Prefix == "" && attribute.Local == "xmlns" {
				attribute.Value = uri
				return
			}
			continue
		}
		if attribute.Prefix == "xmlns" && attribute.Local == prefix {
			attribute.Value = uri
			return
		}
	}
	if prefix == "" {
		e.Attrs = append(e.Attrs, Attr{Prefix: "", Local: "xmlns", Value: uri})
		return
	}
	e.Attrs = append(e.Attrs, Attr{Prefix: "xmlns", Local: prefix, Value: uri})
}

// AttrValue khop theo URI + ten; thuoc tinh khong tien to thi uri la "".
func (e *Element) AttrValue(uri, local string) string {
	for _, attribute := range e.Attrs {
		if attribute.URI == uri && attribute.Local == local {
			return attribute.Value
		}
	}
	return ""
}

func (e *Element) HasAttr(uri, local string) bool {
	for _, attribute := range e.Attrs {
		if attribute.URI == uri && attribute.Local == local {
			return true
		}
	}
	return false
}

// SetAttr thay gia tri tai cho (giu nguyen vi tri) hoac them vao cuoi.
func (e *Element) SetAttr(uri, prefix, local, value string) {
	for index := range e.Attrs {
		if e.Attrs[index].URI == uri && e.Attrs[index].Local == local {
			e.Attrs[index].Value = value
			return
		}
	}
	e.Attrs = append(e.Attrs, Attr{URI: uri, Prefix: prefix, Local: local, Value: value})
}

func (e *Element) RemoveAttr(uri, local string) {
	kept := e.Attrs[:0]
	for _, attribute := range e.Attrs {
		if attribute.URI == uri && attribute.Local == local {
			continue
		}
		kept = append(kept, attribute)
	}
	e.Attrs = kept
}

// Text gop moi CharData trong ca cay con (giong XElement.Value cua ban C#).
func (e *Element) Text() string {
	var builder strings.Builder
	collectText(&builder, e)
	return builder.String()
}

func collectText(builder *strings.Builder, node Node) {
	switch typed := node.(type) {
	case *Element:
		for _, kid := range typed.Kids {
			collectText(builder, kid)
		}
	case CharData:
		builder.WriteString(typed.Text)
	}
}

// AddText them mot doan text (duoc escape khi ghi ra).
func (e *Element) AddText(value string) {
	e.Kids = append(e.Kids, CharData{Text: value})
}

// SetText thay toan bo noi dung con bang mot doan text duy nhat.
func (e *Element) SetText(value string) {
	e.Kids = nil
	if value != "" {
		e.AddText(value)
	}
}

func (e *Element) Append(child Node) {
	e.Kids = append(e.Kids, child)
}

func (e *Element) InsertAt(index int, child Node) {
	if index < 0 {
		index = 0
	}
	if index > len(e.Kids) {
		index = len(e.Kids)
	}
	e.Kids = append(e.Kids, nil)
	copy(e.Kids[index+1:], e.Kids[index:])
	e.Kids[index] = child
}

func (e *Element) ChildIndex(child Node) int {
	for index, kid := range e.Kids {
		if kid == child {
			return index
		}
	}
	return -1
}

func (e *Element) RemoveChild(child Node) bool {
	index := e.ChildIndex(child)
	if index < 0 {
		return false
	}
	e.Kids = append(e.Kids[:index], e.Kids[index+1:]...)
	return true
}

// NewElement tao phan tu moi chua thuoc cay nao.
func NewElement(uri, prefix, local string) *Element {
	return &Element{URI: uri, Prefix: prefix, Local: local}
}

// AppendElement tao va them mot phan tu con.
func (e *Element) AppendElement(uri, prefix, local string) *Element {
	child := NewElement(uri, prefix, local)
	e.Append(child)
	return child
}

// Clone sao chep sau ca cay (giong XElement cua ban C#).
func (e *Element) Clone() *Element {
	copied := &Element{URI: e.URI, Prefix: e.Prefix, Local: e.Local}
	copied.Attrs = append(copied.Attrs, e.Attrs...)
	for _, kid := range e.Kids {
		copied.Kids = append(copied.Kids, cloneNode(kid))
	}
	return copied
}

func cloneNode(node Node) Node {
	if element, ok := node.(*Element); ok {
		return element.Clone()
	}
	return node
}
