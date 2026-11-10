package slidelayout

import "testing"

// So do slide 4 cua luot that 25/10/2026: Customer -> Gateway -> Orchestrator -> [Policy, Retrieval, Banking APIs,
// Response Check], co canh nhay qua nut cung hang (P -> B, R -> V) va canh quay ve tang dau (V -> C). Truoc day hai canh
// nhay xuyen qua hop o giua, canh quay ve chong len duong khac va cat qua Gateway.
func TestRouteDiagramArchitectureSlide(t *testing.T) {
	rows := Diagram(Rect{40, 130, 880, 370}, []int{1, 1, 1, 4})
	c, g, o := [2]int{0, 0}, [2]int{1, 0}, [2]int{2, 0}
	p, r, b, v := [2]int{3, 0}, [2]int{3, 1}, [2]int{3, 2}, [2]int{3, 3}
	pairs := [][2][2]int{{c, g}, {g, o}, {o, p}, {p, r}, {p, b}, {r, v}, {b, v}, {v, c}}
	edges := []Edge{}
	for _, pair := range pairs {
		edges = append(edges, Edge{pair[0][0], pair[0][1], pair[1][0], pair[1][1]})
	}
	routes := RouteDiagram(rows, edges)
	for e, route := range routes {
		if !route.Clear {
			t.Fatalf("canh %d (%v) khong co duong sach: %+v", e, pairs[e], route)
		}
		for l, row := range rows {
			for n, box := range row {
				at := [2]int{l, n}
				if at != pairs[e][0] && at != pairs[e][1] && PathCrosses(route.Path, box) {
					t.Fatalf("canh %d cat qua hop %v: %v", e, at, route.Path)
				}
			}
		}
	}
	// Nhay qua nut cung hang: vong duoi hang, hai duong vong lui sau khac nhau (khong trung nhau).
	if routes[4].FromSide != SideBottom || routes[4].ToSide != SideBottom || routes[5].FromSide != SideBottom {
		t.Fatalf("P->B, R->V phai vong duoi hang: %+v %+v", routes[4], routes[5])
	}
	if routes[4].Offset == routes[5].Offset || PathsOverlap(routes[4].Path, routes[5].Path) {
		t.Fatalf("hai duong vong duoi cung hang phai lech nhau: %v / %v", routes[4].Path, routes[5].Path)
	}
	// Quay ve tang dau: vong ben phai (V la nut phai cung), ra ngoai hop xa nhat.
	if routes[7].FromSide != SideRight || routes[7].ToSide != SideRight {
		t.Fatalf("V->C phai vong ben phai: %+v", routes[7])
	}
	if x := routes[7].Path[1][0]; x <= rows[3][3][0]+rows[3][3][2] {
		t.Fatalf("duong vong phai ra ngoai hop phai cung, x = %v", x)
	}
	// Tang ke nhau: day cha -> dinh con; cung hang ke nhau: thang ngang.
	if routes[0].FromSide != SideBottom || routes[0].ToSide != SideTop || routes[3].Elbow || routes[3].FromSide != SideRight {
		t.Fatalf("C->G / P->R: %+v %+v", routes[0], routes[3])
	}
}

func TestConnectorPathLoopAndCrossing(t *testing.T) {
	a, b := Rect{0, 0, 100, 50}, Rect{300, 0, 100, 50}
	loop := ConnectorPath(a, b, SideBottom, SideBottom, true, 20)
	want := []Point{{50, 50}, {50, 70}, {350, 70}, {350, 50}}
	for i := range want {
		if loop[i] != want[i] {
			t.Fatalf("vong duoi = %v", loop)
		}
	}
	straight := ConnectorPath(a, b, SideRight, SideLeft, false, 0)
	if !PathCrosses(straight, Rect{150, 0, 100, 50}) || PathCrosses(loop, Rect{150, 0, 100, 50}) {
		t.Fatal("duong thang phai cat hop o giua, duong vong thi khong")
	}
	// Sat mep (chay doc theo vien) khong tinh la cat.
	if PathCrosses([]Point{{0, 0}, {100, 0}}, Rect{0, 0, 100, 50}) {
		t.Fatal("di theo vien khong phai cat qua")
	}
	if !PathsOverlap([]Point{{0, 10}, {100, 10}}, []Point{{50, 11}, {200, 11}}) || PathsOverlap([]Point{{0, 10}, {100, 10}}, []Point{{0, 30}, {100, 30}}) {
		t.Fatal("chong / khong chong")
	}
}

// Khi PowerPoint ve lech mo hinh (checkLayout bao connector-*): ung vien ke tiep phai khac hinh hoc vua hong, va lan
// thu lai khong quay ve ung vien da thu (luot that 25/10/2026: 1 crossing + 3 overlap o lan dung dau slide 4).
func TestDiagramAlternativesSkipCurrentAndTried(t *testing.T) {
	rows := Diagram(Rect{40, 130, 880, 370}, []int{1, 1})
	edges := []Edge{{0, 0, 1, 0}}
	routes := RouteDiagram(rows, edges)
	if len(routes) != 1 || routes[0].FromSide != SideBottom || routes[0].ToSide != SideTop {
		t.Fatalf("duong dau = %+v", routes)
	}
	alternatives := DiagramAlternatives(rows, edges, routes, nil, 0, routes[0])
	if len(alternatives) == 0 {
		t.Fatal("phai co ung vien khac")
	}
	for _, route := range alternatives {
		if route.FromSide == routes[0].FromSide && route.ToSide == routes[0].ToSide && route.Elbow == routes[0].Elbow {
			t.Fatalf("khong duoc tra lai dung duong vua hong: %+v", route)
		}
		if !route.Clear {
			t.Fatalf("ung vien phai sach theo mo hinh: %+v", route)
		}
	}
	// Da thu ung vien dau: lan sau khong tra lai no.
	tried := append([]Route{routes[0]}, alternatives[0])
	next := DiagramAlternatives(rows, edges, []Route{alternatives[0]}, nil, 0, tried...)
	for _, route := range next {
		if sameRoute(route, alternatives[0]) {
			t.Fatalf("ung vien da thu bi tra lai: %+v", route)
		}
	}
}

// Vat can ngoai so do (dong chu thich, tieu de): duong vong phia tren khong duoc chay qua no (luot that 25/10/2026).
func TestRouteDiagramAvoidsObstacles(t *testing.T) {
	rows := Diagram(Rect{40, 180, 880, 320}, []int{3})
	// Ba hop mot hang, noi hop 1 -> hop 3 (nhay qua hop giua): vong duoi la lua chon dau. Chan duoi bang vat can -> vong tren.
	below := Rect{40, rows[0][0][1] + rows[0][0][3] + 4, 880, 30}
	routes := RouteDiagram(rows, []Edge{{0, 0, 0, 2}}, below)
	if !routes[0].Clear || routes[0].FromSide != SideTop || PathCrosses(routes[0].Path, below) {
		t.Fatalf("phai vong tren de tranh vat can: %+v", routes[0])
	}
	// Chan ca tren: khong con duong sach -> Clear = false (checkLayout se bao, khong im lang ve sai).
	above := Rect{40, rows[0][0][1] - 34, 880, 30}
	if routes := RouteDiagram(rows, []Edge{{0, 0, 0, 2}}, below, above); routes[0].Clear {
		t.Fatalf("khong co duong sach thi Clear phai false: %+v", routes[0])
	}
}

// Dai xuyen suot nam ben phai, cao bang ca so do, khong de len nut nao; duong vong ben phai (quay nguoc tu tang duoi len
// tang tren) khong cat qua dai khi dai la vat can.
func TestDiagramWithSpansReservesRightStrip(t *testing.T) {
	area := Rect{40, 130, 880, 370}
	rows, spans := DiagramWithSpans(area, []int{1, 2, 2}, 1)
	if len(spans) != 1 {
		t.Fatalf("phai co 1 dai: %v", spans)
	}
	span := spans[0]
	if right := span[0] + span[2]; right > area[0]+area[2]+0.1 || span[1] != rows[0][0][1] ||
		span[1]+span[3] != rows[2][0][1]+rows[2][0][3] {
		t.Fatalf("dai phai nam trong vung, cao tu tang dau toi tang cuoi: %v rows %v", span, rows)
	}
	for _, row := range rows {
		for _, box := range row {
			if box[0]+box[2]+SpanGap > span[0]+0.1 {
				t.Fatalf("nut %v lan vao khe/dai %v", box, span)
			}
		}
	}
	routes := RouteDiagram(rows, []Edge{{2, 1, 0, 0}}, span)
	if !routes[0].Clear {
		t.Fatalf("duong quay nguoc phai tim duoc duong khong cat dai: %+v", routes[0])
	}
	if rows, spans := DiagramWithSpans(area, []int{1, 2}, 0); spans != nil || len(rows) != 2 {
		t.Fatalf("khong co dai thi nhu Diagram: %v %v", rows, spans)
	}
}
