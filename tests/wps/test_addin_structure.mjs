// Spec P1-2, P1-3 (docs/research/doi-chieu-wps-ai.md): gan style cho doan, muc luc, dinh dang bang cho JS add-in WPS.
//
//   node --test tests/wps/test_addin_structure.mjs
//   node --test --test-name-pattern=^setStyle tests/wps/test_addin_structure.mjs     (moi nhom mot tien to)
//
// Hop dong (ten lenh, tham so, dang ket qua, cau bao loi) la cua catalog/live-commands.json va ban Windows
// (src/AxiomOffice/Bridge/CommandDispatcher.WriterStructure.cs: WriterSetStyle, WriterInsertToc, BuiltinStyle;
// CommandDispatcher.Writer.cs: WriterFormatTable, TargetTable) - Core dung chung moi nen tang nen ket qua phai giong.
// Khac ban Windows CHI o cho WPS 12.1.2 that khac Word (do 01/11/2026, xem dau tests/wps/addin_fake_structure.mjs):
//   - setStyle: WPS gan ten style la thi am tham thanh Normal -> phai kiem doc.Styles.Item(...) !== null TRUOC khi gan,
//     va kiem moi so doan truoc khi doi doan nao.
//   - formatTable: chu trong o mang style Heading (bang chen vao doan tieu de) -> dua ve Normal truoc tien, bao trong
//     `applied` la "bodyStyle"; mau nen hang ma WPS bo qua (doc lai khong dung mau) -> vao `skipped`, khong vao `applied`.

import test from "node:test";
import assert from "node:assert/strict";
import { makeStructureApp, writerStructureOf } from "./addin_fake_structure.mjs";
import { loadAddin, run } from "./addin_harness.mjs";

function fails(ctx, app, action, params, pattern) {
  const reply = ctx.AxiomCommands.execute(app, action, params);
  assert.equal(reply.ok, false, action + " phai bao loi: " + JSON.stringify(reply));
  assert.match(String(reply.error), pattern);
}

const BAO_CAO = [
  { text: "BAO CAO QUY BA", style: "Title" },
  { text: "I. Tinh hinh chung" },
  { text: "Noi dung tinh hinh." },
  { text: "1. Nhan su" },
  { text: "Noi dung nhan su." },
  { text: "a) Tuyen dung" },
  { text: "II. Ke hoach" },
];

// ---- setStyle: writer.setStyle ----

test("setStyle: gan style dung san cho cac doan theo so (ten khong phan biet hoa thuong); mot buoc hoan tac", () => {
  const ctx = loadAddin();
  const app = makeStructureApp({ paragraphs: BAO_CAO });
  assert.deepEqual(run(ctx, app, "writer.setStyle", { style: "heading 1", paragraphs: [2, 7] }),
    { style: "Heading 1", paragraphs: 2, texts: ["I. Tinh hinh chung", "II. Ke hoach"] });
  assert.deepEqual(run(ctx, app, "writer.setStyle", { style: "Heading 2", paragraphs: [4] }).texts, ["1. Nhan su"]);
  assert.deepEqual(writerStructureOf(app).paragraphs.map((p) => p[1]),
    ["Title", "Heading 1", "Normal", "Heading 2", "Normal", "Normal", "Heading 1"]);
  run(ctx, app, "writer.undo", { count: 1 });
  assert.equal(writerStructureOf(app).paragraphs[3][1], "Normal", "lenh setStyle sau cung la MOT buoc hoan tac");
  assert.equal(writerStructureOf(app).paragraphs[1][1], "Heading 1");
});

test("setStyle: style rieng cua tai lieu dung dung ten; khong truyen paragraphs thi lay cac doan dang chon", () => {
  const ctx = loadAddin();
  const app = makeStructureApp({ paragraphs: BAO_CAO, styles: ["Ten co quan"], selection: [5, 6] });
  assert.equal(run(ctx, app, "writer.setStyle", { style: "Ten co quan", paragraphs: [1] }).style, "Ten co quan");
  assert.deepEqual(run(ctx, app, "writer.setStyle", { style: "List Bullet" }),
    { style: "List Bullet", paragraphs: 2, texts: ["Noi dung nhan su.", "a) Tuyen dung"] });
  assert.deepEqual(writerStructureOf(app).paragraphs.map((p) => p[1]).slice(4), ["List Bullet", "List Bullet", "Normal"]);
});

test("setStyle: style khong co hoac so doan ngoai tai lieu thi bao loi va KHONG doi doan nao (WPS am tham doi thanh Normal)", () => {
  const ctx = loadAddin();
  const app = makeStructureApp({ paragraphs: BAO_CAO });
  fails(ctx, app, "writer.setStyle", { style: "Tieu de rieng", paragraphs: [1, 2] },
    /unknown style 'Tieu de rieng'; use Heading 1-9, Normal, Title, Subtitle, List Bullet, List Number, Caption or Quote/);
  fails(ctx, app, "writer.setStyle", { style: "Heading 1", paragraphs: [2, 9] },
    /paragraph 9 does not exist \(the document has 7; numbers come from writer.outline\)/);
  fails(ctx, app, "writer.setStyle", { paragraphs: [2] }, /'style' is required, e.g. 'Heading 2'/);
  assert.deepEqual(writerStructureOf(app).paragraphs.map((p) => p[1]),
    ["Title", "Normal", "Normal", "Normal", "Normal", "Normal", "Normal"]);
});

// ---- insertToc: writer.insertToc ----

test("insertToc: chua co muc luc thi chen tai con tro voi Heading 1..levels; goi lai thi cap nhat", () => {
  const ctx = loadAddin();
  const styles = ["Title", "Heading 1", "Normal", "Heading 2", "Normal", "Heading 3", "Heading 1"];
  const paragraphs = BAO_CAO.map((p, i) => ({ text: p.text, style: styles[i] }));
  const app = makeStructureApp({ paragraphs, selection: [2] });
  assert.deepEqual(run(ctx, app, "writer.insertToc", { levels: 2 }), { inserted: true, levels: 2, entries: 3 });
  const toc = writerStructureOf(app).tocs[0];
  assert.deepEqual({ at: toc.at, lower: toc.lower, entries: toc.entries },
    { at: 2, lower: 2, entries: ["I. Tinh hinh chung", "1. Nhan su", "II. Ke hoach"] });
  assert.ok(toc.updates >= 1, "muc luc moi duoc cap nhat ngay (so trang)");
  run(ctx, app, "writer.setStyle", { style: "Heading 2", paragraphs: [3] });
  assert.deepEqual(run(ctx, app, "writer.insertToc", {}), { updated: 1, entries: 4 });
  assert.equal(writerStructureOf(app).tocs.length, 1, "da co muc luc thi khong chen them");
});

test("insertToc: `before` chen muc luc truoc doan do, doan trong moi mang style Normal (khong thanh tieu de trong)", () => {
  const ctx = loadAddin();
  const paragraphs = BAO_CAO.map((p, i) => ({ ...p, style: i === 1 || i === 6 ? "Heading 1" : p.style }));
  const app = makeStructureApp({ paragraphs });
  const reply = run(ctx, app, "writer.insertToc", { before: 2 });
  assert.deepEqual(reply, { inserted: true, levels: 3, entries: 2 });
  const state = writerStructureOf(app);
  assert.deepEqual(state.paragraphs[1], ["", "Normal"]);
  assert.equal(state.paragraphs.length, 8);
  assert.equal(state.tocs[0].at, 2);
  fails(ctx, makeStructureApp({ paragraphs }), "writer.insertToc", { before: 99 }, /paragraph 99 does not exist \(the document has 7\)/);
});

// ---- formatTable: writer.formatTable ----

const BANG = { cells: [["Hang muc", "So luong", "Thanh tien"], ["Giay A4", "10", "500000"], ["But bi", "120", "360000"]] };

test("formatTable: chu trong o mang style Heading thi dua ve Normal truoc; ap kieu, font, co, can, tieu de dam + lap lai, co gian", () => {
  const ctx = loadAddin();
  const app = makeStructureApp({ paragraphs: BAO_CAO, tables: [{ ...BANG, cellStyle: "Heading 1" }] });
  const reply = run(ctx, app, "writer.formatTable", {
    table: 1, style: "Table Grid", font: "Times New Roman", size: 13, alignment: "left", headerBold: true, autoFit: "window",
  });
  assert.deepEqual(reply, { table: 1, rows: 3, cols: 3,
    applied: ["bodyStyle", "style", "font", "size", "alignment", "headerBold", "autoFit"] });
  const t = writerStructureOf(app).tables[0];
  assert.ok(t.cellStyle.every((row) => row.every((s) => s === "Normal")));
  assert.equal(t.style, "Table Grid");
  assert.deepEqual(t.font, { name: "Times New Roman", size: 13, color: -16777216 });
  assert.deepEqual([t.bold, t.headingFormat], [[true, false, false], [true, false, false]]);
  assert.equal(t.autoFit, 2);
  run(ctx, app, "writer.undo", { count: 1 });
  assert.equal(writerStructureOf(app).tables[0].cellStyle[0][0], "Heading 1", "ca lenh la MOT buoc hoan tac");
});

test("formatTable: chu trong o da la Normal thi khong co bodyStyle; mau chu theo #RRGGBB (BGR); mau nen WPS bo qua thi vao skipped", () => {
  const ctx = loadAddin();
  const app = makeStructureApp({ tables: [BANG, BANG], cursorTable: 2 });
  const reply = run(ctx, app, "writer.formatTable", { color: "#1F4E79", headerFill: "#D9E2F3" });
  assert.equal(reply.table, "at cursor");
  assert.deepEqual(reply.applied, ["color", "headerBold"]);
  assert.equal(reply.skipped.length, 1);
  assert.match(reply.skipped[0], /^headerFill: /);
  const t = writerStructureOf(app).tables[1];
  assert.equal(t.font.color, 0x1F + (0x4E << 8) + (0x79 << 16));
  assert.deepEqual(t.headingFormat, [true, false, false], "co mau tieu de thi hang dau dam + lap lai khi sang trang");
  assert.deepEqual(writerStructureOf(app).tables[0].font.color, -16777216, "bang khac khong doi");
});

test("formatTable: borders:false = bang dan trang (Title AxiomLayout); borders:true bo tem; khong chon bang thi lay bang cuoi", () => {
  const ctx = loadAddin();
  const app = makeStructureApp({ tables: [BANG, BANG] });
  const off = run(ctx, app, "writer.formatTable", { borders: false });
  assert.equal(off.table, 2);
  assert.deepEqual(off.applied, ["borders", "layout-table"]);
  assert.deepEqual([writerStructureOf(app).tables[1].borders, writerStructureOf(app).tables[1].title], [0, "AxiomLayout"]);
  run(ctx, app, "writer.formatTable", { table: 2, borders: true });
  assert.deepEqual([writerStructureOf(app).tables[1].borders, writerStructureOf(app).tables[1].title], [1, "AxiomTable"]);
});

test("formatTable: khong co bang hoac so bang sai thi bao loi ro", () => {
  const ctx = loadAddin();
  fails(ctx, makeStructureApp({ paragraphs: BAO_CAO }), "writer.formatTable", { font: "Arial" },
    /the document has no table - use writer.insertTable first/);
  fails(ctx, makeStructureApp({ tables: [BANG] }), "writer.formatTable", { table: 3, font: "Arial" },
    /'table' must be between 1 and 1, got 3/);
});
