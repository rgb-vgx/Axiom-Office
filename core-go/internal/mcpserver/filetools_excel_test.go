package mcpserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// callTool goi tool qua dung duong JSON-RPC ma client that dung.
func callTool(t *testing.T, server *Server, name string, arguments map[string]any) (map[string]any, bool) {
	t.Helper()
	request := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": arguments},
	}
	reply, ok := server.HandleLine(encode(request))
	if !ok {
		t.Fatalf("%s: khong co tra loi", name)
	}
	var parsed struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(reply), &parsed); err != nil {
		t.Fatalf("%s: tra loi khong phai JSON: %v (%s)", name, err, reply)
	}
	if parsed.Error != nil {
		t.Fatalf("%s: loi JSON-RPC %d: %s", name, parsed.Error.Code, parsed.Error.Message)
	}
	value := map[string]any{}
	if len(parsed.Result.Content) > 0 && parsed.Result.Content[0].Text != "" {
		if err := json.Unmarshal([]byte(parsed.Result.Content[0].Text), &value); err != nil {
			t.Fatalf("%s: ket qua khong phai JSON: %v (%s)", name, err, parsed.Result.Content[0].Text)
		}
	}
	return value, parsed.Result.IsError
}

func excelServer() *Server {
	return NewServer("office-tools", excelFileTools())
}

// require goi tool va doi phai thanh cong.
func require(t *testing.T, server *Server, name string, arguments map[string]any) map[string]any {
	t.Helper()
	value, isError := callTool(t, server, name, arguments)
	if isError {
		t.Fatalf("%s that bai: %v", name, value)
	}
	return value
}

func TestExcelCreateWriteReadProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bang.xlsx")
	server := excelServer()

	created := require(t, server, "excel_create", map[string]any{
		"path": path,
		"sheets": []any{map[string]any{
			"name":   "Data",
			"values": [][]any{{"Tên", "Điểm"}, {"An", 9.5}, {"Bình", int64(8)}},
		}},
	})
	if !reflect.DeepEqual(created["sheets"], []any{"Data"}) {
		t.Fatalf("sheets sau khi tao: %v", created["sheets"])
	}

	require(t, server, "excel_write", map[string]any{
		"path": path, "sheet": "Data", "start_cell": "D1", "values": [][]any{{"Ghi chú"}, {"ok"}},
	})
	read := require(t, server, "excel_read", map[string]any{"path": path, "cell_range": "A1:D3"})
	values, _ := read["values"].([]any)
	if len(values) != 3 {
		t.Fatalf("so dong doc duoc: %v", read["values"])
	}
	first, _ := values[0].([]any)
	if len(first) < 2 || first[0] != "Tên" || first[1] != "Điểm" {
		t.Fatalf("dong tieu de sai: %v", first)
	}
	second, _ := values[1].([]any)
	if second[1] != 9.5 {
		t.Fatalf("o so thuc sai: %v", second[1])
	}
	if len(second) > 3 && second[3] != "ok" {
		t.Fatalf("o ghi bang excel_write sai: %v", second)
	}

	profile := require(t, server, "excel_profile", map[string]any{"path": path})
	if profile["format"] != "xlsx" {
		t.Fatalf("format sai: %v", profile["format"])
	}
	sheets, _ := profile["sheets"].([]any)
	if len(sheets) != 1 {
		t.Fatalf("so sheet sai: %v", sheets)
	}
	only, _ := sheets[0].(map[string]any)
	if only["name"] != "Data" || only["rows"].(float64) < 3 || only["columns"].(float64) < 4 {
		t.Fatalf("profile sai: %v", only)
	}
}

func TestExcelSheetOperationsThroughTools(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bang.xlsx")
	server := excelServer()
	require(t, server, "excel_create", map[string]any{
		"path": path, "sheets": []any{map[string]any{"name": "Data", "values": [][]any{{"a", "b"}}}},
	})

	created := require(t, server, "excel_create_sheet", map[string]any{"path": path, "sheet": "Trống"})
	if got := created["sheets"]; !reflect.DeepEqual(got, []any{"Data", "Trống"}) {
		t.Fatalf("them sheet sai: %v", got)
	}
	// Thieu overwrite thi phai bao loi, va loi phai noi ro "exists".
	if value, isError := callTool(t, server, "excel_create_sheet", map[string]any{"path": path, "sheet": "Trống"}); !isError {
		t.Fatalf("them sheet trung phai loi: %v", value)
	} else if message, _ := value["error"].(string); message == "" {
		t.Fatalf("loi phai co thong bao: %v", value)
	}

	copied := require(t, server, "excel_copy_sheet", map[string]any{"path": path, "src_sheet": "Data", "dst_sheet": "Bản sao"})
	if got := copied["sheets"]; !reflect.DeepEqual(got, []any{"Data", "Trống", "Bản sao"}) {
		t.Fatalf("sao chep sheet sai: %v", got)
	}
	renamed := require(t, server, "excel_rename_sheet", map[string]any{"path": path, "sheet": "Bản sao", "new_name": "Đổi tên"})
	if got := renamed["sheets"]; !reflect.DeepEqual(got, []any{"Data", "Trống", "Đổi tên"}) {
		t.Fatalf("doi ten sheet sai: %v", got)
	}
	deleted := require(t, server, "excel_delete_sheet", map[string]any{"path": path, "sheet": "Trống"})
	if got := deleted["sheets"]; !reflect.DeepEqual(got, []any{"Data", "Đổi tên"}) {
		t.Fatalf("xoa sheet sai: %v", got)
	}
	// Xoa sheet khong ton tai phai bao loi "sheet not found".
	if value, isError := callTool(t, server, "excel_delete_sheet", map[string]any{"path": path, "sheet": "Không có"}); !isError {
		t.Fatalf("xoa sheet khong ton tai phai loi: %v", value)
	}
}

func TestExcelFormatRangeAndTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bang.xlsx")
	server := excelServer()
	require(t, server, "excel_create", map[string]any{
		"path": path,
		"sheets": []any{map[string]any{
			"name":   "Data",
			"values": [][]any{{"Tên", "Điểm"}, {"An", 9.5}, {"Bình", int64(8)}},
		}},
	})

	// Mot object style ap cho ca vung.
	styled := require(t, server, "excel_format_range", map[string]any{
		"path": path, "sheet": "Data", "cell_range": "A2:B3", "styles": map[string]any{"bold": true},
	})
	if styled["styled_cells"] != float64(4) {
		t.Fatalf("so o duoc dinh dang sai: %v", styled["styled_cells"])
	}
	// Style long nhau (font/fill/numFmt) phai tao duoc.
	require(t, server, "excel_format_range", map[string]any{
		"path": path, "sheet": "Data", "cell_range": "A1:B1",
		"styles": map[string]any{
			"font": map[string]any{"bold": true, "color": "#FF0000", "size": 14},
			"fill": map[string]any{"color": "#FFFF00"},
			"alignment": map[string]any{"horizontal": "center", "wrap": true},
			"numFmt":    "#,##0.00",
		},
	})
	// Ma tran style phai khop kich thuoc vung.
	if value, isError := callTool(t, server, "excel_format_range", map[string]any{
		"path": path, "sheet": "Data", "cell_range": "A1:B2", "styles": []any{[]any{map[string]any{"bold": true}}},
	}); !isError {
		t.Fatalf("ma tran sai kich thuoc phai loi: %v", value)
	}

	table := require(t, server, "excel_create_table", map[string]any{
		"path": path, "sheet": "Data", "cell_range": "A1:B3", "table_name": "BangDiem",
	})
	if !reflect.DeepEqual(table["tables"], []any{"BangDiem"}) {
		t.Fatalf("bang sai: %v", table["tables"])
	}
	if value, isError := callTool(t, server, "excel_create_table", map[string]any{
		"path": path, "sheet": "Data", "cell_range": "A1:B3", "table_name": "A1",
	}); !isError {
		t.Fatalf("ten bang giong dia chi o phai loi: %v", value)
	}
	// Du lieu van doc lai duoc sau khi dinh dang va them bang.
	read := require(t, server, "excel_read", map[string]any{"path": path, "cell_range": "A1:B3"})
	values, _ := read["values"].([]any)
	header, _ := values[0].([]any)
	if header[0] != "Tên" || header[1] != "Điểm" {
		t.Fatalf("tieu de bi doi sau khi them bang: %v", header)
	}
}

func TestExcelConvertAndErrors(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "bang.xlsx")
	server := excelServer()
	require(t, server, "excel_create", map[string]any{
		"path": path,
		"sheets": []any{map[string]any{
			"name":   "Data",
			"values": [][]any{{"Tên", "Điểm"}, {"An", 9.5}, {"Bình", int64(8)}},
		}},
	})

	converted := require(t, server, "excel_convert", map[string]any{"path": path, "sheet": "Data"})
	output, _ := converted["output"].(string)
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("file csv ket qua khong ton tai: %v", err)
	}
	columns, _ := converted["columns"].([]any)
	if len(columns) < 2 || columns[0] != "Tên" || columns[1] != "Điểm" {
		t.Fatalf("ten cot sai: %v", columns)
	}
	if converted["rows"] != float64(2) {
		t.Fatalf("so dong sai: %v", converted["rows"])
	}

	// parquet phai bao loi ro rang.
	value, isError := callTool(t, server, "excel_convert", map[string]any{"path": path, "to": "parquet"})
	if !isError {
		t.Fatalf("parquet phai loi: %v", value)
	}
	if message, _ := value["error"].(string); !strings.Contains(message, "parquet") {
		t.Fatalf("thong bao loi parquet sai: %v", value)
	}

	// Dinh dang khong ho tro.
	if _, isError := callTool(t, server, "excel_read", map[string]any{"path": filepath.Join(directory, "a.parquet")}); !isError {
		t.Fatal("dinh dang la phai loi")
	}
	// File khong ton tai.
	if _, isError := callTool(t, server, "excel_profile", map[string]any{"path": filepath.Join(directory, "khong-co.xlsx")}); !isError {
		t.Fatal("file khong ton tai phai loi")
	}
	// Ghi vao .xls bi tu choi.
	if _, isError := callTool(t, server, "excel_write", map[string]any{
		"path": filepath.Join(directory, "cu.xls"), "sheet": "S", "start_cell": "A1", "values": [][]any{{"x"}},
	}); !isError {
		t.Fatal("ghi .xls phai bi tu choi")
	}
}

func TestExcelCsvRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bang.csv")
	server := excelServer()
	require(t, server, "excel_create", map[string]any{
		"path": path,
		"sheets": []any{map[string]any{
			"values": [][]any{{"Tên", "Điểm"}, {"An", 9.5}},
		}},
	})
	written := require(t, server, "excel_write", map[string]any{
		"path": path, "sheet": "bang", "start_cell": "C1", "values": [][]any{{"Ghi chú"}, {`có "ngoặc"`}},
	})
	if written["written"] != float64(2) {
		t.Fatalf("so o ghi sai: %v", written["written"])
	}
	profile := require(t, server, "excel_profile", map[string]any{"path": path})
	sheets, _ := profile["sheets"].([]any)
	only, _ := sheets[0].(map[string]any)
	if only["name"] != "bang" {
		t.Fatalf("ten sheet cua csv phai la ten file: %v", only["name"])
	}
	read := require(t, server, "excel_read", map[string]any{"path": path})
	values, _ := read["values"].([]any)
	first, _ := values[0].([]any)
	if len(first) < 3 || first[2] != "Ghi chú" {
		t.Fatalf("doc lai csv sai: %v", first)
	}
	if read["range"] != "A1:C2" {
		t.Fatalf("range cua csv sai: %v", read["range"])
	}
}

