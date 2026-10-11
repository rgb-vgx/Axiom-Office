// Spec cua chinh `tests/wps/addin_fake.mjs`: nhung API da DO tren WPS that phai duoc mo phong dung.
//
//   node --test tests/wps/test_addin_fake.mjs
//
// Vi sao co file nay: mot fake "than thien" hon WPS that se che loi cua add-in - dung ca da xay ra voi
// `FileSystem.readFileString` (fake cho doc duong dan day du, WPS that nem loi sandbox, nen `et.importCsv`
// hong tren WPS ma 71 test node van xanh). O day ghim lai tung hanh vi da do, kem ngay do.
//
// Do tren WPS 11.1.0.11723 ngay 11/10/2026 (bang day du o tests/wps/REFERENCE.md):
// - `ReadFile(path)`: giai ma UTF-8, thieu file -> nem `"Wrong path!"`, thua tham so -> `参数太多.`
// - `readFileString`/`writeFileString`: sandbox - duong dan co dau phan cach -> nem, ten file tran -> null
// - `readAsBinaryString`: moi byte >= 0x80 -> `0xFF00 | b`; thieu file -> null
// - `writeAsBinaryString`: ghi byte nguyen ven (PDF that doc lai duoc, 1811 byte, `%PDF-1.4` ... `%%EOF`)
import { test } from "node:test";
import assert from "node:assert/strict";

import { makeEtApp } from "./addin_fake.mjs";

const CSV = "Tên,Điểm\nAn,9\n";
// Byte da biet: 'A', UTF-8 cua 'e-circumflex' (C3 AA), LF, UTF-8 euro (E2 82 AC), NUL, FF.
const BYTES = [0x41, 0xC3, 0xAA, 0x0A, 0xE2, 0x82, 0xAC, 0x00, 0xFF];

function fsOf(files) {
  return makeEtApp({ files: files }).FileSystem;
}

test("FileSystem.ReadFile: giai ma UTF-8, dung o byte NUL dau tien, nem khi thieu file", () => {
  const fs = fsOf({ "/tmp/v.csv": CSV, "/tmp/b.bin": BYTES });
  assert.equal(fs.ReadFile("/tmp/v.csv"), CSV, "doc dung chuoi UTF-8 (ke ca dau tieng Viet)");
  assert.equal(fs.ReadFile("/tmp/b.bin"), "Aê\n€", "moi thu sau byte NUL mat (do tren WPS that)");
  assert.throws(() => fs.ReadFile("/tmp/khong-co.csv"), /Wrong path!/, "thieu file -> nem");
  assert.throws(() => fs.ReadFile("/tmp/v.csv", 1), /参数太多/, "thua tham so -> nem");
});

test("FileSystem.readFileString/writeFileString: sandbox cua WPS, khong doc duoc duong dan nao", () => {
  const fs = fsOf({ "/tmp/v.csv": CSV });
  assert.throws(() => fs.readFileString("/tmp/v.csv"), /path cannot contains/);
  assert.throws(() => fs.readFileString("C:\\data\\v.csv"), /path cannot contains/, "ca duong dan Windows");
  assert.equal(fs.readFileString("v.csv"), null, "ten file tran -> null (khong nem)");
  assert.throws(() => fs.writeFileString("/tmp/moi.csv", "a,b\n"), /path cannot contains/);
  assert.equal(fs.existsSync("/tmp/moi.csv"), false, "writeFileString khong ghi duoc gi");
});

test("FileSystem.readAsBinaryString lam hong byte >= 0x80; writeAsBinaryString ghi byte nguyen ven", () => {
  const fs = fsOf({ "/tmp/b.bin": BYTES });
  const mangled = fs.readAsBinaryString("/tmp/b.bin");
  assert.equal(mangled.charCodeAt(1), 0xFFC3, "byte >= 0x80 thanh 0xFF00|b");
  assert.equal(mangled.charCodeAt(3), 0x0A, "byte < 0x80 giu nguyen");
  assert.equal(fs.readAsBinaryString("/tmp/khong-co.bin"), null, "thieu file -> null (khong nem)");

  fs.writeAsBinaryString("/tmp/ra.pdf", "%PDF-1.4\n%%EOF");
  assert.equal(fs.ReadFile("/tmp/ra.pdf"), "%PDF-1.4\n%%EOF", "ghi byte nguyen ven: doc lai dung");
  assert.equal(fs.Exists("/tmp/ra.pdf"), true, "Exists thay file vua ghi bang duong dan tuyet doi");
  assert.equal(fs.existsSync("/tmp/ra.pdf"), true, "existsSync cung thay (add-in thu ca hai)");
  assert.equal(fs.Exists("/tmp/khong-co.pdf"), false);
  assert.equal(fs.tmpdir(), "/tmp");
});

test("FileSystem: thanh vien chua do thi nem ro rang, khong tra undefined im lang", () => {
  const fs = fsOf({});
  assert.throws(() => fs.mkdtempSync("/tmp/x"), /fake chua mo phong/,
    "chua do API nao thi phai nem, de khong ai tuong no chay duoc");
});
