// Spec T7 (docs/wps-bridge.md muc 5): 23 lenh `et.*` (Spreadsheets) cua JS add-in WPS.
//
//   node --test tests/wps/test_addin_et.mjs
//
// Khong can WPS: `addin_fake.mjs` mo phong object model ET da do tren WPS that 11/10/2026
// (tests/wps/REFERENCE.md: Range("A1") la LOI GOI, Cells.Item(r,c), UsedRange.Address() la ham,
// mau BGR, chi so 1-based, KHONG co UndoRecord, ExportAsFixedFormat khong ghi file).
//
// Hop dong (giong het ban LibreOffice cho cung ten lenh, tru vai cho ghi ro duoi day):
//   et.newWorkbook {}                      -> {name, fullName, saved, opened}
//   et.open {path}                         -> {name, fullName, saved, opened}
//   et.listSheets {}                       -> {workbook, activeSheet, sheets:[ten]}
//   et.sheetInfo {}                        -> {activeSheet, sheets:[{name, usedRange, rows, cols, empty}]}
//   et.addSheet {name?, index?}            -> {sheet, sheets}; ten trong -> Sheet1, Sheet2... (tranh trung)
//   et.renameSheet {sheet, name}           -> {sheet, sheets}
//   et.activateSheet {sheet}               -> {active}
//   et.readRange {range, sheet?, formulas?}-> {sheet, range, values} | {sheet, range, formulas}
//   et.writeRange {range, values, sheet?}  -> {written, sheet}
//   et.importCsv {path, range?, sheet?, delimiter?, encoding?}
//                                          -> {path, sheet, range, rows, columns, cells}
//   et.writeRanges {writes:[{range, values, sheet?}]} -> {ranges, written, sheets}
//   et.fillRange {range, formula?, sheet?} -> {filled, range, sheet, sample}
//   et.formatRange {range, bold, italic, fontSize, fontColor, fillColor, numFmt, horizontal, wrap, sheet?}
//                                          -> {sheet, range}
//   et.setConditionalFormat {range, rules, sheet?} -> {sheet, range, rules}
//   et.listConditionalFormats {range?, sheet?}     -> {sheet, formats:[{range, rules}]}
//   et.addChart {range, type, title?, name?, anchor?, width?, height?, sheet?}
//                                          -> {chart, sheet, type, diagram, typeApplied, titleApplied, range, anchor}
//   et.listCharts {sheet?}                 -> {sheet, charts:[{name, diagram}], shapes}
//   et.undo {count?}                       -> {undone, restored}
//   et.exportPdf {path}                    -> {exported}
//   et.save {} / et.saveAs {path}          -> {saved, fullName}   (so chua luu lan nao -> "never been saved")
//   et.closeAll {}                         -> {closed, rescued}
//   et.checkRange {range?, sheet?}         -> {sheet, range, usedRange, table, rows, cols, title, columns, issueCount, issues}
//
// Khac ban LibreOffice (do WPS that, ghi o docs/wps-bridge.md muc 6 va REFERENCE.md):
// - `undo`: ET khong co UndoRecord. Add-in TU CHUP gia tri o truoc khi ghi (va ghi nhan sheet do luot
//   truoc tao), roi khoi phuc theo thu tu nguoc. Chi hoan tac duoc trong cung phien add-in.
// - `exportPdf`: API goc (ExportAsFixedFormat/SaveAs .pdf) bao thanh cong ma KHONG ghi file, nen add-in
//   kiem bang FileSystem.existsSync roi tu dung PDF. Ket qua phai la file "%PDF" that.
// - `setConditionalFormat`: WPS ET khong co cell style CO TEN (LibreOffice gan rule bang styleName).
//   Ban WPS dung dinh dang truc tiep (bold/italic/fontColor/fillColor) va tra `styleName` ve loi.
// - `checkRange`: `table` luon "" (khong co CurrentRegion trong object model da do) va khong bao
//   `outside-table`; sau loai issue con lai thi ban WPS lam duoc.
// - `closeAll`: khong co ban cuu ho rieng cua LibreOffice, nhung add-in tu SaveAs mot ban cuu ho vao
//   <tmpdir>/rescued truoc khi dong so chua luu (WPS SaveAs ghi file that).
// - `importCsv`: WPS chi doc duoc file qua `FileSystem.ReadFile` (UTF-8; `readFileString`/`writeFileString` bi
//   sandbox - nem voi moi duong dan co dau phan cach). Nen `encoding` chi nhan cac cach viet UTF-8
//   (mac dinh "utf-8-sig"; chuan hoa "_" -> "-", khong phan biet hoa/thuong), giao thuc khac bi tu choi ro rang
//   thay vi am tham tra ve chuoi hong; BOM dau file bi cat. Chi tiet: docs/wps-bridge.md muc 6, REFERENCE.md.
// - Ghi o: ET EP KIEU chuoi trong nhu so / nhu ngay khi gan vao `Value2` ("42" -> 42, "01/01" -> ngay), khac
//   LibreOffice (giu nguyen chuoi). Fake CHUA mo phong viec ep kieu nay (xem ghi chu o setCellScalar trong
//   addin_fake.mjs): spec o day kiem hanh vi cua add-in, khong kiem lai luat ep kieu cua WPS.
import { test } from "node:test";
import assert from "node:assert/strict";

import { makeEtApp, callsOf } from "./addin_fake.mjs";
import { addinCatalog, assertFails, assertKindRefused, assertMetadata, loadAddin, mockApp, run, throwing } from "./addin_harness.mjs";

const ET_COMMANDS = [
  "et.newWorkbook", "et.open", "et.listSheets", "et.sheetInfo", "et.addSheet", "et.renameSheet",
  "et.activateSheet", "et.readRange", "et.writeRange", "et.importCsv", "et.writeRanges", "et.fillRange",
  "et.formatRange", "et.setConditionalFormat", "et.listConditionalFormats", "et.addChart", "et.listCharts",
  "et.undo", "et.exportPdf", "et.save", "et.saveAs", "et.closeAll", "et.checkRange",
];

const MUTATING = ["et.addSheet", "et.renameSheet", "et.writeRange", "et.importCsv", "et.writeRanges",
  "et.fillRange", "et.formatRange", "et.setConditionalFormat", "et.addChart"];

// Lenh PHAI tu choi khi thieu tham so bat buoc. `et.readRange` khong co trong day vi ban LibreOffice
// van chay voi `range` mac dinh "A1" (metadata ghi required nhung handler co default) - add-in giu
// dung hanh vi runtime cua LibreOffice, xem test readRange ben duoi.
const REQUIRES_PARAM = ["et.open", "et.renameSheet", "et.activateSheet", "et.writeRange", "et.importCsv",
  "et.writeRanges", "et.fillRange", "et.formatRange", "et.setConditionalFormat", "et.addChart",
  "et.exportPdf", "et.saveAs"];

test("registry: du 23 lenh et.*, metadata giong het registry LibreOffice", () => {
  const ctx = loadAddin();
  const mine = addinCatalog(ctx).filter((c) => c.name.indexOf("et.") === 0);
  assertMetadata(ET_COMMANDS, mine);
  assert.equal(mine.length, ET_COMMANDS.length, "catalog co lenh et.* la khong co trong danh sach nay");
  for (const command of mine) {
    assert.equal(command.kind, "et", command.name + ": lenh et.* phai co kind 'et'");
  }
});

// ET khong co UndoRecord (REFERENCE.md): lenh sua tai lieu KHONG duoc mo UndoRecord nhu ban Writer.
test("lenh et.* sua tai lieu khong mo UndoRecord (ET khong co UndoRecord)", () => {
  const ctx = loadAddin();
  const params = {
    "et.addSheet": {}, "et.renameSheet": { sheet: "S", name: "T" },
    "et.writeRange": { range: "A1", values: [["x"]] }, "et.importCsv": { path: "/tmp/khong-co.csv" },
    "et.writeRanges": { writes: [{ range: "A1", values: [["x"]] }] },
    "et.fillRange": { range: "A1", formula: "=1" }, "et.formatRange": { range: "A1", bold: true },
    "et.setConditionalFormat": { range: "A1", rules: [{ operator: "less", formula1: "1" }] },
    "et.addChart": { range: "A1:B2" },
  };
  for (const action of MUTATING) {
    const app = makeEtApp({ sheets: [{ name: "S", cells: { A1: 1, B1: 2, A2: 3, B2: 4 } }] });
    ctx.AxiomCommands.execute(app, action, params[action]);
    const log = callsOf(app).join("\n");
    assert.ok(!/UndoRecord/.test(log), action + " khong duoc dung UndoRecord: " + log);
  }
});

test("et.listSheets / et.readRange / et.writeRange: ghi roi doc lai, cong thuc, o loi", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "Sheet1", cells: { A1: "Ten", A2: "An", B2: 9.5 } }] });

  const sheets = run(ctx, app, "et.listSheets");
  assert.equal(sheets.workbook, "blank.xlsx");
  assert.equal(sheets.activeSheet, "Sheet1");
  assert.deepEqual(sheets.sheets, ["Sheet1"]);

  assert.equal(run(ctx, app, "et.writeRange", { range: "B1", values: [["Diem"]] }).written, 1);
  assert.equal(run(ctx, app, "et.writeRange", { range: "C1", values: [[1, 2], [3, 4]] }).written, 4);
  const read = run(ctx, app, "et.readRange", { range: "A1:D2" });
  assert.equal(read.sheet, "Sheet1");
  assert.equal(read.range, "A1:D2");
  assert.deepEqual(read.values, [["Ten", "Diem", 1, 2], ["An", 9.5, 3, 4]]);

  // Cong thuc: o cong thuc tra ve cong thuc khi formulas=true, gia tri khi khong.
  run(ctx, app, "et.writeRange", { range: "F1", values: [["=C1*10"], ["=1/0"]] });
  assert.deepEqual(run(ctx, app, "et.readRange", { range: "F1:F2", formulas: true }).formulas,
    [["=C1*10"], ["=1/0"]]);
  const values = run(ctx, app, "et.readRange", { range: "F1:F2" });
  assert.equal(values.values[0][0], 10);
  assert.equal(values.values[1][0], null, "o loi: Value2 null (REFERENCE.md)");

  // values phai la mang 2 chieu.
  assertFails(ctx, app, "et.writeRange", { range: "A1", values: ["x"] }, /values/);
  assertFails(ctx, app, "et.writeRange", { values: [["x"]] }, /'range' is required/);
  // sheet khong ton tai -> loi ro rang, khong doan bua.
  assertFails(ctx, app, "et.readRange", { range: "A1", sheet: "KhongCo" }, /no sheet named/);
  // Bo trong range -> doc A1 (giong ban LibreOffice).
  const fallback = run(ctx, app, "et.readRange", {});
  assert.equal(fallback.range, "A1");
  assert.deepEqual(fallback.values, [["Ten"]]);
});

test("et.newWorkbook / et.open: tra ve tai lieu dang mo", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "S" }] });
  const made = run(ctx, app, "et.newWorkbook");
  assert.equal(typeof made.name, "string");
  assert.equal(typeof made.fullName, "string");
  assert.equal(made.opened, true);
  assert.equal(app.Workbooks.Count, 2, "so moi phai duoc them vao WPS");

  const opened = run(ctx, app, "et.open", { path: "/tmp/bao-cao.xlsx" });
  assert.equal(opened.name, "bao-cao.xlsx");
  assert.equal(opened.fullName, "/tmp/bao-cao.xlsx");
  assert.equal(opened.saved, true);
  assert.equal(opened.opened, true);
  assert.equal(app.Workbooks.Count, 3);
  assertFails(ctx, app, "et.open", {}, /'path' is required/);
});

test("et.sheetInfo: kich thuoc dung that tung sheet, sheet trong = empty", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [
    { name: "Trong" },
    { name: "DuLieu", cells: { A1: "Ten", B1: "So", A2: "An", B2: 2 } },
  ] });
  const info = run(ctx, app, "et.sheetInfo");
  assert.equal(info.activeSheet, "Trong");
  const byName = new Map(info.sheets.map((s) => [s.name, s]));
  assert.deepEqual(byName.get("Trong"), { name: "Trong", usedRange: null, rows: 0, cols: 0, empty: true });
  assert.equal(byName.get("DuLieu").usedRange, "A1:B2");
  assert.equal(byName.get("DuLieu").rows, 2);
  assert.equal(byName.get("DuLieu").cols, 2);
  assert.equal(byName.get("DuLieu").empty, false);
});

test("et.addSheet / et.renameSheet / et.activateSheet", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "Sheet1" }] });
  const added = run(ctx, app, "et.addSheet", {});
  assert.equal(added.sheet, "Sheet2", "ten trong -> ten kieu Excel, tranh trung");
  assert.deepEqual(added.sheets, ["Sheet1", "Sheet2"]);
  const atOne = run(ctx, app, "et.addSheet", { name: "Bang", index: 1 });
  assert.equal(atOne.sheet, "Bang");
  assert.deepEqual(atOne.sheets, ["Sheet1", "Bang", "Sheet2"], "index la vi tri chen 0-based");
  assertFails(ctx, app, "et.addSheet", { name: "Bang" }, /already exists/);

  const renamed = run(ctx, app, "et.renameSheet", { sheet: "Bang", name: "SoLieu" });
  assert.equal(renamed.sheet, "SoLieu");
  assert.deepEqual(renamed.sheets, ["Sheet1", "SoLieu", "Sheet2"]);
  assertFails(ctx, app, "et.renameSheet", { sheet: "KhongCo", name: "X" }, /no sheet named/);
  assertFails(ctx, app, "et.renameSheet", { sheet: "SoLieu" }, /'name' is required/);

  assert.equal(run(ctx, app, "et.activateSheet", { sheet: "SoLieu" }).active, "SoLieu");
  assert.equal(run(ctx, app, "et.listSheets").activeSheet, "SoLieu");
  assertFails(ctx, app, "et.activateSheet", {}, /'sheet' is required/);
});

test("et.importCsv: doc file bang FileSystem.ReadFile (readFileString cua WPS bi sandbox)", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ files: { "/tmp/ban-hang.csv": "Ten,So\nAn,42\nBinh,7\n" } });
  const imported = run(ctx, app, "et.importCsv", { path: "/tmp/ban-hang.csv" });
  assert.equal(imported.path, "/tmp/ban-hang.csv");
  assert.equal(imported.sheet, "ban-hang");
  assert.equal(imported.range, "A1");
  assert.equal(imported.rows, 3);
  assert.equal(imported.columns, 2);
  assert.equal(imported.cells, 6);
  assert.deepEqual(run(ctx, app, "et.readRange", { range: "A1:B3", sheet: "ban-hang" }).values,
    [["Ten", "So"], ["An", 42], ["Binh", 7]], "o trong nhu so thi thanh so");
  assert.ok(run(ctx, app, "et.listSheets").sheets.includes("ban-hang"));
  const calls = callsOf(app);
  assert.ok(calls.indexOf("FileSystem.ReadFile(/tmp/ban-hang.csv)") >= 0,
    "phai doc bang ReadFile: " + calls.join(" | "));
  assert.ok(!calls.some((line) => line.indexOf("readFileString") >= 0),
    "readFileString cua WPS bi sandbox, khong doc duoc duong dan nao: " + calls.join(" | "));

  // Co range: ghi vao o goc do cua sheet chi dinh, khong tao sheet moi.
  const app2 = makeEtApp({ sheets: [{ name: "Sheet1" }], files: { "/tmp/x.csv": "a;b\n1;2\n" } });
  const into = run(ctx, app2, "et.importCsv", { path: "/tmp/x.csv", range: "C3", delimiter: ";" });
  assert.equal(into.sheet, "Sheet1");
  assert.equal(into.range, "C3");
  assert.deepEqual(run(ctx, app2, "et.listSheets").sheets, ["Sheet1"]);
  assert.deepEqual(run(ctx, app2, "et.readRange", { range: "C3:D4" }).values, [["a", "b"], [1, 2]]);

  assertFails(ctx, app, "et.importCsv", {}, /'path' is required/);
  assertFails(ctx, app, "et.importCsv", { path: "/tmp/khong-co.csv" }, /no file at/);
  assertFails(ctx, app, "et.importCsv", { path: "/tmp/ban-hang.csv", delimiter: ";;" }, /single character/);
});

test("et.importCsv: cat BOM cua utf-8-sig, tu choi encoding khong phai UTF-8", () => {
  const ctx = loadAddin();
  // ReadFile tra ve nguyen noi dung file ke ca BOM (xem makeFileSystem), nen add-in phai tu cat theo utf-8-sig:
  // khong cat thi o dau tien thanh "\uFEFFTen" - sai ten cot va sai moi so sanh sau do.
  const app = makeEtApp({ files: { "/tmp/bom.csv": "﻿Ten,So\nAn,42\n" } });
  const out = run(ctx, app, "et.importCsv", { path: "/tmp/bom.csv" });
  assert.equal(out.rows, 2);
  assert.deepEqual(run(ctx, app, "et.readRange", { range: "A1:B2", sheet: "bom" }).values,
    [["Ten", "So"], ["An", 42]], "BOM khong duoc dinh vao o dau tien");

  // WPS khong co API doc theo charset (ReadFile nhan dung mot tham so) nen encoding khac UTF-8 phai bi tu choi
  // ro rang, khong duoc am tham tra ve chuoi hong.
  const before = run(ctx, app, "et.listSheets").sheets.slice();
  assertFails(ctx, app, "et.importCsv", { path: "/tmp/bom.csv", encoding: "cp1258" }, /utf-8/i);
  assert.deepEqual(run(ctx, app, "et.listSheets").sheets, before, "tu choi truoc khi ghi: khong tao sheet nao");

  // Cac cach viet UTF-8 khac van chay (LibreOffice nhan qua Python, o day chuan hoa "_" -> "-", khong phan biet hoa/thuong).
  const ok = makeEtApp({ files: { "/tmp/u.csv": "Ten,So\nAn,1\n" } });
  for (const encoding of ["utf-8-sig", "UTF-8", "utf8", "utf_8_sig"]) {
    assert.equal(run(ctx, ok, "et.importCsv", { path: "/tmp/u.csv", range: "A1", encoding: encoding }).rows, 2,
      "encoding '" + encoding + "' phai chay");
  }
});

test("et.writeRanges: gop nhieu vung, kiem HET truoc khi ghi", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "S" }] });
  const out = run(ctx, app, "et.writeRanges", { writes: [
    { range: "A1", values: [["Ten", "So"]] },
    { range: "A2", values: [["An", 1]] },
  ] });
  assert.equal(out.ranges, 2);
  assert.equal(out.written, 4);
  assert.deepEqual(out.sheets, ["S"]);
  assert.deepEqual(run(ctx, app, "et.readRange", { range: "A1:B2" }).values, [["Ten", "So"], ["An", 1]]);

  // Vung thu hai sai -> khong vung nao duoc ghi (kiem het tham so truoc).
  assertFails(ctx, app, "et.writeRanges", { writes: [{ range: "D1", values: [["x"]] }, { values: [["y"]] }] },
    /writes\[1\]/);
  assert.equal(run(ctx, app, "et.readRange", { range: "D1" }).values[0][0], null, "khong duoc ghi vung nao");
  assertFails(ctx, app, "et.writeRanges", {}, /'writes' is required/);
  assertFails(ctx, app, "et.writeRanges", { writes: [] }, /'writes' is required/);
});

test("et.fillRange: mot cong thuc roi dien, tham chieu tuong doi tu dich", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "S", cells: { A1: 2, A2: 3, A3: 4 } }] });
  const out = run(ctx, app, "et.fillRange", { range: "B1:B3", formula: "=A1*2" });
  assert.equal(out.filled, 3);
  assert.equal(out.range, "B1:B3");
  assert.equal(out.sheet, "S");
  assert.deepEqual(out.sample, [["=A1*2"], ["=A2*2"], ["=A3*2"]]);
  assert.deepEqual(run(ctx, app, "et.readRange", { range: "B1:B3", formulas: true }).formulas,
    [["=A1*2"], ["=A2*2"], ["=A3*2"]]);
  assert.deepEqual(run(ctx, app, "et.readRange", { range: "B1:B3" }).values, [[4], [6], [8]]);
  assertFails(ctx, app, "et.fillRange", {}, /'range' is required/);
});

test("et.formatRange: co chu, mau BGR, numFmt, can le", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "S", cells: { A1: 1.5, A2: 2.5 } }] });
  const out = run(ctx, app, "et.formatRange", { range: "A1:A2", bold: true, numFmt: "#,##0.00",
    fontColor: "#FF0000", fillColor: "#00FF00", horizontal: "center", wrap: true });
  assert.equal(out.sheet, "S");
  assert.equal(out.range, "A1:A2");

  const cell = app.ActiveSheet.Range("A1");
  assert.equal(cell.Font.Bold, true);
  assert.equal(cell.NumberFormat, "#,##0.00");
  assert.equal(cell.Font.Color, 255, "#FF0000 -> BGR 0x0000FF");
  assert.equal(cell.Interior.Color, 65280, "#00FF00 -> BGR 0x00FF00");
  assert.equal(cell.HorizontalAlignment, -4108, "center = -4108 (REFERENCE.md)");
  assert.equal(cell.WrapText, true);
  assert.equal(app.ActiveSheet.Range("A2").Font.Bold, true, "dinh dang ca vung, khong chi o goc");

  assertFails(ctx, app, "et.formatRange", { range: "A1", horizontal: "giua" }, /left\/center\/right/);
  assertFails(ctx, app, "et.formatRange", {}, /'range' is required/);
});

test("et.setConditionalFormat / et.listConditionalFormats: ghi roi doc lai", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "S", cells: { A1: "So", A2: 10, A3: 60 } }] });
  const set = run(ctx, app, "et.setConditionalFormat", { range: "A2:A3", rules: [
    { operator: "less", formula1: "50", fillColor: "#FF0000", bold: true },
    { operator: "between", formula1: "10", formula2: "20" },
  ] });
  assert.equal(set.sheet, "S");
  assert.equal(set.rules.length, 2);
  assert.equal(set.rules[0].operator, "less");
  assert.equal(set.rules[0].formula1, "50");
  assert.deepEqual(set.rules.map((r) => r.operator), ["less", "between"]);

  const listed = run(ctx, app, "et.listConditionalFormats", { range: "A2:A3" });
  assert.equal(listed.sheet, "S");
  assert.equal(listed.formats.length, 1, "mot vung mot bo rule: " + JSON.stringify(listed));
  assert.ok(typeof listed.formats[0].range === "string" && listed.formats[0].range.length > 0);
  assert.deepEqual(listed.formats[0].rules.map((r) => r.operator), ["less", "between"]);
  assert.equal(listed.formats[0].rules[0].formula1, "50");
  assert.equal(listed.formats[0].rules[0].formula2, null, "operator 'less' khong dung formula2 -> null");
  assert.equal(listed.formats[0].rules[1].formula2, "20");
  // Bo trong range: van liet ke duoc (quet vung dung that), khong duoc tra rong.
  assert.ok(run(ctx, app, "et.listConditionalFormats").formats.length >= 1);

  assertFails(ctx, app, "et.setConditionalFormat", { range: "A1", rules: [
    { operator: "between", formula1: "1" }] }, /formula2/);
  assertFails(ctx, app, "et.setConditionalFormat", { range: "A1", rules: [
    { operator: "khong-co", formula1: "1" }] }, /must be one of/);
  assertFails(ctx, app, "et.setConditionalFormat", { range: "A1", rules: [{ operator: "less" }] }, /formula1/);
  assertFails(ctx, app, "et.setConditionalFormat", { range: "A1", rules: [] }, /'rules' is required/);
  assertFails(ctx, app, "et.setConditionalFormat", { range: "A1" }, /'rules' is required/);
  // ET khong co cell style co ten nhu LibreOffice -> phai tu choi chu khong bo qua im lang.
  assertFails(ctx, app, "et.setConditionalFormat", { range: "A1", rules: [
    { operator: "less", formula1: "1", styleName: "Bad" }] }, /styleName/);

  // Kiem tra het rule TRUOC khi ghi: rule thu hai hong thi vung moi khong duoc dinh gi.
  assertFails(ctx, app, "et.setConditionalFormat", { range: "D1", rules: [
    { operator: "less", formula1: "1" }, { operator: "greater" }] }, /formula1/);
  assert.equal(app.ActiveSheet.Range("D1").FormatConditions.Count, 0, "rule hong -> khong ghi gi");
});

test("et.addChart / et.listCharts: chen bieu do va doc lai kieu that", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "S", cells: {
    A1: "Thang", B1: "Doanh thu", A2: "T1", B2: 10, A3: "T2", B3: 20 } }] });
  const added = run(ctx, app, "et.addChart", { range: "A1:B3", type: "column", title: "Doanh thu",
    width: 12, height: 7 });
  assert.equal(added.sheet, "S");
  assert.equal(added.type, "column");
  assert.equal(added.diagram, "column");
  assert.equal(added.titleApplied, true);
  assert.equal(added.range, "A1:B3");
  assert.equal(added.anchor, "A1");
  assert.equal(typeof added.chart, "string");

  const objects = app.ActiveSheet.ChartObjects();
  assert.equal(objects.Count, 1);
  const object = objects.Item(1);
  assert.equal(object.Chart.ChartType, 51, "column = xlColumnClustered (51)");
  assert.equal(object.Chart.ChartTitle.Text, "Doanh thu");
  assert.ok(Math.abs(object.Width - 12 * 28.3465) < 0.1, "width tinh bang cm -> point: " + object.Width);
  assert.ok(Math.abs(object.Height - 7 * 28.3465) < 0.1, "height tinh bang cm -> point: " + object.Height);
  assert.equal(object.Left, app.ActiveSheet.Range("A1").Left, "anchor A1 -> neo theo o");
  assert.equal(object.Top, app.ActiveSheet.Range("A1").Top);

  const listed = run(ctx, app, "et.listCharts");
  assert.equal(listed.sheet, "S");
  assert.equal(listed.charts.length, 1);
  assert.equal(listed.charts[0].diagram, "column");
  assert.equal(listed.charts[0].name, added.chart);
  assert.ok(Array.isArray(listed.shapes));

  assertFails(ctx, app, "et.addChart", { range: "A1:B3", type: "khong-co" }, /must be one of/);
  assertFails(ctx, app, "et.addChart", {}, /'range' is required/);
});

test("et.undo: khoi phuc gia tri o (ET khong co undo cua WPS)", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "S", cells: { A1: "cu", B1: 1 } }] });
  run(ctx, app, "et.writeRange", { range: "A1:B1", values: [["moi", 99]] });
  assert.deepEqual(run(ctx, app, "et.readRange", { range: "A1:B1" }).values, [["moi", 99]]);

  const undone = run(ctx, app, "et.undo", {});
  assert.equal(undone.undone, 1);
  assert.equal(undone.restored.length, 1);
  assert.equal(undone.restored[0].sheet, "S");
  assert.equal(undone.restored[0].range, "A1:B1");
  assert.equal(undone.restored[0].cells, 2);
  assert.deepEqual(run(ctx, app, "et.readRange", { range: "A1:B1" }).values, [["cu", 1]]);

  // Het buoc -> undone 0, khong loi.
  assert.deepEqual(run(ctx, app, "et.undo", {}), { undone: 0, restored: [] });
  assert.deepEqual(run(ctx, app, "et.undo", { count: 5 }), { undone: 0, restored: [] });
});

test("et.undo: o cong thuc duoc khoi phuc thanh cong thuc, khong phai gia tri", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "S", cells: { A1: 2, B1: { formula: "=A1*3" } } }] });
  run(ctx, app, "et.writeRange", { range: "B1", values: [[0]] });
  assert.deepEqual(run(ctx, app, "et.readRange", { range: "B1" }).values, [[0]]);

  run(ctx, app, "et.undo", {});
  assert.deepEqual(run(ctx, app, "et.readRange", { range: "B1", formulas: true }).formulas, [["=A1*3"]]);
  assert.deepEqual(run(ctx, app, "et.readRange", { range: "B1" }).values, [[6]]);
});

test("et.undo: xoa sheet do chinh luot truoc tao ra", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "Sheet1" }] });
  run(ctx, app, "et.addSheet", { name: "Tam" });
  const undone = run(ctx, app, "et.undo", {});
  assert.equal(undone.undone, 1);
  assert.equal(undone.restored[0].sheet, "Tam");
  assert.equal(undone.restored[0].deleted, true);
  assert.deepEqual(run(ctx, app, "et.listSheets").sheets, ["Sheet1"]);
});

test("et.undo {count}: go N buoc theo thu tu nguoc", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "S", cells: { A1: "a0", A2: "a1" } }] });
  run(ctx, app, "et.writeRange", { range: "A1", values: [["x"]] });
  run(ctx, app, "et.writeRange", { range: "A2", values: [["y"]] });
  const undone = run(ctx, app, "et.undo", { count: 2 });
  assert.equal(undone.undone, 2);
  assert.equal(undone.restored.length, 2);
  assert.deepEqual(undone.restored.map((r) => r.range), ["A2", "A1"], "go buoc moi nhat truoc");
  assert.deepEqual(run(ctx, app, "et.readRange", { range: "A1:A2" }).values, [["a0"], ["a1"]]);
});

test("et.exportPdf: API goc khong ghi file -> tu dung PDF that", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "S", cells: { A1: "Xin chao", A2: 42 } }] });
  const out = run(ctx, app, "et.exportPdf", { path: "/tmp/et-ra.pdf" });
  assert.equal(out.exported, "/tmp/et-ra.pdf");
  assert.equal(app.FileSystem.existsSync("/tmp/et-ra.pdf"), true, "phai ghi file that (WPS bao xong ma khong ghi)");
  const pdf = app.FileSystem.ReadFile("/tmp/et-ra.pdf");
  assert.equal(pdf.slice(0, 5), "%PDF-", "phai la PDF that");
  assert.ok(pdf.indexOf("%%EOF") > 0, "PDF phai co %%EOF");
  assert.ok(pdf.length > 300, "PDF rong qua thi khong phai PDF that: " + pdf.length);
  assertFails(ctx, app, "et.exportPdf", {}, /'path' is required/);
});

test("et.save / et.saveAs / et.closeAll", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "S" }] });
  const saved = run(ctx, app, "et.save");
  assert.equal(saved.saved, true);
  assert.equal(saved.fullName, "/tmp/blank.xlsx");
  assert.ok(callsOf(app).some((line) => line === "Workbook.Save()"), "phai goi Save() cua WPS");

  const as = run(ctx, app, "et.saveAs", { path: "/tmp/moi.xlsx" });
  assert.equal(as.saved, true);
  assert.equal(as.fullName, "/tmp/moi.xlsx");
  assert.equal(app.ActiveWorkbook.Name, "moi.xlsx");
  assertFails(ctx, app, "et.saveAs", {}, /'path' is required/);

  // So chua luu lan nao (Path rong): Save() se mo hop thoai Save As cua WPS va chan vong poll,
  // nen phai tu choi ro rang nhu ban LibreOffice.
  const untitled = makeEtApp({ path: "blank.xlsx" });
  assertFails(ctx, untitled, "et.save", {}, /never been saved/);

  // So chua luu: phai cuu mot ban truoc khi dong (LibreOffice lam viec nay; WPS khong co san).
  app.ActiveWorkbook.Saved = false;
  const closed = run(ctx, app, "et.closeAll");
  assert.equal(closed.closed, 1);
  assert.equal(app.Workbooks.Count, 0, "closeAll phai dong so");
  assert.equal(closed.rescued.length, 1);
  assert.match(closed.rescued[0], /rescued/);
  const log = callsOf(app);
  // ET ghi nhat ky co tien to "Workbook." (ban WPP ghi tran "SaveAs(...)"), nen khop ca hai dang.
  assert.ok(log.some((line) => /(^|\.)SaveAs\(/.test(line) && line.indexOf("rescued") > 0),
    "phai SaveAs ban cuu ho truoc khi Close: " + log.slice(-6).join(" | "));
});

test("et.checkRange: soat bang du lieu (tieu de trong, kieu lan, so dang chu, o loi, numFmt)", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "Bang", cells: {
    A1: "Ten", B1: "So", C1: "", D1: "Loi", E1: "Le", F1: "Cong",
    A2: "An", B2: "42", C2: 1, D2: { formula: "=1/0" }, E2: 1.5, F2: { formula: "=A99*2" },
    A3: "Binh", B3: "7", C3: "x", D3: "ok", E3: 2.5,
  } }] });
  const report = run(ctx, app, "et.checkRange", { range: "A1:F3" });
  assert.equal(report.sheet, "Bang");
  assert.equal(report.range, "A1:F3");
  assert.equal(report.usedRange, "A1:F3");
  assert.equal(report.rows, 3);
  assert.equal(report.cols, 6);
  assert.equal(report.title, null);
  assert.equal(report.table, "", "WPS khong co CurrentRegion -> khong bao vung bang duoc");
  assert.equal(report.issueCount, report.issues.length);
  assert.ok(report.issues.every((i) => typeof i.type === "string" && typeof i.detail === "string"),
    "moi issue phai co type + detail");

  const types = new Set(report.issues.map((i) => i.type));
  for (const wanted of ["empty-header", "numbers-as-text", "mixed-types", "error-values",
    "no-number-format", "empty-reference"]) {
    assert.ok(types.has(wanted), "thieu issue " + wanted + ": " + JSON.stringify(report.issues));
  }
  const columns = new Map(report.columns.map((c) => [c.column, c]));
  assert.deepEqual([...columns.keys()], ["A", "B", "C", "D", "E", "F"]);
  assert.equal(columns.get("A").header, "Ten");
  assert.equal(columns.get("A").texts, 2);
  assert.equal(columns.get("A").numbers, 0);
  assert.equal(columns.get("B").texts, 2, "'42' la chu");
  assert.equal(columns.get("B").numbers, 0);
  assert.equal(columns.get("C").header, null, "tieu de trong -> null");
  assert.equal(columns.get("D").formulas, 1);
  assert.equal(columns.get("D").blanks, 0);
  assert.equal(columns.get("F").formulas, 1);
  assert.equal(columns.get("F").blanks, 1, "F3 trong vung nhung khong co du lieu -> 1 o trong");
  assert.equal(columns.get("E").numberFormat, "General");

  // Dong tieu de: dong 1 chi co mot o chu, dong 2 co chu o it nhat nua so cot -> do la tieu de.
  const app2 = makeEtApp({ sheets: [{ name: "T", cells: {
    A1: "Bao cao", A2: "Ten", B2: "So", C2: "Ngay", D2: "Ghi",
    A3: "An", B3: 1, C3: "01/01", D3: "x",
  } }] });
  const titled = run(ctx, app2, "et.checkRange");
  assert.equal(titled.title, "Bao cao", "phai nhan ra dong tieu de: " + JSON.stringify(titled.columns));
  assert.equal(titled.columns[0].header, "Ten", "header nam o dong 2 khi co tieu de");
  assert.equal(titled.range, "A1:D3");
});

test("et.* chi chay trong Spreadsheets; lenh la va object model hong khong throw", () => {
  const ctx = loadAddin();
  for (const app of [{ Name: "WPS Writer", ActiveDocument: { Name: "x.docx" } },
    { Name: "WPS Presentation", ActivePresentation: { Name: "x.pptx" } }]) {
    const reply = assertKindRefused(ctx, app, "et.listSheets");
    assert.match(String(reply.error), /et\.listSheets/);
  }

  const app = makeEtApp({ sheets: [{ name: "S" }] });
  const unknown = ctx.AxiomCommands.execute(app, "et.khongCo", {});
  assert.equal(unknown.ok, false);
  assert.match(String(unknown.error), /et\.khongCo/);

  for (const action of ["et.listSheets", "et.sheetInfo", "et.readRange", "et.addChart"]) {
    const broken = mockApp([], { "app.Name": "WPS Spreadsheets",
      "app.ActiveWorkbook": throwing("boom"), "app.ActiveSheet": throwing("boom") });
    assertFails(ctx, broken, action, { range: "A1:B2", type: "column" }, /boom/);
  }
});

test("moi lenh et.* deu tra {ok:false, error} khi thieu tham so bat buoc, khong throw", () => {
  const ctx = loadAddin();
  const app = makeEtApp({ sheets: [{ name: "S" }] });
  for (const command of REQUIRES_PARAM) {
    let reply;
    assert.doesNotThrow(() => { reply = ctx.AxiomCommands.execute(app, command, {}); },
      command + ": execute khong duoc throw");
    assert.equal(reply.ok, false, command + " thieu tham so bat buoc phai ok:false, duoc " + JSON.stringify(reply));
    assert.ok(String(reply.error).length > 0, command + ": phai co thong diep loi");
  }
});
