package tools

import (
	"strings"
)

// So doi tuong cua luot (docs/plans/19-h0 H4): slide va shape do CHINH luot nay tao ra. Xoa nhung thu do thi khong can
// hoi nguoi dung - agent dang sua lai cai minh vua dung. Benchmark 25/10/2026: luot khong co nguoi bam (ai.ask, MCP)
// bi tu choi moi lenh xoa, nen agent nop bai kem slide trong do chinh no them thua; luot co nguoi thi nguoi dung bi
// hoi ve nhung thu ho chua tung thay. Doi tuong co san trong tai lieu van theo quy tac xac nhan cu.
type ownedObjects struct {
	// slides[bai][so slide]; shapes[bai][so slide][ten shape]. bai = tham so presentation (chu thuong), "" = bai active.
	slides map[string]map[int]bool
	shapes map[string]map[int]map[string]bool
}

func (run *RunContext) ownedObjects() *ownedObjects {
	if run.owned == nil {
		run.owned = &ownedObjects{slides: map[string]map[int]bool{}, shapes: map[string]map[int]map[string]bool{}}
	}
	return run.owned
}

func ownedDeck(params map[string]any) string {
	deck, _ := params["presentation"].(string)
	return strings.ToLower(strings.TrimSpace(deck))
}

// recordOwned: ghi lai sau mot lenh thanh cong.
func (o *ownedObjects) record(action string, params, result map[string]any) {
	deck := ownedDeck(params)
	slide := asInt(result["slide"])
	switch {
	case action == "wpp.addSlide":
		if slide > 0 {
			if o.slides[deck] == nil {
				o.slides[deck] = map[int]bool{}
			}
			o.slides[deck][slide] = true
		}
	case action == "wpp.deleteSlide":
		o.removeSlide(deck, asInt(result["deleted"]))
	case action == "wpp.deleteShapes":
		for _, name := range stringList(params["shapes"]) {
			delete(o.shapes[deck][asInt(params["slide"])], name)
		}
	case strings.HasPrefix(action, "wpp.add") || action == "wpp.copyPicture":
		if shape, _ := result["shape"].(string); slide > 0 && shape != "" {
			if o.shapes[deck] == nil {
				o.shapes[deck] = map[int]map[string]bool{}
			}
			if o.shapes[deck][slide] == nil {
				o.shapes[deck][slide] = map[string]bool{}
			}
			o.shapes[deck][slide][shape] = true
		}
	}
}

// removeSlide: slide k bi xoa - slide sau no lui len mot so.
func (o *ownedObjects) removeSlide(deck string, deleted int) {
	if deleted <= 0 {
		return
	}
	slides := map[int]bool{}
	for index := range o.slides[deck] {
		switch {
		case index < deleted:
			slides[index] = true
		case index > deleted:
			slides[index-1] = true
		}
	}
	o.slides[deck] = slides
	shapes := map[int]map[string]bool{}
	for index, names := range o.shapes[deck] {
		switch {
		case index < deleted:
			shapes[index] = names
		case index > deleted:
			shapes[index-1] = names
		}
	}
	o.shapes[deck] = shapes
}

// covers: lenh xoa nay chi dung toi doi tuong cua luot. Thieu so slide (lenh tu chon slide cuoi) thi khong chac -> false.
func (o *ownedObjects) covers(action string, params map[string]any) bool {
	deck := ownedDeck(params)
	slide := asInt(params["slide"])
	if slide <= 0 {
		return false
	}
	switch action {
	case "wpp.deleteSlide":
		return o.slides[deck][slide]
	case "wpp.deleteShapes":
		if o.slides[deck][slide] {
			return true // moi thu tren slide do luot tao deu la cua luot
		}
		names := stringList(params["shapes"])
		if len(names) == 0 || params["emptyPlaceholders"] == true {
			return false
		}
		for _, name := range names {
			if !o.shapes[deck][slide][name] {
				return false
			}
		}
		return true
	}
	return false
}
