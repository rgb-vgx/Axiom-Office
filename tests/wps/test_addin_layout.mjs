// Spec T4 (doc_from_brief tren WPS, 01/11/2026): writer.checkLayout + writer.deleteParagraphs cho JS add-in WPS.
// writer.checkLayout la lenh Required cuoi cung cua doc_from_brief (core-go/internal/tools/needs.go) con thieu, nen tool
// soan van ban hanh chinh bi an tren WPS; deleteParagraphs la lenh ma moi goi y sua cua checkLayout tro toi.
//
//   node --test tests/wps/test_addin_layout.mjs
//
// Hop dong (ten lenh, tham so, dang ket qua, cau chu) la cua catalog/live-commands.json va ban Windows
// (src/AxiomOffice/Bridge/CommandDispatcher.WriterLayout.cs: WriterCheckLayout, WriterDeleteParagraphs, DeleteParagraph;
// InsideToc o .WriterStructure.cs; Shorten o .Checks.cs) - Core dung chung moi nen tang nen ket qua phai giong tung chu.
// Khac ban Windows CHI o cho WPS 12.1.2 that khac Word (do 01/11/2026, xem dau tests/wps/addin_fake_layout.mjs):
//   - Information(12) tra boolean; doc.Paragraphs gom ca doan "dau cuoi hang" cua bang (mot muc bang van chi tinh mot lan).
//   - deleteParagraphs: kiem HET truoc roi moi xoa (WPS chua do Undo(1) de lui giua chung), xoa tu so lon ve so nho.
//     Doan cuoi bai ngay sau bang: chi xoa chu (MoveEnd(1, -1) + Delete), nhu ban Windows.
//   - "Trong muc luc": Start <= vi tri < End (ban Windows dung <= End nen doan NGAY SAU muc luc cung bi tinh la muc luc).

import test from "node:test";
import assert from "node:assert/strict";
import { makeLayoutApp, layoutOf } from "./addin_fake_layout.mjs";
import { loadAddin, run } from "./addin_harness.mjs";

function fails(ctx, app, action, params, pattern) {
  const reply = ctx.AxiomCommands.execute(app, action, params);
  assert.equal(reply.ok, false, action + " phai bao loi: " + JSON.stringify(reply));
  assert.match(String(reply.error), pattern);
}

const types = (out) => out.issues.map((i) => i.type);

// ---- checkLayout ----

test("checkLayout: bai sach -> so trang, so chu, so doan, tieu de kem trang; khong issue; Repaginate; khong mo Undo", () => {
  const ctx = loadAddin();
  const app = makeLayoutApp([
    { t: "Báo cáo quý III", page: 1, level: 1, top: 72 },
    { t: "Doanh thu tăng đều qua các tháng.", page: 1 },
    { t: "Kế hoạch", page: 2, level: 2, top: 72 },
    { t: "Mở rộng thị trường miền Trung.", page: 2 },
  ], { words: 120 });
  const out = run(ctx, app, "writer.checkLayout");
  assert.deepEqual(out, {
    pages: 2, words: 120, paragraphs: 4, tables: 0,
    headings: [{ n: 1, level: 1, page: 1, t: "Báo cáo quý III" }, { n: 3, level: 2, page: 2, t: "Kế hoạch" }],
    issueCount: 0, issues: [],
  });
  assert.ok(layoutOf(app).repaginated >= 1, "phai Repaginate truoc khi doc trang");
  assert.deepEqual(layoutOf(app).undo, [], "lenh chi doc: khong mo UndoRecord");
});

test("checkLayout: trang trang giua hai dau ngat trang -> blank-page chi ra doan nam TREN trang trang", () => {
  const ctx = loadAddin();
  const app = makeLayoutApp([
    { t: "Mở đầu", page: 1, level: 1 },
    { t: "Đoạn một.", page: 1 },
    { t: "\f", page: 1 },
    { t: "\f", page: 2 },
    { t: "Phụ lục", page: 3, level: 1 },
    { t: "Bảng số liệu.", page: 3 },
  ]);
  const out = run(ctx, app, "writer.checkLayout");
  assert.equal(out.pages, 3);
  assert.deepEqual(out.issues, [{
    type: "blank-page",
    detail: "page 2 of 3 is blank: delete the empty paragraph(s) / page break(s) 4 with writer.deleteParagraphs {\"paragraphs\": [4]}",
    page: 2, paragraphs: [4],
  }]);
  assert.equal(out.issueCount, 1);
});

test("checkLayout: trang trang khong co doan nao cua rieng no -> goi y chung", () => {
  const ctx = loadAddin();
  const app = makeLayoutApp([{ t: "Một trang.", page: 1 }], { pages: 2 });
  const out = run(ctx, app, "writer.checkLayout");
  assert.deepEqual(out.issues, [{
    type: "blank-page", detail: "page 2 of 2 is blank: remove the extra page break or empty paragraphs around it",
    page: 2, paragraphs: [],
  }]);
});

test("checkLayout: tieu de nam cuoi trang, noi dung sang trang sau -> heading-at-page-bottom (ngat trang, hoac xoa doan trong)", () => {
  const ctx = loadAddin();
  const plain = makeLayoutApp([
    { t: "Mở đầu", page: 1, level: 1 },
    { t: "Nội dung dài.", page: 1 },
    { t: "Kết luận", page: 1, level: 2, top: 760 },
    { t: "Đề nghị phê duyệt.", page: 2 },
  ]);
  assert.deepEqual(run(ctx, plain, "writer.checkLayout").issues, [{
    type: "heading-at-page-bottom",
    detail: "heading 'Kết luận' (paragraph 3) ends page 1 but its text starts on page 2: start it on the next page: writer.insertPageBreak {\"before\": 3}",
    paragraph: 3, page: 1,
  }]);
  const gap = makeLayoutApp([
    { t: "Kết luận", page: 1, level: 2 },
    { t: "", page: 1 },
    { t: "Đề nghị phê duyệt.", page: 2 },
  ]);
  assert.deepEqual(run(ctx, gap, "writer.checkLayout").issues, [{
    type: "heading-at-page-bottom",
    detail: "heading 'Kết luận' (paragraph 1) ends page 1 but its text starts on page 2: delete the empty paragraph(s) 2 after it (writer.deleteParagraphs) so it stays with its text",
    paragraph: 1, page: 1,
  }]);
  // Doan van tran sang trang sau nhung BAT DAU cung trang voi tieu de: khong phai loi.
  const flowing = makeLayoutApp([{ t: "Kết luận", page: 1, level: 2 }, { t: "Đoạn dài.", startPage: 1, page: 2 }]);
  assert.deepEqual(types(run(ctx, flowing, "writer.checkLayout")), []);
  // Tieu de cuoi bai, khong co gi sau: khong phai loi.
  assert.deepEqual(types(run(ctx, makeLayoutApp([{ t: "Phụ lục", page: 1, level: 1 }]), "writer.checkLayout")), []);
});

test("checkLayout: bang nho bi cat sang trang -> split-table (bang dan trang va bang > 15 hang thi khong); bang la noi dung cua ca hai trang", () => {
  const ctx = loadAddin();
  const rows = (n) => Array.from({ length: n }, (_, i) => ["Mục " + (i + 1), String(i)]);
  const app = makeLayoutApp([
    { t: "Số liệu", page: 1, level: 1 },
    { table: rows(3), page: 1, lastPage: 2 },
    { t: "Ký tên", page: 2 },
    { table: rows(3), page: 2, lastPage: 3, title: "AxiomLayout" },
    { t: "", page: 3 },
    { table: rows(16), page: 3, lastPage: 4 },
    { t: "Hết.", page: 4 },
  ]);
  const out = run(ctx, app, "writer.checkLayout");
  assert.equal(out.tables, 3);
  assert.deepEqual(out.issues, [{
    type: "split-table",
    detail: "table 1 has at most 15 rows but is cut across pages: keep it on one page with writer.formatTable {\"table\": 1, \"keepTogether\": true}",
    table: 1,
  }]);
  // Doan cua bang (o + dau cuoi hang) van duoc dem trong `paragraphs` nhu doc.Paragraphs.Count.
  assert.equal(out.paragraphs, 1 + 9 + 1 + 9 + 1 + 48 + 1);
});

test("checkLayout: hai doan trong tro len lien nhau -> empty-paragraphs; mot doan trong, dau ngat trang va o bang thi khong", () => {
  const ctx = loadAddin();
  const app = makeLayoutApp([
    { t: "Mở đầu", page: 1, level: 1 },
    { t: "", page: 1 },
    { t: "", page: 1 },
    { t: "", page: 1 },
    { t: "Đoạn hai.", page: 1 },
    { t: "", page: 1 },
    { t: "Đoạn ba.", page: 1 },
    { t: "", page: 1 },
    { t: "\f", page: 1 },
    { t: "", page: 2 },
    { t: "Trang hai.", page: 2 },
    { table: [["", ""], ["", ""]], page: 2 },
    { t: "Hết.", page: 2 },
  ]);
  const out = run(ctx, app, "writer.checkLayout");
  assert.deepEqual(out.issues, [{
    type: "empty-paragraphs", detail: "3 empty paragraphs in a row (2-4) used as spacing: delete them with writer.deleteParagraphs",
    paragraphs: [2, 3, 4],
  }]);
});

test("checkLayout: maxPages -> too-long kem so chu can cat (theo so chu moi trang)", () => {
  const ctx = loadAddin();
  const app = makeLayoutApp([{ t: "A", page: 1 }, { t: "B", page: 2 }, { t: "C", page: 3 }], { words: 900 });
  const out = run(ctx, app, "writer.checkLayout", { maxPages: 2 });
  assert.deepEqual(out.issues, [{
    type: "too-long",
    detail: "the document has 3 pages but the limit is 2: cut about 300 words (~300 words per page) - shorten long paragraphs and drop "
      + "repeated points (writer.replaceMany / writer.deleteParagraphs), keep every required section and table",
    pages: 3, maxPages: 2,
  }]);
  assert.deepEqual(types(run(ctx, app, "writer.checkLayout", { maxPages: 3 })), []);
  assert.deepEqual(types(run(ctx, app, "writer.checkLayout")), [], "khong co maxPages thi khong xet do dai");
});

test("checkLayout: muc luc va doan chi co anh la noi dung (khong bi bao trong/trang trang); tieu de dai bi cat", () => {
  const ctx = loadAddin();
  const long = "Phần một: " + "rất dài ".repeat(10);
  const app = makeLayoutApp([
    { toc: ["", "Mục lục"], page: 1 },
    { t: "\f", page: 1 },
    { t: "", page: 2, shapes: 1 },
    { t: "", page: 2, shapes: 1 },
    { t: long, page: 2, level: 1 },
    { t: "Nội dung.", page: 2 },
  ]);
  const out = run(ctx, app, "writer.checkLayout");
  assert.deepEqual(out.issues, []);
  assert.deepEqual(out.headings, [{ n: 6, level: 1, page: 2, t: long.trim().slice(0, 60) + "..." }]);
});

// ---- deleteParagraphs ----

test("deleteParagraphs: xoa theo so (tu so lon ve so nho), tra so da xoa + chu (cat 60) + so doan con lai; mot buoc Undo", () => {
  const ctx = loadAddin();
  const app = makeLayoutApp([
    { t: "Mở đầu", page: 1, level: 1 },
    { t: "", page: 1 },
    { t: "\f", page: 1 },
    { t: "Đoạn thừa " + "x".repeat(70), page: 2 },
    { t: "Hết.", page: 2 },
  ]);
  const out = run(ctx, app, "writer.deleteParagraphs", { paragraphs: [2, 4, 3, 4] });
  assert.deepEqual(out, { deleted: 3, texts: [("Đoạn thừa " + "x".repeat(70)).slice(0, 60) + "...", "", ""], paragraphs: 2 });
  assert.deepEqual(layoutOf(app).texts, ["Mở đầu", "Hết."]);
  assert.deepEqual(layoutOf(app).undo, ["start:AI: writer.deleteParagraphs", "end"]);
});

test("deleteParagraphs: doan cuoi bai ngay sau bang chi bi xoa chu (WPS luon giu mot doan sau bang)", () => {
  const ctx = loadAddin();
  const app = makeLayoutApp([{ t: "Bảng", page: 1, level: 1 }, { table: [["A", "B"]], page: 1 }, { t: "ghi chú", page: 1 }]);
  const total = 1 + 3 + 1;
  const out = run(ctx, app, "writer.deleteParagraphs", { paragraphs: [total] });
  assert.deepEqual(out, { deleted: 1, texts: ["ghi chú"], paragraphs: total });
  assert.equal(layoutOf(app).texts[total - 1], "");
  const empty = makeLayoutApp([{ t: "Bảng", page: 1, level: 1 }, { table: [["A", "B"]], page: 1 }, { t: "", page: 1 }]);
  assert.deepEqual(run(ctx, empty, "writer.deleteParagraphs", { paragraphs: [total] }), { deleted: 0, texts: [], paragraphs: total });
});

test("deleteParagraphs: danh sach rong, so ngoai pham vi, doan trong bang, doan muc luc -> bao loi va KHONG xoa gi", () => {
  const ctx = loadAddin();
  const blocks = [
    { toc: ["Mục lục"], page: 1 },
    { t: "", page: 1 },
    { table: [["A"]], page: 1 },
    { t: "Hết.", page: 1 },
  ];
  const app = makeLayoutApp(blocks);
  fails(ctx, app, "writer.deleteParagraphs", { paragraphs: [] }, /'paragraphs' must list paragraph numbers from writer\.outline \/ writer\.checkLayout/);
  fails(ctx, app, "writer.deleteParagraphs", {}, /'paragraphs'/);
  fails(ctx, app, "writer.deleteParagraphs", { paragraphs: [2, 9] }, /paragraph 9 does not exist \(the document has 5\)/);
  fails(ctx, app, "writer.deleteParagraphs", { paragraphs: [2, 3] }, /paragraph 3 is inside a table; edit the table instead/);
  fails(ctx, app, "writer.deleteParagraphs", { paragraphs: [1, 2] },
    /paragraph 1 is part of the table of contents; it cannot be edited by hand - call writer\.insertToc again to refresh it/);
  assert.deepEqual(layoutOf(app).deletes, [], "kiem het truoc roi moi xoa: loi thi khong doan nao bi xoa");
  assert.equal(layoutOf(app).texts.length, 5);
});

test("checkLayout/deleteParagraphs chi chay trong Writer", () => {
  const ctx = loadAddin();
  const calc = { Name: "WPS Spreadsheets" };
  for (const action of ["writer.checkLayout", "writer.deleteParagraphs"]) {
    const reply = ctx.AxiomCommands.execute(calc, action, { paragraphs: [1] });
    assert.equal(reply.ok, false);
    assert.match(String(reply.error), /cannot run in/);
  }
});
