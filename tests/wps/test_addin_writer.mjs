// Spec khoa vung boi den cua JS add-in WPS (writer.lockSelection / writer.unlockSelection).
//
//   node --test tests/wps/test_addin_writer.mjs
//
// Khong can WPS: `addin_fake.mjs` mo phong object model Writer (chuoi tai lieu + Range + Tables +
// UndoRecord). Day la ban WPS cua `CommandDispatcher.WriterScope.cs` ben add-in Windows; thiet ke
// giong het, xem khoi chu thich dau `commands.js`:
//
//   1. Vung khoa nho bang phan chu TRUOC va SAU no, khong bang mot Range.
//   2. Lenh writer.* ghi chay trong mot buoc hoan tac; con tro bi keo ve vung khoa truoc khi chay.
//   3. Luoi an toan SAU khi chay: phan ngoai vung doi -> hoan tac ngay + bao loi; mat mot bang nam
//      tron trong vung -> hoan tac ngay + bao loi.
//   4. Danh sach tu choi TRUOC (LOCK_REFUSED, LOCK_TABLE_REFUSED) de model biet duong lam khac.
//
// Phep kiem quan trong nhat o day la (3): mot lenh di ra ngoai vung phai bi HOAN TAC THAT, khong
// chi "bao loi". Vi vay moi phep kiem deu doc lai `writerStateOf(app)` chu khong tin ket qua tra ve.

import test from "node:test";
import assert from "node:assert/strict";
import { makeWriterApp, writerSelect, writerStateOf, callsOf } from "./addin_fake.mjs";
import { loadAddin, run, assertFails, addinCatalog } from "./addin_harness.mjs";

const LOCK = "writer.lockSelection";
const UNLOCK = "writer.unlockSelection";

// Vung khoa [0,3) "AAA" trong "AAA BBB CCC": phan truoc rong, phan sau " BBB CCC".
function lockedApp(ctx, options = {}) {
  const app = makeWriterApp({ text: "AAA BBB CCC", selection: { start: 0, end: 3 }, ...options });
  const reply = run(ctx, app, LOCK, {});
  assert.equal(reply.chars, 3, "khoa phai nhan dung vung dang boi den");
  return app;
}

test("registry: writer.lockSelection/unlockSelection co handler, agent=false (chi client ngoai goi)", () => {
  const ctx = loadAddin();
  assert.equal(ctx.AxiomCommands.hasHandler(LOCK), true);
  assert.equal(ctx.AxiomCommands.hasHandler(UNLOCK), true);
  const mine = new Map(addinCatalog(ctx).map((c) => [c.name, c]));
  for (const name of [LOCK, UNLOCK]) {
    assert.ok(mine.has(name), "add-in phai khai bao " + name);
    assert.equal(mine.get(name).agent, false, name + ": agent phai la false (agent khong tu khoa vung)");
    assert.equal(mine.get(name).kind, "wps", name + ": kind");
  }
});

test("lockSelection: khong boi den / chi boi khoang trang / con tro dung yen -> chars 0, khong khoa", () => {
  const ctx = loadAddin();
  const cases = [
    [{ start: 4, end: 4 }, "con tro dung yen"],
    [{ start: 4, end: 5 }, "mot ky tu"],
    [{ start: 4, end: 5 }, "khoang trang", "AAA   BBB"],
  ];
  for (const [selection, why, text] of cases) {
    const app = makeWriterApp({ text: text || "AAA BBB CCC", selection });
    const reply = run(ctx, app, LOCK, {});
    assert.equal(reply.chars, 0, why + ": khong duoc khoa");
    assert.ok(String(reply.reason).length > 0, why + ": phai noi ro vi sao khong khoa");
    // Chua khoa thi lenh cham ra ngoai van chay binh thuong.
    run(ctx, app, "writer.appendText", { text: "!" });
    assert.equal(writerStateOf(app).text, (text || "AAA BBB CCC") + "!");
  }
});

test("lockSelection: tra so do vung (chars/start/end/docEnd/tables) de con do duoc tren WPS that", () => {
  const ctx = loadAddin();
  const app = makeWriterApp({ text: "AAA BBB CCC", selection: { start: 4, end: 7 } });
  const reply = run(ctx, app, LOCK, {});
  assert.deepEqual(reply, { chars: 3, start: 4, end: 7, docEnd: 11, tables: 0, tablesKnown: true });
});

test("lockSelection: ban WPS thieu API thi TU CHOI khoa (thieu luoi an toan thi khong khoa)", () => {
  const ctx = loadAddin();
  const base = { text: "AAA BBB CCC", selection: { start: 0, end: 3 } };
  const cases = [
    [{ ...base, selectionPositions: false }, /Selection\.Start\/End/, "khong doc duoc Selection.Start/End"],
    [{ ...base, range: false }, /Range\(from, to\)\.Text/, "khong doc duoc doc.Range"],
  ];
  for (const [options, pattern, why] of cases) {
    const app = makeWriterApp(options);
    const reply = run(ctx, app, LOCK, {});
    assert.equal(reply.chars, 0, why + ": khong duoc khoa");
    assert.match(String(reply.reason), pattern, why + ": phai noi ro API nao thieu");
    // Chua khoa thi lenh ghi ca tai lieu van chay - dung nhu khi nguoi dung khong boi den.
    run(ctx, app, "writer.appendText", { text: "!" });
    assert.equal(writerStateOf(app).text, "AAA BBB CCC!");
  }
});

test("khi dang khoa: lenh doc chay binh thuong, lenh ghi ra ngoai vung bi tu choi TRUOC", () => {
  const ctx = loadAddin();
  const app = lockedApp(ctx);
  // Lenh doc: khong doi gi, khong bi cham.
  assert.equal(run(ctx, app, "writer.getText", {}).text, "AAA BBB CCC");
  assert.equal(run(ctx, app, "writer.selection", {}).text, "AAA");
  // Lenh luon cham ra ngoai: tu choi TRUOC khi goi API nao (khong duoc de lai dau vet trong tai lieu).
  // Danh sach chan co ca lenh add-in CHUA co handler (newDocument/open/closeAll/restyle/insertBlocks):
  // chung khong the bi tu choi vi `execute` tra "unknown action" truoc - phep kiem duoi day doi dung
  // mot trong hai, de khong lenh nao trong danh sach chan lai di lot vao tai lieu khi dang khoa.
  const refused = ["writer.appendText", "writer.insertPageBreak", "writer.newDocument",
    "writer.open", "writer.closeAll", "writer.restyle", "writer.insertBlocks"];
  for (const action of refused) {
    const before = callsOf(app).length;
    if (ctx.AxiomCommands.hasHandler(action)) {
      assertFails(ctx, app, action, { text: "X", path: "/tmp/x.docx" }, /Refused: the user selected part/);
      assert.equal(callsOf(app).length, before, action + ": bi tu choi TRUOC, khong duoc goi API nao");
    } else {
      assertFails(ctx, app, action, { text: "X", path: "/tmp/x.docx" }, /unknown action/);
    }
  }
  assert.equal(writerStateOf(app).text, "AAA BBB CCC");
  // writer.undo la lenh nguoi dung goi: chay duoc khi dang khoa.
  run(ctx, app, "writer.undo", { count: 1 });
});

test("khi dang khoa: typeText thay DUNG vung khoa, phan ngoai khong doi", () => {
  const ctx = loadAddin();
  const app = lockedApp(ctx);
  run(ctx, app, "writer.typeText", { text: "XXX" });
  assert.equal(writerStateOf(app).text, "XXX BBB CCC");
});

test("khi dang khoa: con tro o ngoai vung bi KEO VE vung khoa truoc khi chay", () => {
  const ctx = loadAddin();
  const app = lockedApp(ctx);
  writerSelect(app, 8, 11);          // nguoi dung bam sang cuoi tai lieu
  run(ctx, app, "writer.typeText", { text: "XXX" });
  assert.equal(writerStateOf(app).text, "XXX BBB CCC", "lenh phai chay trong vung khoa, khong phai cho con tro");
  assert.ok(callsOf(app).includes("Selection.SetRange(0,3)"), "phai keo con tro bang SetRange");
});

test("LUOI AN TOAN: ban WPS khong keo duoc con tro -> lenh ra ngoai bi HOAN TAC that", () => {
  const ctx = loadAddin();
  // selectionLockable:false = ban WPS khong co SetRange va Start/End chi doc (khong keo con tro duoc).
  const app = lockedApp(ctx, { selectionLockable: false });
  writerSelect(app, 8, 11);          // nguoi dung bam sang cuoi tai lieu sau khi khoa
  assertFails(ctx, app, "writer.typeText", { text: "XXX" },
    /writer\.typeText changed text outside the selection, so it was undone/);
  // Quan trong: chu phai VE NGUYEN TRANG, khong phai chi bao loi.
  assert.equal(writerStateOf(app).text, "AAA BBB CCC", "lenh ra ngoai vung phai bi hoan tac that");
  assert.ok(callsOf(app).includes("doc.Undo(1)"), "phai goi doc.Undo(1)");
});

test("LUOI AN TOAN: lenh lam mat mot bang nam tron trong vung bi HOAN TAC that", () => {
  const ctx = loadAddin();
  // Bang chiem [4,10); vung khoa [4,10) chua TRON bang do.
  const app = makeWriterApp({ text: "AAA [BANG] CCC", tables: [{ start: 4, end: 10 }], selection: { start: 4, end: 10 } });
  const reply = run(ctx, app, LOCK, {});
  assert.equal(reply.tables, 1, "phai dem duoc 1 bang nam tron trong vung");
  // typeText nam trong LOCK_TABLE_REFUSED: bi tu choi TRUOC, tai lieu khong doi.
  assertFails(ctx, app, "writer.typeText", { text: "XXX" }, /Refused: the selection contains a whole table/);
  assert.deepEqual(writerStateOf(app).tables, [{ start: 4, end: 10 }], "bang phai con nguyen");
  // Bang nam MOT PHAN trong vung (con tro trong o) thi khong bi coi la bang bi de: sua chu van duoc.
  // "AAA [BANG] CCC": bang [4,10); vung khoa [6,8) nam trong long bang.
  const partial = makeWriterApp({ text: "AAA [BANG] CCC", tables: [{ start: 4, end: 10 }], selection: { start: 6, end: 8 } });
  assert.equal(run(ctx, partial, LOCK, {}).tables, 0, "bang nam mot phan khong tinh la bang trong vung");
  run(ctx, partial, "writer.typeText", { text: "XXX" });
  assert.equal(writerStateOf(partial).text, "AAA [BXXXG] CCC");
  assert.deepEqual(writerStateOf(partial).tables, [{ start: 4, end: 11 }], "bang song, chi dich vi tri theo");
});

test("replaceAll khi dang khoa: chi tim TRONG vung khoa, khong quet ca tai lieu", () => {
  const ctx = loadAddin();
  const app = lockedApp(ctx);
  // "CCC" nam ngoai vung [0,3): khong duoc thay.
  const outside = run(ctx, app, "writer.replaceAll", { find: "CCC", replace: "ZZZ" });
  assert.equal(outside.replaced, false);
  assert.equal(writerStateOf(app).text, "AAA BBB CCC");
  // "AAA" nam trong vung: thay duoc, va van chi trong vung.
  const inside = run(ctx, app, "writer.replaceAll", { find: "AAA", replace: "YYY" });
  assert.equal(inside.replaced, true);
  assert.equal(inside.count, 1);
  assert.equal(writerStateOf(app).text, "YYY BBB CCC");
});

test("unlockSelection: mo khoa roi thi lenh ghi ca tai lieu chay lai binh thuong", () => {
  const ctx = loadAddin();
  const app = lockedApp(ctx);
  assertFails(ctx, app, "writer.appendText", { text: "!" }, /Refused: the user selected part/);
  assert.deepEqual(run(ctx, app, UNLOCK, {}), { locked: false });
  run(ctx, app, "writer.appendText", { text: "!" });
  assert.equal(writerStateOf(app).text, "AAA BBB CCC!");
});

test("khoa thuoc ve CA ADD-IN: mo khoa la het, khong dinh gi tai lieu khac", () => {
  const ctx = loadAddin();
  const app = lockedApp(ctx);
  run(ctx, app, UNLOCK, {});
  const other = makeWriterApp({ text: "ZZZ", selection: { start: 0, end: 3 } });
  run(ctx, other, "writer.appendText", { text: "!" });
  assert.equal(writerStateOf(other).text, "ZZZ!");
});

test("khoa lai lan hai: vung khoa moi thay vung cu, khong cong don", () => {
  const ctx = loadAddin();
  // selectionLockable:false de con tro KHONG bi keo ve - do la cach duy nhat thay duoc rang vung cu
  // da het duoc bao ve (ban WPS keo duoc con tro thi moi lenh deu bi keo ve vung khoa, xem test tren).
  const app = lockedApp(ctx, { selectionLockable: false });
  writerSelect(app, 4, 7);           // nguoi dung boi den doan khac
  const reply = run(ctx, app, LOCK, {});
  assert.equal(reply.chars, 3);
  assert.equal(reply.start, 4);
  // Vung cu [0,3) gio nam NGOAI vung khoa moi: sua vao do phai bi hoan tac.
  writerSelect(app, 0, 3);
  assertFails(ctx, app, "writer.typeText", { text: "XXX" }, /changed text outside the selection, so it was undone/);
  assert.equal(writerStateOf(app).text, "AAA BBB CCC");
  // Con trong vung moi thi chay duoc.
  writerSelect(app, 4, 7);
  run(ctx, app, "writer.typeText", { text: "XXX" });
  assert.equal(writerStateOf(app).text, "AAA XXX CCC");
});

test("moi lenh writer.* deu chay duoc khi KHONG khoa (khoa khong lam hong duong binh thuong)", () => {
  const ctx = loadAddin();
  const app = makeWriterApp({ text: "AAA BBB CCC", selection: { start: 0, end: 3 } });
  run(ctx, app, "writer.typeText", { text: "XXX" });
  run(ctx, app, "writer.heading", { level: 1, text: "Tieu de" });
  run(ctx, app, "writer.replaceAll", { find: "BBB", replace: "YYY" });
  const text = writerStateOf(app).text;
  assert.ok(text.indexOf("YYY") >= 0, "replaceAll phai quet ca tai lieu khi khong khoa: " + text);
});

// ---- writer.checkTables: ban WPS cua checks.writer_check_tables (LibreOffice) ----
//
// Cung mot lenh trong cung mot hop dong, nen hai ban phai ra CUNG ket qua tren cung mot tai lieu:
// cung nguong, cung cach tinh trung vi, cung cau `detail`, cung thu tu truong. Cac phep kiem duoi
// day ghim tung quy tac cua src/AxiomOffice.LibreOffice/python/pythonpath/axiom/checks.py.
//
// `tables[].cells` cua fake la luoi o theo hang roi cot; fake tu tra kem "\r\x07" o cuoi moi o y nhu
// WPS that, nen day cung la phep do cho tableCellText() (boc ky tu ket o truoc khi do dai).

const CHECK = "writer.checkTables";

function checked(ctx, app) {
  return run(ctx, app, CHECK, {});
}

test("checkTables: doc dung luoi (hang/cot/o trong/tieu de theo HANG), va la lenh CHI DOC", () => {
  const ctx = loadAddin();
  const app = makeWriterApp({
    text: "AAA BBB CCC",
    selection: { start: 0, end: 3 },
    tables: [{ start: 4, end: 10, cells: [["Ho ten", "Tuoi"], ["An", "30"]] }],
  });
  const reply = checked(ctx, app);
  assert.deepEqual(reply, {
    tableCount: 1,
    tables: [{ index: 1, rows: 2, cols: 2, emptyCells: 0, header: ["Ho ten", "Tuoi"], layout: false }],
    issueCount: 0,
    issues: [],
  });
  // Chi doc: khong mo UndoRecord, khong doi tai lieu.
  assert.ok(!callsOf(app).some((entry) => entry.indexOf("UndoRecord") >= 0), "checkTables khong duoc mo UndoRecord");
  assert.deepEqual(writerStateOf(app), {
    text: "AAA BBB CCC",
    tables: [{ start: 4, end: 10 }],
    selection: { start: 0, end: 3 },
    style: "",
  });
});

test("checkTables: o chi co ky tu ket o cua Word van tinh la o TRONG", () => {
  const ctx = loadAddin();
  // O trong that tra ve "\r\x07" (ky tu ket o), khong phai chuoi rong - do dai phai do SAU khi boc.
  const app = makeWriterApp({ tables: [{ start: 0, end: 0, cells: [["A", ""], ["", "  "]] }] });
  const reply = checked(ctx, app);
  assert.equal(reply.tables[0].emptyCells, 3, "ba o trong (mot o chi co khoang trang)");
  assert.equal(reply.issueCount, 1);
  assert.equal(reply.issues[0].type, "empty-cells");
  assert.equal(reply.issues[0].table, 1);
  assert.equal(reply.issues[0].detail, "table 1 has 3 empty cell(s)", "cau detail phai GIONG ban LibreOffice");
});

test("checkTables: o tieu de dai bat thuong -> long-header-cell, nguong y nhu LibreOffice", () => {
  const ctx = loadAddin();
  const cases = [
    // [tieu de, co bao khong, vi sao]
    [["Muc", "x".repeat(41)], true, "41 ky tu, cac o khac ngan -> bat thuong"],
    [["Muc", "x".repeat(40)], false, "dung bang nguong LONG_HEADER_CHARS=40 thi khong bao (> chu khong phai >=)"],
    [["x".repeat(50), "y".repeat(50)], false, "dai nhung CA HAI deu dai -> khong phai chu lot vao o"],
    [["x".repeat(41), "y".repeat(20)], false, "3 x 20 = 60 > 41 -> ti le binh thuong"],
  ];
  for (const [header, expected, why] of cases) {
    const app = makeWriterApp({ tables: [{ start: 0, end: 0, cells: [header.slice(), ["a", "b"]] }] });
    const reply = checked(ctx, app);
    const hits = reply.issues.filter((i) => i.type === "long-header-cell");
    assert.equal(hits.length > 0, expected, why + ": " + JSON.stringify(reply.issues));
    if (expected) {
      assert.equal(hits[0].table, 1);
      assert.ok(hits[0].detail.indexOf("with 41 characters") >= 0, "detail phai noi ro so ky tu: " + hits[0].detail);
      assert.ok(hits[0].detail.indexOf("'" + "x".repeat(40) + "'") >= 0, "detail phai cat o 40 ky tu: " + hits[0].detail);
    }
  }
});

test("checkTables: bang MOT cot khong bao long-header-cell (khong co nhom nao de so)", () => {
  const ctx = loadAddin();
  const app = makeWriterApp({ tables: [{ start: 0, end: 0, cells: [["x".repeat(80)], ["a"]] }] });
  const reply = checked(ctx, app);
  assert.equal(reply.tables[0].cols, 1);
  assert.equal(reply.issueCount, 0, "mot cot thi khong co ti le nao de noi o do dai BAT THUONG");
});

test("checkTables: bang dan trang duoc mien 'empty-cells' nhung VAN bao 'long-header-cell'", () => {
  const ctx = loadAddin();
  const app = makeWriterApp({
    tables: [{
      start: 0, end: 0, title: "AxiomLayout",
      cells: [["Muc", "x".repeat(45)], ["", ""]],
    }],
  });
  const reply = checked(ctx, app);
  assert.equal(reply.tables[0].layout, true, "ten bang bat dau bang AxiomLayout -> bang dan trang");
  assert.equal(reply.tables[0].emptyCells, 2, "so o trong VAN de trong bao cao de agent thay");
  assert.deepEqual(reply.issues.map((i) => i.type), ["long-header-cell"],
    "o trong la CO Y voi bang dan trang, con chu lot vao o tieu de thi van la loi that");
});

test("checkTables: bang khong khai luoi -> coi la MOT o, doc chu trong Range, khong nem", () => {
  const ctx = loadAddin();
  const app = makeWriterApp({ text: "AAA [BANG] CCC", tables: [{ start: 4, end: 10 }] });
  const reply = checked(ctx, app);
  assert.equal(reply.tables[0].rows, 1);
  assert.equal(reply.tables[0].cols, 1);
  assert.deepEqual(reply.tables[0].header, ["[BANG]"]);
  assert.equal(reply.issueCount, 0);
});

test("checkTables: o bi gop (hang thieu cot) bi bo qua, khong lam hong ca bao cao", () => {
  const ctx = loadAddin();
  const app = makeWriterApp({ tables: [{ start: 0, end: 0, cells: [["A", "B"], ["C"]] }] });
  const reply = checked(ctx, app);
  assert.equal(reply.tables[0].rows, 2);
  assert.equal(reply.tables[0].cols, 2);
  assert.deepEqual(reply.tables[0].header, ["A", "B"], "tieu de van la hang dau, theo (hang, cot)");
  assert.equal(reply.tables[0].emptyCells, 0, "o khong ton tai bi bo qua, khong tinh la o trong");
});

test("checkTables: nhieu bang -> bao cao theo tung bang, chi so 1-based", () => {
  const ctx = loadAddin();
  const app = makeWriterApp({
    text: "x".repeat(30),
    tables: [
      { start: 0, end: 4, cells: [["A", "B"]] },
      { start: 6, end: 10, cells: [["C", ""]] },
    ],
  });
  const reply = checked(ctx, app);
  assert.equal(reply.tableCount, 2);
  assert.deepEqual(reply.tables.map((t) => t.index), [1, 2]);
  assert.deepEqual(reply.issues.map((i) => i.table), [2], "chi bang 2 co o trong");
});


test("writer.save / writer.saveAs / writer.exportPdf: ghi that va tra dung duong dan", () => {
  const ctx = loadAddin();
  const app = makeWriterApp({ text: "Xin chao" });
  const saved = run(ctx, app, "writer.save", {});
  assert.equal(saved.saved, true);
  assert.equal(saved.fullName, "/tmp/blank.docx");
  assert.ok(callsOf(app).includes("doc.Save()"), "phai goi Save() cua WPS");

  const as = run(ctx, app, "writer.saveAs", { path: "/tmp/moi.docx" });
  assert.equal(as.saved, true);
  assert.equal(as.fullName, "/tmp/moi.docx");
  assert.equal(app.ActiveDocument.Name, "moi.docx");
  assertFails(ctx, app, "writer.saveAs", {}, /'path' is required/);

  const out = run(ctx, app, "writer.exportPdf", { path: "/tmp/writer-ra.pdf" });
  assert.equal(out.exported, "/tmp/writer-ra.pdf");
  assert.equal(app.FileSystem.existsSync("/tmp/writer-ra.pdf"), true, "phai ghi file that");
  const pdf = app.FileSystem.ReadFile("/tmp/writer-ra.pdf");
  assert.equal(pdf.slice(0, 5), "%PDF-", "phai la PDF that");
  assert.ok(pdf.indexOf("%%EOF") > 0, "PDF phai co %%EOF");
  assertFails(ctx, app, "writer.exportPdf", {}, /'path' is required/);
});

test("writer.exportPdf: thieu app.FileSystem thi bao loi, khong duoc bao 'da xuat' khi khong co file", () => {
  const ctx = loadAddin();
  const app = makeWriterApp({ text: "Xin chao", fileSystem: false });
  assertFails(ctx, app, "writer.exportPdf", { path: "/tmp/writer-ra.pdf" }, /FileSystem/);
});
