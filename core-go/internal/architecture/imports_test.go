package architecture

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// boundary: importer must NOT import imported (both relative to core-go/internal).
type boundary struct {
	importer string
	imported string
}

// Cac ranh gioi da chot: loi phu thuoc mot chieu de khong keo tang API/agent vao cac goi thap hon.
var boundaries = []boundary{
	// internal/api la lop giao tiep HTTP; khong goi nao o duoi duoc phep keo no vao.
	{"agent", "api"},
	{"tools", "api"},
	{"model", "api"},
	{"store", "api"},
	{"memory", "api"},
	{"policy", "api"},
	{"skills", "api"},
	{"office", "api"},
	{"wpsbridge", "api"},
	// internal/agent dieu phoi; cac goi cong cu/du lieu khong duoc phu thuoc nguoc.
	{"tools", "agent"},
	{"model", "agent"},
	{"store", "agent"},
	{"memory", "agent"},
	{"policy", "agent"},
	{"office", "agent"},
	{"wpsbridge", "agent"},
	// internal/tools chi dung model qua interface/giao thuc, khong import model.
	{"model", "tools"},
	// internal/model la tang thap nhat trong nhom nay.
	{"store", "model"},
	{"memory", "model"},
	{"office", "model"},
}

// knownExceptions: cac canh da bi vi pham trong code hien tai. Go ra khoi day can sua production
// (memory/office dang dung ham tien ich cua model: Truncate, NormalizeEndpoint, BuildURL, MarshalRelaxed),
// nam ngoai pham vi task chi them test. Giu lai de khi refactor xong chi can xoa ngoai le, va de mot
// vi pham MOI o cac canh khac van lam test do.
var knownExceptions = map[string]string{
	"memory->model": "memory dung model.Truncate/NormalizeEndpoint/BuildURL",
	"office->model": "office dung model.MarshalRelaxed",
}

// TestImportBoundaries chay `go list` mot lan tren core-go va kiem tra cac canh cam.
// Bo qua khi may khong co go trong PATH (test chay trong binary khong kem toolchain).
func TestImportBoundaries(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not on PATH")
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("khong xac dinh duoc duong dan test")
	}
	coreGo := filepath.Join(filepath.Dir(file), "..", "..")

	cmd := exec.Command(goBin, "list", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./...")
	cmd.Dir = coreGo
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list ./... trong %s: %v\n%s", coreGo, err, stderr.String())
	}

	imports := map[string]map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		set := make(map[string]bool, len(fields)-1)
		for _, imported := range fields[1:] {
			set[imported] = true
		}
		imports[fields[0]] = set
	}

	const prefix = "axiomoffice/core/internal/"
	for _, rule := range boundaries {
		importer := prefix + rule.importer
		imported := prefix + rule.imported
		if reason, tolerated := knownExceptions[rule.importer+"->"+rule.imported]; tolerated {
			if imports[importer][imported] {
				t.Logf("canh da biet vi pham (ngoai le): %s -> %s (%s)", importer, imported, reason)
			}
			continue
		}
		if imports[importer][imported] {
			t.Errorf("%s khong duoc import %s", importer, imported)
		}
	}
}
