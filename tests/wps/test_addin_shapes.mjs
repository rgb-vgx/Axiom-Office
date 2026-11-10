// Spec T2 (dung slide tren WPS, 01/11/2026): wpp.addShape, wpp.addParagraphs cho JS add-in WPS - hai lenh Required
// cua slides_from_brief (core-go/internal/tools/needs.go) con thieu, nen tool dung slide bi an tren WPS.
//
//   node --test tests/wps/test_addin_shapes.mjs
//
// Hop dong (ten lenh, tham so, dang ket qua) la cua catalog/live-commands.json va ban Windows
// (src/AxiomOffice/Bridge/CommandDispatcher.Shapes.cs: WppAddShape; CommandDispatcher.Template.cs: WppAddParagraphs) -
// Core dung chung moi nen tang nen ket qua phai giong. Khac ban Windows CHI o cho WPS 12.1.2 that khac PowerPoint (do
// 01/11/2026, xem dau tests/wps/addin_fake_shapes.mjs):
//   - Hinh moi cua WPS co bong va chu co bong: addShape va addParagraphs (box) phai tat ca hai (Shadow.Visible = 0,
//     Font.Shadow = 0) - ban PowerPoint khong can.
//   - Words(n) tra null: nhan manh **tu** chi bang Characters(start, len) cua doan (ban Windows cung vay).
//   - Adjustments (do bo goc) khong gan duoc tren WPS: khong dung, hoac boc try/catch.

import test from "node:test";
import assert from "node:assert/strict";
import { makeShapesApp, shapesOf, boundHeightOf } from "./addin_fake_shapes.mjs";
import { loadAddin, run } from "./addin_harness.mjs";

function fails(ctx, app, action, params, pattern) {
  const reply = ctx.AxiomCommands.execute(app, action, params);
  assert.equal(reply.ok, false, action + " phai bao loi: " + JSON.stringify(reply));
  assert.match(String(reply.error), pattern);
}

const last = (app) => { const s = shapesOf(app); const shapes = s[s.length - 1].shapes; return shapes[shapes.length - 1]; };

// ---- addShape ----

test("addShape: hinh chu nhat co nen phang, vien, chu giua; tat bong cua WPS; ket qua {slide, shape}", () => {
  const ctx = loadAddin();
  const app = makeShapesApp({ slides: 2 });
  const out = run(ctx, app, "wpp.addShape", { slide: 1, left: 10, top: 20, width: 200, height: 80, text: "Buoc 1",
    fill: "#1F3C5A", line: "#102030", fontSize: 16, bold: true, font: "Arial", color: "#FFFFFF" });
  const shape = shapesOf(app)[0].shapes[0];
  assert.deepEqual(out, { slide: 1, shape: shape.name });
  assert.equal(shape.autoShapeType, 1);
  assert.deepEqual([shape.left, shape.top, shape.width, shape.height], [10, 20, 200, 80]);
  assert.deepEqual([shape.fill.visible, shape.fill.type, shape.fill.rgb], [-1, 1, 0x5A3C1F], "nen PHANG (Fill.Solid) mau BGR");
  assert.deepEqual([shape.line.visible, shape.line.rgb, shape.line.weight], [-1, 0x302010, 1.25]);
  assert.equal(shape.shadow, 0, "hinh moi cua WPS co bong - phai tat");
  const p = shape.paragraphs[0];
  assert.deepEqual([p.text, p.size, p.bold, p.font, p.color, p.align, p.textShadow], ["Buoc 1", 16, -1, "Arial", 0xFFFFFF, 2, 0]);
  assert.deepEqual([shape.anchor, shape.wordWrap, shape.autoSize, shape.marginLeft, shape.marginRight], [3, -1, 0, 6, 6]);
});

test("addShape: kieu hinh, mac dinh (slide cuoi, chu #262626 co 18, giua), khong fill/line thi an nen/vien", () => {
  const ctx = loadAddin();
  const app = makeShapesApp({ slides: 3 });
  const types = {};
  for (const kind of ["rect", "rounded", "oval", "chevron", "line"]) {
    assert.equal(run(ctx, app, "wpp.addShape", { shape: kind }).slide, 3, "khong co slide = slide cuoi");
    types[kind] = last(app).autoShapeType;
  }
  assert.deepEqual(types, { rect: 1, rounded: 5, oval: 9, chevron: 52, line: 1 });
  const plain = last(app);
  assert.deepEqual([plain.left, plain.top, plain.width, plain.height], [40, 130, 200, 80]);
  assert.equal(plain.fill.visible, 0);
  assert.equal(plain.line.visible, 0);
  assert.equal(plain.shadow, 0);
  run(ctx, app, "wpp.addShape", { text: "Mot\nHai\nBa", boldFirst: true, anchor: "top" });
  const box = last(app);
  assert.deepEqual(box.paragraphs.map((p) => [p.text, p.bold, p.size, p.color, p.align]),
    [["Mot", -1, 18, 0x262626, 2], ["Hai", 0, 18, 0x262626, 2], ["Ba", 0, 18, 0x262626, 2]], "moi dong mot doan");
  assert.equal(box.anchor, 1);
});

test("addShape: vien net dut khi dash; kieu hinh la thi bao loi va KHONG ve gi", () => {
  const ctx = loadAddin();
  const app = makeShapesApp();
  run(ctx, app, "wpp.addShape", { line: "#595959", dash: true });
  assert.equal(last(app).line.dash, 4);
  fails(ctx, app, "wpp.addShape", { shape: "star" }, /'shape' must be rect, rounded, oval, chevron or line/);
  assert.equal(shapesOf(app)[0].shapes.length, 1);
  fails(ctx, makeShapesApp({ slides: 0 }), "wpp.addShape", {}, /no slides/);
});

// ---- addParagraphs ----

const MUC_TIEU = [
  { t: "Muc tieu nam 2027", b: true },
  { t: "Tang **doanh thu** 20%", u: true },
  { t: "Mo 3 chi nhanh", l: 2, u: true },
  { t: "Tuyen dung", n: "1.", i: true },
];

test("addParagraphs: khung chu tron, tung doan giu cap/dam/nghieng/so/gach dau dong; ** thanh chu dam to mau", () => {
  const ctx = loadAddin();
  const app = makeShapesApp();
  const out = run(ctx, app, "wpp.addParagraphs", { slide: 1, left: 40, top: 100, width: 640, height: 360,
    paragraphs: MUC_TIEU, color: "#333333", accent: "#1F3C5A", font: "Arial" });
  const shape = shapesOf(app)[0].shapes[0];
  assert.equal(shape.kind, "textbox", "khong box/fill = khung chu tron (AddTextbox)");
  assert.deepEqual(Object.keys(out).sort(), ["fontSize", "overflow", "shape", "slide", "textHeight"]);
  assert.deepEqual([out.slide, out.shape, out.fontSize, out.overflow], [1, shape.name, 24, false]);
  assert.equal(out.textHeight, Math.round(boundHeightOf(shape) * 10) / 10);
  assert.deepEqual(shape.paragraphs.map((p) => p.text),
    ["Muc tieu nam 2027", "Tang doanh thu 20%", "Mo 3 chi nhanh", "1. Tuyen dung"], "bo dau **, them so thu tu");
  assert.deepEqual(shape.paragraphs.map((p) => p.indentLevel), [1, 1, 2, 1]);
  assert.deepEqual(shape.paragraphs.map((p) => [p.bold, p.italic]), [[-1, 0], [0, 0], [0, 0], [0, -1]]);
  assert.deepEqual(shape.paragraphs.map((p) => p.color), [0x5A3C1F, 0x333333, 0x333333, 0x333333], "doan dam mang mau accent");
  assert.deepEqual(shape.paragraphs.map((p) => p.font), ["Arial", "Arial", "Arial", "Arial"]);
  assert.deepEqual(shape.paragraphs.map((p) => p.align), [1, 1, 1, 1]);
  assert.deepEqual(shape.paragraphs.map((p) => [p.bullet.visible, p.bullet.type, p.bullet.character]),
    [[0, 0, 0], [-1, 1, 8226], [-1, 1, 8226], [0, 0, 0]], "gach dau dong la dau cham (Type 1), khong phai danh so");
  const marks = shape.paragraphs[1].runs.filter((r) => r.bold === -1);
  assert.deepEqual(marks.map((r) => [r.start, r.length, r.color]), [[6, 9, 0x5A3C1F]], "'doanh thu' dam, mau accent");
  assert.deepEqual([shape.anchor, shape.wordWrap, shape.autoSize, shape.height], [1, -1, 0, 360]);
});

test("addParagraphs: thut le tung doan theo co chu (cap 1,2 em; gach dau dong treo 0,9 em); doan cap 1 cach tren 0,35 em", () => {
  const ctx = loadAddin();
  const app = makeShapesApp();
  run(ctx, app, "wpp.addParagraphs", { paragraphs: MUC_TIEU, width: 640, height: 360 });
  const paras = shapesOf(app)[0].shapes[0].paragraphs;
  // co 24: hang = 21.6; cap 2 + gach: (2-1)*1.2*24 + 21.6 = 50.4
  assert.deepEqual(paras.map((p) => [p.leftIndent, p.firstIndent]), [[0, -0], [21.6, -21.6], [50.4, -21.6], [0, -0]]);
  assert.deepEqual(paras.map((p) => p.spaceBefore), [0, 8.4, 0, 8.4]);
});

test("addParagraphs: chon co chu LON NHAT vua khung (giam tu fontSize toi minFontSize); khong vua thi overflow", () => {
  const ctx = loadAddin();
  const app = makeShapesApp();
  const long = Array.from({ length: 6 }, (_, i) => ({ t: "Noi dung dai so " + (i + 1) + " can xuong dong nhieu lan trong khung hep", u: true }));
  const out = run(ctx, app, "wpp.addParagraphs", { paragraphs: long, width: 300, height: 260, fontSize: 28, minFontSize: 10 });
  const shape = shapesOf(app)[0].shapes[0];
  assert.ok(out.fontSize < 28 && out.fontSize >= 10, JSON.stringify(out));
  assert.equal(out.overflow, false);
  const inner = shape.marginTop + shape.marginBottom;
  assert.ok(boundHeightOf(shape) <= 260 - inner + 1, "vua khung o co da chon");
  // co lon hon mot bac thi tran: dung mo hinh cua fake voi thut le cua co do
  const bigger = JSON.parse(JSON.stringify(shape));
  for (const p of bigger.paragraphs) { p.size = out.fontSize + 1; p.leftIndent = Math.round(0.9 * (out.fontSize + 1) * 10) / 10; p.spaceBefore = p === bigger.paragraphs[0] ? 0 : Math.round(0.35 * (out.fontSize + 1) * 10) / 10; }
  assert.ok(boundHeightOf(bigger) > 260 - inner + 1, "co lon hon mot bac khong vua - phai chon co LON NHAT");
  const tight = run(ctx, app, "wpp.addParagraphs", { paragraphs: long, width: 200, height: 40, fontSize: 20, minFontSize: 14 });
  assert.deepEqual([tight.fontSize, tight.overflow], [14, true]);
  assert.equal(last(app).height, 40, "khung giu chieu cao da cho");
});

test("addParagraphs: box = hinh co nen (bo goc mac dinh, rect), le 10/8, chu #262626 khi khong co color, tat bong; anchor middle", () => {
  const ctx = loadAddin();
  const app = makeShapesApp();
  run(ctx, app, "wpp.addParagraphs", { paragraphs: [{ t: "The 1", b: true }, { t: "Noi dung" }], box: "rounded",
    fill: "#EAF1F8", line: "#1F3C5A", anchor: "middle", width: 200, height: 150 });
  const card = last(app);
  assert.deepEqual([card.kind, card.autoShapeType], ["shape", 5]);
  assert.deepEqual([card.fill.visible, card.fill.type, card.fill.rgb], [-1, 1, 0xF8F1EA]);
  assert.deepEqual([card.line.visible, card.line.rgb, card.line.weight], [-1, 0x5A3C1F, 1]);
  assert.equal(card.shadow, 0);
  assert.deepEqual([card.marginLeft, card.marginRight, card.marginTop, card.marginBottom], [10, 10, 8, 8]);
  assert.deepEqual(card.paragraphs.map((p) => [p.color, p.textShadow, p.align]), [[0x262626, 0, 1], [0x262626, 0, 1]],
    "chu toi tren nen nhat, khong bong, can trai (hinh WPS mac dinh chu trang giua)");
  assert.equal(card.anchor, 3);
  run(ctx, app, "wpp.addParagraphs", { paragraphs: [{ t: "Ghi chu" }], box: "rect" });
  assert.deepEqual([last(app).autoShapeType, last(app).fill.visible, last(app).line.visible], [1, 0, 0]);
  run(ctx, app, "wpp.addParagraphs", { paragraphs: [{ t: "Can giua", a: "center" }], fill: "#FFFFFF" });
  assert.deepEqual([last(app).kind, last(app).autoShapeType, last(app).paragraphs[0].align], ["shape", 5, 2],
    "co fill ma khong box = the bo goc; a: center can giua doan");
});

test("addParagraphs: highlight rieng cho chu nhan manh; doan rong bi bo; khong co doan nao thi bao loi, khong ve", () => {
  const ctx = loadAddin();
  const app = makeShapesApp();
  run(ctx, app, "wpp.addParagraphs", { paragraphs: [{ t: "  " }, { t: "Loi **nhuan**" }], highlight: "#C00000" });
  const paras = last(app).paragraphs;
  assert.deepEqual(paras.map((p) => p.text), ["Loi nhuan"]);
  assert.deepEqual(paras[0].runs.filter((r) => r.bold === -1).map((r) => [r.start, r.length, r.color]), [[5, 5, 0x0000C0]]);
  fails(ctx, app, "wpp.addParagraphs", { paragraphs: [{ t: " " }] }, /'paragraphs' must be a non-empty array/);
  fails(ctx, app, "wpp.addParagraphs", {}, /'paragraphs' must be a non-empty array/);
  assert.equal(shapesOf(app)[0].shapes.length, 1);
});

// ---- checkLayout: tran chu ----
// TextRange.BoundHeight do duoc tren WPS 12.1.2 (01/11/2026) nen wpp.checkLayout bao `overflow` nhu ban Windows
// (CommandDispatcher.Checks.cs LayoutIssues: chieu cao chu > chieu cao khung + 2pt).

test("checkLayout: chu cao hon khung thi bao overflow; khung vua chu thi khong", () => {
  const ctx = loadAddin();
  const app = makeShapesApp();
  run(ctx, app, "wpp.addShape", { left: 10, top: 20, width: 200, height: 40, text: "x".repeat(100) });
  run(ctx, app, "wpp.addShape", { left: 300, top: 20, width: 200, height: 80, text: "Vua khung" });
  const [tight, roomy] = shapesOf(app)[0].shapes;
  assert.ok(boundHeightOf(tight) > tight.height + 2 && boundHeightOf(roomy) <= roomy.height, "dieu kien cua phep thu");
  const out = run(ctx, app, "wpp.checkLayout");
  const issues = out.slides[0].issues;
  assert.deepEqual(issues.map((i) => [i.type, i.shapes]), [["overflow", [tight.name]]]);
  assert.match(issues[0].detail, new RegExp("needs " + Math.round(boundHeightOf(tight)) + "pt height but the box is 40pt"));
});
