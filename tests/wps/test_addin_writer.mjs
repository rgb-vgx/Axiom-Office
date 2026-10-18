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
