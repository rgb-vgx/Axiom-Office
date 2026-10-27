// Spec add-in WPS cho 15 lenh wpp.* (Presentation) — T8 trong docs/architecture/wps-bridge.md muc 5.
// Chay offline: nap add-in trong vm voi object model WPS gia (addin_fake.mjs), doi chieu metadata
// voi registry LibreOffice (lo_catalog.py qua addin_harness.mjs).
//
// Hop dong (cung ten tham so / cung ket qua voi ban LibreOffice —
// src/AxiomOffice.LibreOffice/python/pythonpath/axiom/impress.py va checks.py):
//   wpp.newPresentation {}                  -> {name, fullName, saved, opened}
//   wpp.open {path}                         -> {name, fullName, saved, opened}
//   wpp.listSlides {}                       -> {presentation, slideCount, slides:[{index, layout, shapeTexts}]}
//   wpp.addSlide {layout?}                  -> {slide, layout}
//   wpp.addText {text, slide?, left?, top?, width?, height?, fontSize?, bold?, color?, align?}
//                                           -> {slide, shape}
//   wpp.addTextBox {text, slide?, left?, top?, width?, height?}
//                                           -> {slide, shape}
//   wpp.addImage {path, slide?, left?, top?, width?, height?}
//                                           -> {slide, shape}
//   wpp.addTable {rows?, cols?, values?, slide?, left?, top?, width?, height?}
//                                           -> {slide, rows, cols, filled}
//   wpp.setNotes {text, slide?}              -> {slide, notes}
//   wpp.deleteSlide {slide?}                 -> {deleted, slideCount}
//   wpp.exportPdf {path}                     -> {exported}
//   wpp.save {} / wpp.saveAs {path}          -> {saved, fullName}  (bai chua luu lan nao -> "never been saved")
//   wpp.closeAll {}                          -> {closed, rescued}
//   wpp.checkLayout {slide?}                 -> {slideWidth, slideHeight, issueCount, slides:[{index, shapes, issues}]}
//
// Vi tri/kich thuoc tinh bang point (slide mau 720x540 - do tren WPS that); chi so slide 1-based;
// `slide` ngoai khoang hoac bo trong = slide CUOI; bai chua co slide nao thi cac lenh can slide bao
// "presentation has no slides; call wpp.addSlide first" (giong `_page` cua ban LibreOffice).
// `align` nhan left/center/right -> WPS ParagraphFormat.Alignment 1/2/3.
//
// Khac ban LibreOffice (do tren WPS that 11/10/2026, xem docs/architecture/wps-bridge.md muc 6 va REFERENCE.md):
// - `addSlide`: layout la so kieu PowerPoint va WPS dung dung cac hang so nay
//   (ppLayoutTitle=1, ppLayoutText=2, ppLayoutTitleOnly=11, ppLayoutBlank=12). Layout sai bi tu choi
//   TRUOC khi chen, nen bai khong bi de lai slide rac.
// - `addText`: `Font.Bold` cua WPP nhan msoTrue = -1 (khong phai `true` nhu ET) - do tren WPS that.
// - `setNotes`: WPS khong co service NotesShape; ghi vao placeholder cuoi cua NotesPage (trang ghi chu
//   do tren WPS that co 2 placeholder: 1 = so trang, 2 = ghi chu).
// - `exportPdf`: API goc (ExportAsFixedFormat/SaveAs .pdf) bao thanh cong ma KHONG ghi file, nen add-in
//   kiem bang FileSystem.existsSync roi tu dung PDF. Ket qua phai la file "%PDF" that.
// - `checkLayout`: khong bao `overflow` (can chieu cao chu that, object model WPS khong do duoc);
//   bon loai issue con lai (offslide, small-font, overlap, dense) lam duoc, nguong giong ban LibreOffice
//   (sai so 2pt, chu nho nhat 12pt, 600 ky tu/slide).
// - Registry KHONG co `wpp.undo` (khac `et.undo`) va WPP khong co UndoRecord: khong lenh nao cua add-in
//   duoc mo UndoRecord.
// - Tham so `text` cua addText/addTextBox/setNotes: metadata ghi required nhung ban LibreOffice van chay
//   khi thieu (mac dinh ""), nen add-in giu dung hanh vi runtime do.
import { test } from "node:test";
import assert from "node:assert/strict";

import { makeWppApp, callsOf } from "./addin_fake.mjs";
import { addinCatalog, assertFails, assertKindRefused, assertMetadata, loadAddin, mockApp, run, throwing }
  from "./addin_harness.mjs";

const WPP_COMMANDS = [
  "wpp.newPresentation", "wpp.open", "wpp.listSlides", "wpp.addSlide", "wpp.addText", "wpp.addTextBox",
  "wpp.addImage", "wpp.addTable", "wpp.setNotes", "wpp.deleteSlide", "wpp.exportPdf", "wpp.save",
  "wpp.saveAs", "wpp.closeAll", "wpp.checkLayout", "wpp.goto",
];

const MUTATING = ["wpp.addSlide", "wpp.addText", "wpp.addTextBox", "wpp.addImage", "wpp.addTable",
  "wpp.setNotes", "wpp.deleteSlide"];

// Lenh PHAI tu choi khi thieu tham so bat buoc. addText/addTextBox/setNotes khong co trong day vi ban
// LibreOffice van chay voi `text` mac dinh "" - add-in giu dung hanh vi runtime do (xem dau file).
const REQUIRES_PARAM = ["wpp.open", "wpp.addImage", "wpp.addTable", "wpp.exportPdf", "wpp.saveAs"];

function slideOf(app, index) {
  return app.ActivePresentation.Slides.Item(index);
}

// Lenh lam bai theo mau + sap xep bai (compose): chi co handler tren PowerPoint/LibreOffice/Windows
// (CHUA_CO_HANDLER cua test_addin.mjs); add-in WPS JS chua viet.
// Hinh khoi + duong noi (slides_from_brief, docs/plans/18-s0 P2): cung chi co tren PowerPoint, WPS la to-do.
const TEMPLATE_COMMANDS = ["wpp.newFromTemplate", "wpp.addSlideFromTemplate", "wpp.sourceManifest", "wpp.setShapeText",
  "wpp.deleteShapes", "wpp.copyPicture", "wpp.addParagraphs", "wpp.addGrid", "wpp.deleteTagged", "wpp.addShape",
  "wpp.addConnector", "wpp.renderSlide", "wpp.addChart"];

test("registry: du 15 lenh wpp.*, metadata giong het registry LibreOffice", () => {
  const ctx = loadAddin();
  const mine = addinCatalog(ctx).filter((c) => c.name.indexOf("wpp.") === 0 && !TEMPLATE_COMMANDS.includes(c.name));
  assertMetadata(WPP_COMMANDS, mine);
  assert.equal(mine.length, WPP_COMMANDS.length, "catalog co lenh wpp.* la khong co trong danh sach nay");
  for (const command of mine) {
    assert.equal(command.kind, "wpp", command.name + ": lenh wpp.* phai co kind 'wpp'");
  }
});

// WPP khong co UndoRecord (REFERENCE.md): khong lenh nao duoc mo UndoRecord nhu ban Writer.
test("lenh wpp.* sua bai khong mo UndoRecord (WPP khong co UndoRecord)", () => {
  const ctx = loadAddin();
  const params = {
    "wpp.addSlide": {}, "wpp.addText": { text: "x" }, "wpp.addTextBox": { text: "x" },
    "wpp.addImage": { path: "/tmp/khong-co.png" }, "wpp.addTable": { rows: 1, cols: 1 },
    "wpp.setNotes": { text: "x" }, "wpp.deleteSlide": {},
  };
  for (const action of MUTATING) {
    const app = makeWppApp({ slides: 1 });
    ctx.AxiomCommands.execute(app, action, params[action]);
    const log = callsOf(app).join("\n");
    assert.ok(!/UndoRecord/.test(log), action + " khong duoc dung UndoRecord: " + log);
  }
});

test("wpp.newPresentation / wpp.open: tra ve bai dang mo", () => {
  const ctx = loadAddin();
  const app = makeWppApp({ slides: 1 });
  const made = run(ctx, app, "wpp.newPresentation");
  assert.equal(typeof made.name, "string");
  assert.equal(typeof made.fullName, "string");
  assert.equal(made.saved, true);
  assert.equal(made.opened, true);
  assert.equal(app.Presentations.Count, 2, "bai moi phai duoc them vao WPS");

  const opened = run(ctx, app, "wpp.open", { path: "/tmp/bao-cao.pptx" });
  assert.equal(opened.name, "bao-cao.pptx");
  assert.equal(opened.fullName, "/tmp/bao-cao.pptx");
  assert.equal(opened.saved, true);
  assert.equal(opened.opened, true);
  assert.equal(app.Presentations.Count, 3);
  assertFails(ctx, app, "wpp.open", {}, /'path' is required/);
});

test("wpp.listSlides: slide nao cung duoc liet ke kem text tung slide", () => {
  const ctx = loadAddin();
  const app = makeWppApp({ slides: 2 });
  const listed = run(ctx, app, "wpp.listSlides");
  assert.equal(listed.presentation, "blank.pptx");
  assert.equal(listed.slideCount, 2);
  assert.deepEqual(listed.slides, [
    { index: 1, layout: 12, shapeTexts: [] },
    { index: 2, layout: 12, shapeTexts: [] },
  ], "slide trong chua co gi: layout 12, khong text");

  run(ctx, app, "wpp.addText", { text: "Xin chao", slide: 2 });
  run(ctx, app, "wpp.addText", { text: "Tieu de phu", slide: 2 });
  const again = run(ctx, app, "wpp.listSlides");
  assert.deepEqual(again.slides[1].shapeTexts, ["Xin chao", "Tieu de phu"], "text theo thu tu shape");

  // Bai chua co slide nao: van liet ke duoc (khong loi).
  assert.deepEqual(run(ctx, makeWppApp({}), "wpp.listSlides"), {
    presentation: "blank.pptx", slideCount: 0, slides: [],
  });
});

test("wpp.addSlide: them slide cuoi, layout sai bi tu choi truoc khi chen", () => {
  const ctx = loadAddin();
  const app = makeWppApp({ slides: 1 });
  const added = run(ctx, app, "wpp.addSlide", {});
  assert.equal(added.slide, 2);
  assert.equal(added.layout, 12, "mac dinh 12 = trong");
  assert.equal(app.ActivePresentation.Slides.Count, 2);

  assert.equal(run(ctx, app, "wpp.addSlide", { layout: 1 }).layout, 1);
  assert.equal(slideOf(app, 3).Layout, 1, "layout phai duoc ap cho slide that");

  assertFails(ctx, app, "wpp.addSlide", { layout: 5 }, /must be one of/);
  assert.equal(app.ActivePresentation.Slides.Count, 3, "layout sai khong duoc de lai slide rac");
});

test("wpp.addText / wpp.addTextBox: hinh hoc, dinh dang, align", () => {
  const ctx = loadAddin();
  const app = makeWppApp({ slides: 2 });
  const added = run(ctx, app, "wpp.addText", { text: "Xin chao", fontSize: 32, bold: true,
    color: "#FF0000", align: "center" });
  assert.equal(added.slide, 2, "mac dinh la slide cuoi");
  assert.equal(typeof added.shape, "string");
  const shape = slideOf(app, 2).Shapes.Item(1);
  assert.equal(shape.Name, added.shape);
  assert.equal(shape.TextFrame.TextRange.Text, "Xin chao");
  assert.equal(shape.TextFrame.TextRange.Font.Size, 32);
  assert.equal(shape.TextFrame.TextRange.Font.Bold, -1, "WPP: msoTrue = -1 (khong phai true)");
  assert.equal(shape.TextFrame.TextRange.Font.Color.RGB, 255, "#FF0000 -> BGR 255");
  assert.equal(shape.TextFrame.TextRange.ParagraphFormat.Alignment, 2, "center = 2");
  assert.equal(shape.Left, 60, "hinh hoc mac dinh (60, 60, 540, 120)");
  assert.equal(shape.Top, 60);
  assert.equal(shape.Width, 540);
  assert.equal(shape.Height, 120);

  // align phai duoc kiem TRUOC khi tao shape: sai thi khong them textbox rac.
  const before = slideOf(app, 2).Shapes.Count;
  assertFails(ctx, app, "wpp.addText", { text: "x", align: "giua" }, /left\/center\/right/);
  assert.equal(slideOf(app, 2).Shapes.Count, before, "align sai khong duoc them shape");

  const boxed = run(ctx, app, "wpp.addText", { text: "Le trai", left: 10, top: 20, width: 100, height: 50 });
  assert.equal(boxed.slide, 2);
  assert.equal(boxed.shape, "TextBox " + app.ActivePresentation.Slides.Item(2).Shapes.Count);

  // addTextBox: cung mot textbox, khong dinh dang.
  const plain = run(ctx, app, "wpp.addTextBox", { text: "Chi chu", slide: 1, left: 30, top: 40 });
  assert.equal(plain.slide, 1);
  const simple = slideOf(app, 1).Shapes.Item(1);
  assert.equal(simple.TextFrame.TextRange.Text, "Chi chu");
  assert.equal(simple.Left, 30);
  assert.equal(simple.Top, 40);
  assert.equal(simple.Width, 540, "kich thuoc mac dinh");
});

test("wpp.addImage: kich thuoc goc, giu ty le khi chi cho mot chieu", () => {
  const ctx = loadAddin();
  const app = makeWppApp({ slides: 1, files: { "/tmp/anh.png": "PNG" },
    images: { "/tmp/anh.png": { width: 200, height: 100 } } });
  const added = run(ctx, app, "wpp.addImage", { path: "/tmp/anh.png" });
  assert.equal(added.slide, 1);
  const original = slideOf(app, 1).Shapes.Item(1);
  assert.equal(original.Name, added.shape);
  assert.equal(original.Left, 60);
  assert.equal(original.Top, 60);
  assert.equal(original.Width, 200, "bo trong width/height -> dung kich thuoc goc");
  assert.equal(original.Height, 100);

  run(ctx, app, "wpp.addImage", { path: "/tmp/anh.png", width: 100 });
  const scaled = slideOf(app, 1).Shapes.Item(2);
  assert.equal(scaled.Width, 100);
  assert.equal(scaled.Height, 50, "chi cho width -> chieu con lai giu ty le goc");

  assertFails(ctx, app, "wpp.addImage", {}, /'path' is required/);
  assertFails(ctx, app, "wpp.addImage", { path: "/tmp/khong-co.png" }, /image file not found/);
});

test("wpp.addTable: rows/cols suy ra tu values, dien duoc noi dung", () => {
  const ctx = loadAddin();
  const app = makeWppApp({ slides: 1 });
  const added = run(ctx, app, "wpp.addTable", { values: [["Ten", "Diem"], ["An", 9.5]] });
  assert.equal(added.slide, 1);
  assert.equal(added.rows, 2);
  assert.equal(added.cols, 2);
  assert.equal(added.filled, 4);
  const shape = slideOf(app, 1).Shapes.Item(1);
  assert.equal(shape.Name, added.shape);
  assert.equal(shape.Left, 60, "hinh hoc mac dinh (60, 120, 600, 200)");
  assert.equal(shape.Top, 120);
  assert.equal(shape.Width, 600);
  assert.equal(shape.Height, 200);
  assert.equal(shape.Table.Cell(1, 1).Shape.TextFrame.TextRange.Text, "Ten");
  assert.equal(shape.Table.Cell(2, 2).Shape.TextFrame.TextRange.Text, "9.5", "so -> chuoi");

  // Chi co rows/cols: bang rong.
  const empty = run(ctx, app, "wpp.addTable", { rows: 2, cols: 3 });
  assert.equal(empty.rows, 2);
  assert.equal(empty.cols, 3);
  assert.equal(empty.filled, 0);

  assertFails(ctx, app, "wpp.addTable", {}, /'rows' and 'cols' are required/);
});

test("wpp.setNotes: ghi chu trang thuyet trinh", () => {
  const ctx = loadAddin();
  const app = makeWppApp({ slides: 2 });
  const notes = run(ctx, app, "wpp.setNotes", { text: "Noi dung thuyet trinh" });
  assert.equal(notes.slide, 2, "mac dinh la slide cuoi");
  assert.equal(notes.notes, 21, "so ky tu da ghi");
  assert.equal(slideOf(app, 2).NotesPage.Shapes.Placeholders.Item(2).TextFrame.TextRange.Text,
    "Noi dung thuyet trinh", "ghi vao placeholder ghi chu (khong phai placeholder so trang)");
  assert.equal(run(ctx, app, "wpp.setNotes", { text: "Rieng slide 1", slide: 1 }).slide, 1);
  // slide ngoai khoang -> slide cuoi (giong _page cua ban LibreOffice).
  assert.equal(run(ctx, app, "wpp.setNotes", { text: "x", slide: 9 }).slide, 2);
});

test("wpp.deleteSlide: mac dinh xoa slide cuoi", () => {
  const ctx = loadAddin();
  const app = makeWppApp({ slides: 3 });
  const last = run(ctx, app, "wpp.deleteSlide");
  assert.equal(last.deleted, 3);
  assert.equal(last.slideCount, 2);
  assert.equal(app.ActivePresentation.Slides.Count, 2);

  const first = run(ctx, app, "wpp.deleteSlide", { slide: 1 });
  assert.equal(first.deleted, 1);
  assert.equal(first.slideCount, 1);

  assertFails(ctx, makeWppApp({}), "wpp.deleteSlide", {}, /presentation has no slides/);
});

test("wpp.exportPdf: API goc khong ghi file -> tu dung PDF that", () => {
  const ctx = loadAddin();
  const app = makeWppApp({ slides: 1 });
  run(ctx, app, "wpp.addText", { text: "Xin chao" });
  const out = run(ctx, app, "wpp.exportPdf", { path: "/tmp/wpp-ra.pdf" });
  assert.equal(out.exported, "/tmp/wpp-ra.pdf");
  assert.equal(app.FileSystem.existsSync("/tmp/wpp-ra.pdf"), true, "phai ghi file that (WPS bao xong ma khong ghi)");
  const pdf = app.FileSystem.ReadFile("/tmp/wpp-ra.pdf");
  assert.equal(pdf.slice(0, 5), "%PDF-", "phai la PDF that");
  assert.ok(pdf.indexOf("%%EOF") > 0, "PDF phai co %%EOF");
  assert.ok(pdf.length > 300, "PDF rong qua thi khong phai PDF that: " + pdf.length);
  assertFails(ctx, app, "wpp.exportPdf", {}, /'path' is required/);
});

test("wpp.save / wpp.saveAs / wpp.closeAll", () => {
  const ctx = loadAddin();
  const app = makeWppApp({ slides: 1 });
  const saved = run(ctx, app, "wpp.save");
  assert.equal(saved.saved, true);
  assert.equal(saved.fullName, "/tmp/blank.pptx");
  assert.ok(callsOf(app).some((line) => line === "Save()"), "phai goi Save() cua WPS");

  const as = run(ctx, app, "wpp.saveAs", { path: "/tmp/moi.pptx" });
  assert.equal(as.saved, true);
  assert.equal(as.fullName, "/tmp/moi.pptx");
  assert.equal(app.ActivePresentation.Name, "moi.pptx");
  assertFails(ctx, app, "wpp.saveAs", {}, /'path' is required/);

  // Bai chua luu lan nao (Path rong): Save() se mo hop thoai Save As cua WPS va chan vong poll,
  // nen phai tu choi ro rang nhu ban LibreOffice.
  assertFails(ctx, makeWppApp({ path: "blank.pptx" }), "wpp.save", {}, /never been saved/);

  // Nhieu bai: phai dong het, va bai chua luu duoc cuu mot ban truoc khi dong.
  const two = makeWppApp({ slides: 1 });
  two.Presentations.Add();
  assert.equal(two.Presentations.Count, 2);
  two.Presentations.Item(2).Saved = false;
  const closed = run(ctx, two, "wpp.closeAll");
  assert.equal(closed.closed, 2, "phai dong ca hai bai");
  assert.equal(two.Presentations.Count, 0);
  assert.equal(closed.rescued.length, 1, "chi bai chua luu moi duoc cuu");
  assert.match(closed.rescued[0], /rescued/);
  const log = callsOf(two);
  assert.ok(log.includes('FileSystem.Mkdir(/tmp/rescued)'), "phai tao thu muc cuu truoc khi SaveAs: " + log.slice(-8).join(" | "));
  assert.ok(log.some((line) => line.indexOf("SaveAs(") === 0 && line.indexOf("rescued") > 0),
    "phai SaveAs ban cuu ho truoc khi Close: " + log.slice(-6).join(" | "));

  // Bai khong co ten (Name rong): ten mac dinh phai duoc dung, khong duoc thanh "/tmp/rescued/".
  const unnamed = makeWppApp({ presentationName: "", slides: 1 });
  unnamed.ActivePresentation.Saved = false;
  const rescued = run(ctx, unnamed, "wpp.closeAll").rescued;
  assert.deepEqual(rescued, ["/tmp/rescued/presentation.pptx"]);
});

test("wpp.checkLayout: soat bo cuc slide (ra ngoai, chu nho, chong nhau, qua nhieu chu)", () => {
  const ctx = loadAddin();
  const app = makeWppApp({ slides: 1 });
  const clean = run(ctx, app, "wpp.checkLayout");
  assert.equal(clean.slideWidth, 720, "khổ trinh chieu do tren WPS that");
  assert.equal(clean.slideHeight, 540);
  assert.equal(clean.issueCount, 0);
  assert.deepEqual(clean.slides, [{ index: 1, shapes: [], issues: [] }], "slide trong: khong co gi de bao");

  // Shape ra ngoai slide.
  const outside = makeWppApp({ slides: 1 });
  run(ctx, outside, "wpp.addText", { text: "Ra ngoai", left: 700, top: 500 });
  const off = run(ctx, outside, "wpp.checkLayout");
  assert.deepEqual(off.slides[0].issues.map((i) => i.type), ["offslide"]);
  assert.equal(off.issueCount, 1);
  assert.equal(off.slides[0].shapes.length, 1);
  assert.equal(off.slides[0].shapes[0].name, off.slides[0].issues[0].shapes[0], "issue phai neu ten shape");
  assert.equal(off.slides[0].shapes[0].chars, 8);
  assert.equal(off.slides[0].shapes[0].left, 700, "shape bao kem vi tri/kich thuoc (point)");
  assert.equal(off.slides[0].shapes[0].width, 540);

  // Chu nho hon 12pt.
  const small = makeWppApp({ slides: 1 });
  run(ctx, small, "wpp.addText", { text: "Nho qua", fontSize: 8 });
  assert.deepEqual(run(ctx, small, "wpp.checkLayout").slides[0].issues.map((i) => i.type), ["small-font"]);

  // Hai shape chong nhau (hon 10% dien tich shape nho).
  const overlap = makeWppApp({ slides: 1 });
  run(ctx, overlap, "wpp.addText", { text: "A", left: 100, top: 100, width: 300, height: 100 });
  run(ctx, overlap, "wpp.addText", { text: "B", left: 200, top: 120, width: 300, height: 100 });
  const over = run(ctx, overlap, "wpp.checkLayout");
  assert.deepEqual(over.slides[0].issues.map((i) => i.type), ["overlap"]);
  assert.equal(over.slides[0].issues[0].shapes.length, 2);

  // Mot slide qua nhieu chu (> 600 ky tu).
  const dense = makeWppApp({ slides: 1 });
  run(ctx, dense, "wpp.addText", { text: "x".repeat(700) });
  assert.deepEqual(run(ctx, dense, "wpp.checkLayout").slides[0].issues.map((i) => i.type), ["dense"]);

  // Chi soat mot slide khi duoc chi dinh.
  const two = makeWppApp({ slides: 2 });
  run(ctx, two, "wpp.addText", { text: "Ra ngoai", left: 700, top: 500, slide: 1 });
  const only = run(ctx, two, "wpp.checkLayout", { slide: 2 });
  assert.equal(only.slides.length, 1);
  assert.equal(only.slides[0].index, 2);
  assert.equal(only.issueCount, 0);
  assert.equal(run(ctx, two, "wpp.checkLayout").issueCount, 1);
  assertFails(ctx, two, "wpp.checkLayout", { slide: 5 }, /'slide' must be between 1 and 2/);
});

test("wpp.* chi chay trong Presentation; lenh la va object model hong khong throw", () => {
  const ctx = loadAddin();
  for (const app of [{ Name: "WPS Writer", ActiveDocument: { Name: "x.docx" } },
    { Name: "WPS Spreadsheets", ActiveWorkbook: { Name: "x.xlsx" } }]) {
    const reply = assertKindRefused(ctx, app, "wpp.listSlides");
    assert.match(String(reply.error), /wpp\.listSlides/);
  }

  const app = makeWppApp({ slides: 1 });
  const unknown = ctx.AxiomCommands.execute(app, "wpp.khongCo", {});
  assert.equal(unknown.ok, false);
  assert.match(String(unknown.error), /wpp\.khongCo/);

  for (const action of ["wpp.listSlides", "wpp.addSlide", "wpp.addText", "wpp.checkLayout"]) {
    const broken = mockApp([], { "app.Name": "WPS Presentation",
      "app.ActivePresentation": throwing("boom") });
    assertFails(ctx, broken, action, { text: "x" }, /boom/);
  }
});

test("moi lenh wpp.* deu tra {ok:false, error} khi thieu tham so bat buoc, khong throw", () => {
  const ctx = loadAddin();
  const app = makeWppApp({ slides: 1 });
  for (const command of REQUIRES_PARAM) {
    let reply;
    assert.doesNotThrow(() => { reply = ctx.AxiomCommands.execute(app, command, {}); },
      command + ": execute khong duoc throw");
    assert.equal(reply.ok, false, command + " thieu tham so bat buoc phai ok:false, duoc " + JSON.stringify(reply));
    assert.ok(String(reply.error).length > 0, command + ": phai co thong diep loi");
  }
});
