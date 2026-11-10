// Spec T1 (Smart Create tren WPS, 01/11/2026): writer.writeContent, writer.insertLayoutTable, writer.insertCaption cho
// JS add-in WPS. writeContent la lenh agent dung de viet ca bai mot luot (Smart Create); ca ba la lenh can cho
// doc_from_brief (van ban hanh chinh) - con thieu writer.checkLayout (de sau).
//
//   node --test tests/wps/test_addin_compose.mjs
//
// Hop dong (ten lenh, tham so, dang ket qua, cau bao loi) la cua catalog/live-commands.json va ban Windows
// (src/AxiomOffice/Bridge/CommandDispatcher.WriterLayout.cs: WriterWriteContent, ParamBlocks, BlockKind, BlockLines,
// AppendList, AppendPageBreak; CommandDispatcher.WriterTemplate.cs: WriterInsertLayoutTable, ParseLayoutCell/Line;
// CommandDispatcher.Writer.cs: WriterInsertCaption, MarkLayoutTable, RejectFormulaCells). Khac ban Windows CHI o cho
// WPS 12.1.2 that khac Word (do 01/11/2026, xem dau tests/wps/addin_fake_compose.mjs):
//   - Hai khoi bang lien nhau: WPS GOP bang moi vao bang ngay truoc -> writeContent phai chen mot doan ngan (doan
//     trong, Normal) giua hai bang de chung la hai bang rieng.
//   - insertCaption chi chen o CUOI bai tren WPS (ban Windows con 'before' = truoc doan/bang co san): 'before' > 0
//     thi bao loi ro rang, khong ghi gi.

import test from "node:test";
import assert from "node:assert/strict";
import { makeComposeApp, composeOf } from "./addin_fake_compose.mjs";
import { loadAddin, run } from "./addin_harness.mjs";

function fails(ctx, app, action, params, pattern) {
  const reply = ctx.AxiomCommands.execute(app, action, params);
  assert.equal(reply.ok, false, action + " phai bao loi: " + JSON.stringify(reply));
  assert.match(String(reply.error), pattern);
}

// Doan thuong -> [chu, style, can le, list]; bang -> ["<bang>", so hang, so cot]
function view(app) {
  return composeOf(app).items.map((it) => (it.type === "p" ? [it.text, it.style, it.align, it.list]
    : ["<bang>", it.cells.length, it.cols]));
}
const tables = (app) => composeOf(app).items.filter((it) => it.type === "t");
const cellLines = (table, r, c) => table.cells[r - 1][c - 1].map((p) => p.text);

// ---- writeContent ----

test("writeContent: tieu de, doan van (can deu), gach dau dong, danh so - noi tiep o CUOI bai; dem theo loai khoi", () => {
  const ctx = loadAddin();
  const app = makeComposeApp({ paragraphs: [{ text: "Da co" }], cursor: 0 });
  const out = run(ctx, app, "writer.writeContent", { blocks: [
    { heading: "I. Muc tieu", level: 1 },
    { text: "Doan mot.\nDoan hai." },
    { bullets: ["Tang doanh thu", "Giam chi phi"] },
    { numbers: "Buoc mot\nBuoc hai" },
    { text: "Sau danh sach.", alignment: "center" },
  ] });
  assert.deepEqual(out, { heading: 1, text: 2, bullets: 1, numbers: 1, blocks: 5, paragraphs: 9 });
  assert.deepEqual(view(app), [
    ["Da co", "Normal", 0, 0],
    ["I. Muc tieu", "Heading 1", 0, 0],
    ["Doan mot.", "Normal", 3, 0],
    ["Doan hai.", "Normal", 3, 0],
    ["Tang doanh thu", "Normal", 0, 2],
    ["Giam chi phi", "Normal", 0, 2],
    ["Buoc mot", "Normal", 0, 3],
    ["Buoc hai", "Normal", 0, 3],
    ["Sau danh sach.", "Normal", 1, 0],
  ], "danh sach that (ListType 2/3); doan sau danh sach khong con trong danh sach");
  assert.deepEqual(composeOf(app).undo, ["start:AI: writer.writeContent", "end"], "ca lenh la MOT buoc hoan tac");
});

test("writeContent: 'alignment' chung cho khoi text; bold/italic/size/font cua tung khoi text", () => {
  const ctx = loadAddin();
  const app = makeComposeApp();
  run(ctx, app, "writer.writeContent", { alignment: "left", blocks: [
    { text: "Thuong" },
    { text: "Nhan manh", bold: true, italic: true, size: 14, font: "Arial" },
  ] });
  const items = composeOf(app).items;
  assert.deepEqual(items.map((p) => [p.text, p.align]), [["Thuong", 0], ["Nhan manh", 0]]);
  assert.deepEqual([items[1].font.bold, items[1].font.italic, items[1].font.size, items[1].font.name], [-1, -1, 14, "Arial"]);
  assert.deepEqual([items[0].font.bold, items[0].font.size, items[0].font.name], [0, 13, "Times New Roman"]);
});

test("writeContent: bang co chu thich 'Bảng N.' (truong SEQ, style Caption, dinh voi bang); dem rieng 'Hinh'", () => {
  const ctx = loadAddin();
  const app = makeComposeApp();
  const out = run(ctx, app, "writer.writeContent", { blocks: [
    { text: "Ket qua:" },
    { table: [["Quy", "Doanh thu"], ["I", "120"], ["II", "135"]], caption: "Doanh thu theo quy" },
    { table: [["A", "B"]], caption: "So do to chuc", label: "Hinh" },
    { table: [["C"]], caption: "Chi phi" },
  ] });
  assert.equal(out.table, 3);
  assert.deepEqual(view(app).filter((v) => v[1] === "Caption" || v[0] === "<bang>").map((v) => v.slice(0, 2)), [
    ["Bảng 1. Doanh thu theo quy", "Caption"], ["<bang>", 3],
    ["Hinh 1. So do to chuc", "Caption"], ["<bang>", 1],
    ["Bảng 2. Chi phi", "Caption"], ["<bang>", 1],
  ]);
  assert.deepEqual(composeOf(app).fields, ["Bảng", "Hinh", "Bảng"]);
  assert.ok(composeOf(app).items.filter((p) => p.style === "Caption").every((p) => p.keepWithNext === -1));
  assert.deepEqual(cellLines(tables(app)[0], 3, 2), ["135"]);
});

test("writeContent: chu thich ngay sau danh sach/doan van - KHONG de doan trong thua o giua", () => {
  const ctx = loadAddin();
  const app = makeComposeApp();
  run(ctx, app, "writer.writeContent", { blocks: [{ bullets: ["Mot", "Hai"] }, { table: [["x"]], caption: "Sau danh sach" },
    { text: "Doan" }, { table: [["y"]], caption: "Sau doan" }] });
  const texts = view(app).map((v) => v[0]);
  assert.deepEqual(texts.slice(0, 3), ["Mot", "Hai", "Bảng 1. Sau danh sach"]);
  const i = texts.indexOf("Bảng 2. Sau doan");
  assert.equal(texts[i - 1], "Doan");
});

test("writeContent: hai khoi bang lien nhau van la HAI bang (WPS gop bang ke nhau) - co doan ngan o giua", () => {
  const ctx = loadAddin();
  const app = makeComposeApp();
  run(ctx, app, "writer.writeContent", { blocks: [{ table: [["A1", "A2"]] }, { table: [["B1", "B2"], ["B3", "B4"]], style: "Table Grid" }] });
  const list = tables(app);
  assert.equal(list.length, 2);
  assert.deepEqual([list[0].cells.length, list[1].cells.length], [1, 2]);
  assert.deepEqual(cellLines(list[0], 1, 1), ["A1"]);
  assert.deepEqual(cellLines(list[1], 2, 2), ["B4"]);
  assert.equal(list[1].style, "Table Grid");
});

test("writeContent: ngat trang tren doan rieng; doan cuoi sau ngat la doan trong de viet tiep", () => {
  const ctx = loadAddin();
  const app = makeComposeApp();
  run(ctx, app, "writer.writeContent", { blocks: [{ text: "Trang mot" }, { pageBreak: true }, { heading: "Phu luc", level: 2 }] });
  const texts = view(app).map((v) => [v[0], v[1]]);
  assert.deepEqual(texts.slice(0, 2), [["Trang mot", "Normal"], ["\f", "Normal"]], "ngat trang khong dinh vao doan chu truoc");
  assert.deepEqual(texts[2], ["Phu luc", "Heading 2"]);
  run(ctx, app, "writer.writeContent", { blocks: [{ pageBreak: false }, { text: "Tiep" }] });
  assert.equal(view(app).filter((v) => v[0].indexOf("\f") >= 0).length, 1, "pageBreak: false khong ngat");
});

test("writeContent: soat HET khoi truoc khi ghi - khoi sai thi bao loi dung so khoi va KHONG ghi gi", () => {
  const ctx = loadAddin();
  const app = makeComposeApp({ paragraphs: [{ text: "Goc" }] });
  const cases = [
    [{}, /'blocks' must be an array of blocks/],
    [{ blocks: [{ text: "ok" }, { title: "x" }] }, /block 2 has none of heading, text, bullets, numbers, table, pageBreak \(keys: title\)/],
    [{ blocks: [{ heading: "  " }] }, /block 1: 'heading' is empty/],
    [{ blocks: [{ text: "ok" }, { text: "ok" }, { bullets: [] }] }, /block 3: 'bullets' must be a non-empty array of items/],
    [{ blocks: [{ table: "a,b" }] }, /block 1: 'table' must be a 2D array of rows/],
    [{ blocks: [{ table: [["Tong", "=SUM(B1:B2)"]] }] }, /do not calculate formulas/],
    [{ blocks: ["chu tron"] }, /block 1 is not an object/],
  ];
  for (const [params, pattern] of cases) {
    fails(ctx, app, "writer.writeContent", params, pattern);
  }
  assert.deepEqual(view(app), [["Goc", "Normal", 0, 0]]);
});

// ---- insertLayoutTable ----

const HEADER = [
  [{ lines: ["BO GIAO DUC VA DAO TAO", { t: "Truong THPT A", bold: true, upper: true, underline: true }] },
    { lines: [{ t: "CONG HOA XA HOI CHU NGHIA VIET NAM", bold: true, size: 12 }, { t: "Doc lap - Tu do - Hanh phuc", bold: true }] }],
  ["So: 12/TTr-THPT", { lines: [{ t: "Ha Noi, ngay 20 thang 10 nam 2026", italic: true }], align: "right" }],
];

test("insertLayoutTable: bang KHONG vien o cuoi bai, tem AxiomLayout, do rong %, tung dong trong o co dinh dang rieng", () => {
  const ctx = loadAddin();
  const app = makeComposeApp({ paragraphs: [{ text: "Mo dau" }] });
  const out = run(ctx, app, "writer.insertLayoutTable", { rows: HEADER, widths: [40, 60] });
  assert.deepEqual(out, { table: 1, rows: 2, cols: 2, layout: true });
  const [table] = tables(app);
  assert.deepEqual(view(app).map((v) => v[0]), ["Mo dau", "<bang>", ""], "bang nam sau doan co san, sau bang la doan trong");
  assert.deepEqual([table.borders, table.title, table.widthType, table.width], [0, "AxiomLayout", 2, 100]);
  assert.deepEqual(table.columns.map((c) => [c.widthType, Math.round(c.width)]), [[2, 40], [2, 60]]);
  assert.deepEqual(cellLines(table, 1, 1), ["BO GIAO DUC VA DAO TAO", "TRUONG THPT A"], "upper: viet hoa");
  const c11 = table.cells[0][0], c12 = table.cells[0][1], c22 = table.cells[1][1];
  assert.deepEqual(c11.map((p) => [p.align, p.font.bold, p.font.underline, p.font.size, p.font.name]),
    [[1, 0, 0, 13, "Times New Roman"], [1, -1, 1, 13, "Times New Roman"]], "mac dinh can giua, co 13, Times New Roman");
  assert.deepEqual(c12.map((p) => [p.font.size, p.font.bold]), [[12, -1], [13, -1]]);
  assert.deepEqual(c22.map((p) => [p.text, p.align, p.font.italic]), [["Ha Noi, ngay 20 thang 10 nam 2026", 2, -1]]);
  assert.deepEqual(cellLines(table, 2, 1), ["So: 12/TTr-THPT"]);
  assert.ok(table.cells.flat(2).every((p) => p.spaceBefore === 0 && p.spaceAfter === 0 && p.lineRule === 0), "gian dong don, khong cach doan");
  assert.deepEqual(composeOf(app).undo, ["start:AI: writer.insertLayoutTable", "end"]);
});

test("insertLayoutTable: o la chuoi co '\\n' thanh nhieu dong; font/size chung; viet tiep nam DUOI bang; so bang dung thu tu", () => {
  const ctx = loadAddin();
  const app = makeComposeApp();
  run(ctx, app, "writer.writeContent", { blocks: [{ table: [["x"]] }, { text: "Giua" }] });
  const out = run(ctx, app, "writer.insertLayoutTable", { rows: [["Noi nhan:\n- Nhu tren;\n- Luu: VT.", "HIEU TRUONG"]], font: "Arial", size: 11 });
  assert.equal(out.table, 2);
  const table = tables(app)[1];
  assert.deepEqual(cellLines(table, 1, 1), ["Noi nhan:", "- Nhu tren;", "- Luu: VT."]);
  assert.ok(table.cells.flat(2).every((p) => p.font.name === "Arial" && p.font.size === 11));
  assert.deepEqual(table.columns.map((c) => c.widthType), [0, 0], "khong widths thi de WPS chia deu");
  run(ctx, app, "writer.writeContent", { blocks: [{ text: "Duoi bang" }] });
  assert.deepEqual(view(app).slice(-2).map((v) => v[0]), ["<bang>", "Duoi bang"]);
});

test("insertLayoutTable: rows/widths sai thi bao loi, KHONG chen bang", () => {
  const ctx = loadAddin();
  const app = makeComposeApp();
  fails(ctx, app, "writer.insertLayoutTable", {}, /'rows' must be a 2D array of cells/);
  fails(ctx, app, "writer.insertLayoutTable", { rows: "a" }, /'rows' must be a 2D array of cells/);
  fails(ctx, app, "writer.insertLayoutTable", { rows: ["a", "b"] }, /row 1 must be an array of cells/);
  fails(ctx, app, "writer.insertLayoutTable", { rows: [] }, /'rows' is empty/);
  fails(ctx, app, "writer.insertLayoutTable", { rows: [["a", "b"]], widths: [100] }, /'widths' needs 2 positive percentages, got \[100\]/);
  assert.equal(tables(app).length, 0);
});

// ---- insertCaption ----

test("insertCaption: chu thich o cuoi bai 'Nhan N. chu' (SEQ), doan Normal sau no; 'before' chua ho tro tren WPS", () => {
  const ctx = loadAddin();
  const app = makeComposeApp({ paragraphs: [{ text: "Noi dung" }] });
  const out = run(ctx, app, "writer.insertCaption", { text: "Co cau to chuc", label: "Hinh" });
  assert.equal(out.number, "1");
  assert.deepEqual(view(app), [["Noi dung", "Normal", 0, 0], ["Hinh 1. Co cau to chuc", "Caption", 0, 0], ["", "Normal", 0, 0]]);
  assert.equal(run(ctx, app, "writer.insertCaption", { text: "Thu hai" }).number, "1", "nhan mac dinh 'Bang', dem rieng");
  fails(ctx, app, "writer.insertCaption", { text: "x", before: 2 }, /'before' is not supported in WPS/);
  fails(ctx, app, "writer.insertCaption", {}, /'text' is required/);
});
