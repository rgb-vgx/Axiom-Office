// Spec P1-1 (docs/research/doi-chieu-wps-ai.md): Track Changes, comment va thay hang loat cho JS add-in WPS - nen cua
// "Soat chinh ta tieng Viet ca tai lieu" (AI sua thanh revision, cho can can nhac thanh comment).
//
//   node --test tests/wps/test_addin_review.mjs
//   node --test --test-name-pattern=^tracking tests/wps/test_addin_review.mjs     (moi nhom mot tien to)
//
// Hop dong (ten lenh, tham so, dang ket qua) la cua catalog/live-commands.json va ban Windows
// (src/AxiomOffice/Bridge/CommandDispatcher.WriterReview.cs, CommandDispatcher.Writer.cs: WriterReplaceMany) - Core dung
// chung cho moi nen tang nen dang ket qua phai giong. Hanh vi WPS ma fake mo phong da do tren WPS 12.1.2 that
// (31/10/2026, tests/wps/REFERENCE.md "Track Changes va comment"): tac gia = app.UserName luc sua; moi cho thay sinh hai
// revision (Type 2 xoa roi Type 1 chen); Content.Text khong chua chu da xoa; mot UndoRecord go ca nhom; Find.Execute
// khong thay thi co Range ve cho khop; Comment.Range.Text ket bang "\r"; Revision/Comment.Date la Date.

import test from "node:test";
import assert from "node:assert/strict";
import { makeWriterApp, writerStateOf, writerReviewOf, writerAdvanceClock } from "./addin_fake.mjs";
import { loadAddin, run } from "./addin_harness.mjs";

const DATE = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/;   // "yyyy-MM-dd HH:mm" nhu ban Windows

function writer(text, options = {}) {
  return makeWriterApp({ text, paragraphs: true, ...options });
}

function fails(ctx, app, action, params, pattern) {
  const reply = ctx.AxiomCommands.execute(app, action, params);
  assert.equal(reply.ok, false, action + " phai bao loi: " + JSON.stringify(reply));
  assert.match(String(reply.error), pattern);
}

// ---- tracking: writer.setTracking + writer.listRevisions ----

test("tracking: setTracking bat theo doi + dat tac gia; tat + restoreAuthor tra lai ten nguoi dung", () => {
  const ctx = loadAddin();
  const app = writer("Phong da tuyen dung tam nguoi.\r");
  const on = run(ctx, app, "writer.setTracking", { on: true, author: "Axiom AI" });
  assert.deepEqual(on, { tracking: true, previous: false, author: "Axiom AI" });
  assert.equal(writerReviewOf(app).track, true);
  assert.equal(writerReviewOf(app).userName, "Axiom AI", "tac gia revision theo app.UserName (do tren WPS that)");
  run(ctx, app, "writer.replaceAll", { find: "tuyen dung", replace: "tuyển dụng" });
  assert.deepEqual(writerReviewOf(app).revisions.map((r) => [r.type, r.author, r.text]),
    [[2, "Axiom AI", "tuyen dung"], [1, "Axiom AI", "tuyển dụng"]]);
  const off = run(ctx, app, "writer.setTracking", { on: false, restoreAuthor: true });
  assert.deepEqual(off, { tracking: false, previous: true, author: "thuyetmt" });
  assert.equal(writerReviewOf(app).userName, "thuyetmt", "restoreAuthor tra lai dung ten cu cua nguoi dung");
  assert.equal(writerReviewOf(app).track, false);
});

test("tracking: setTracking khong co author thi giu ten nguoi dung; khong tao buoc hoan tac", () => {
  const ctx = loadAddin();
  const app = writer("Mot doan.\r");
  assert.deepEqual(run(ctx, app, "writer.setTracking", { on: true }), { tracking: true, previous: false, author: "thuyetmt" });
  assert.equal(writerReviewOf(app).userName, "thuyetmt");
  // restoreAuthor khi chua tung doi ten: khong doi gi, khong loi.
  assert.deepEqual(run(ctx, app, "writer.setTracking", { on: false, restoreAuthor: true }),
    { tracking: false, previous: true, author: "thuyetmt" });
  assert.deepEqual(run(ctx, app, "writer.undo", { count: 1 }), { undone: 0 }, "bat/tat theo doi khong phai mot buoc sua");
});

test("tracking: listRevisions tra id/loai/tac gia/ngay/chu/story/so doan + total, authors, tracking", () => {
  const ctx = loadAddin();
  const app = writer("Dong mot.\rPhong da tuyen dung tam nguoi.\r");
  run(ctx, app, "writer.setTracking", { on: true, author: "Axiom AI" });
  run(ctx, app, "writer.replaceAll", { find: "tuyen dung", replace: "tuyển dụng" });
  const list = run(ctx, app, "writer.listRevisions", {});
  assert.equal(list.total, 2);
  assert.equal(list.tracking, true);
  assert.deepEqual(list.authors, { "Axiom AI": 2 });
  assert.equal(list.revisions.length, 2);
  const [del, ins] = list.revisions;
  assert.deepEqual({ id: del.id, type: del.type, author: del.author, text: del.text, story: del.story, paragraph: del.paragraph },
    { id: "1:1", type: "delete", author: "Axiom AI", text: "tuyen dung", story: "main", paragraph: 2 });
  assert.deepEqual({ id: ins.id, type: ins.type, text: ins.text, paragraph: ins.paragraph },
    { id: "1:2", type: "insert", text: "tuyển dụng", paragraph: 2 });
  assert.match(del.date, DATE, "ngay dang yyyy-MM-dd HH:mm nhu ban Windows (Revision.Date cua WPS la Date)");
});

test("tracking: ngay revision/comment la gio tren may, khong cong them mui gio (Date cua WPS mang gio dia phuong nhan UTC)", () => {
  // Dong ho fake bat dau 09:00 tren may; moi lan sua +1 giay. May o +07 (nhu may that) thi doc bang getHours() ra 16:00 - sai.
  const savedTz = process.env.TZ;
  process.env.TZ = "Asia/Ho_Chi_Minh";
  try {
    const ctx = loadAddin();
    const app = writer("Phong da tuyen dung.\r");
    run(ctx, app, "writer.setTracking", { on: true, author: "Axiom AI" });
    run(ctx, app, "writer.replaceAll", { find: "Phong da", replace: "Phòng đã" });
    assert.deepEqual(run(ctx, app, "writer.listRevisions", {}).revisions.map((r) => r.date),
      ["2026-10-31 09:00", "2026-10-31 09:00"]);
    run(ctx, app, "writer.addComment", { find: "tuyen dung", text: "Xem lai" });
    assert.equal(run(ctx, app, "writer.listComments", {}).comments[0].date, "2026-10-31 09:00");
  } finally {
    if (savedTz === undefined) { delete process.env.TZ; } else { process.env.TZ = savedTz; }
  }
});

test("tracking: listRevisions loc theo author va cat theo max; khong co revision thi danh sach rong", () => {
  const ctx = loadAddin();
  const app = writer("a1 b2 a1.\r");
  assert.deepEqual(run(ctx, app, "writer.listRevisions", {}), { revisions: [], total: 0, authors: {}, tracking: false });
  run(ctx, app, "writer.setTracking", { on: true, author: "Axiom AI" });
  run(ctx, app, "writer.replaceAll", { find: "a1", replace: "x" });
  const limited = run(ctx, app, "writer.listRevisions", { max: 1 });
  assert.equal(limited.revisions.length, 1);
  assert.equal(limited.total, 4, "total dem moi revision khop, khong bi max cat");
  const other = run(ctx, app, "writer.listRevisions", { author: "nguoi khac" });
  assert.deepEqual([other.revisions, other.total], [[], 0]);
  assert.deepEqual(other.authors, { "Axiom AI": 4 }, "authors van liet ke moi tac gia");
  assert.equal(run(ctx, app, "writer.listRevisions", { author: "axiom ai" }).total, 4, "so tac gia khong phan biet hoa thuong");
});

// ---- comments: writer.addComment + writer.listComments ----

test("comments: addComment theo find neo dung chu, tac gia = UserName, mot buoc hoan tac, tra {comment, paragraph}", () => {
  const ctx = loadAddin();
  const app = writer("Dong mot.\rPhong da tuyen dung tam nguoi.\r");
  const reply = run(ctx, app, "writer.addComment", { find: "tuyen dung", text: "Nên viết có dấu: tuyển dụng" });
  assert.deepEqual(reply, { comment: 1, paragraph: 2 });
  assert.deepEqual(writerReviewOf(app).comments,
    [{ author: "thuyetmt", text: "Nên viết có dấu: tuyển dụng", anchor: "tuyen dung", done: false }]);
  assert.equal(writerStateOf(app).text, "Dong mot.\rPhong da tuyen dung tam nguoi.\r", "them comment khong doi chu");
  run(ctx, app, "writer.undo", { count: 1 });
  assert.deepEqual(writerReviewOf(app).comments, [], "addComment la MOT buoc hoan tac");
});

test("comments: addComment dang theo doi voi tac gia AI thi comment mang ten AI", () => {
  const ctx = loadAddin();
  const app = writer("Phong da tuyen dung.\r");
  run(ctx, app, "writer.setTracking", { on: true, author: "Axiom AI" });
  run(ctx, app, "writer.addComment", { find: "tuyen dung", text: "Cần có dấu" });
  assert.equal(writerReviewOf(app).comments[0].author, "Axiom AI");
});

test("comments: addComment theo paragraph neo ca doan; tham so sai thi bao loi ro", () => {
  const ctx = loadAddin();
  const app = writer("Dong mot.\rDong hai co loi.\r");
  assert.deepEqual(run(ctx, app, "writer.addComment", { paragraph: 2, text: "Xem lai doan nay" }), { comment: 1, paragraph: 2 });
  assert.match(writerReviewOf(app).comments[0].anchor, /^Dong hai co loi\./);
  fails(ctx, app, "writer.addComment", { find: "x" }, /'text' is required/);
  fails(ctx, app, "writer.addComment", { find: "khong co trong tai lieu", text: "t" }, /not found/);
  fails(ctx, app, "writer.addComment", { paragraph: 9, text: "t" }, /between 1 and 2/);
  fails(ctx, app, "writer.addComment", { text: "t" }, /paragraph.*find|find.*paragraph/);
});

test("comments: listComments tra id/tac gia/noi dung/chu neo/so doan/done/replies/ngay; mac dinh bo comment da xong", () => {
  const ctx = loadAddin();
  const app = writer("Dong mot co loi.\rDong hai cung loi.\r");
  run(ctx, app, "writer.addComment", { find: "mot co loi", text: "Sửa câu một" });
  run(ctx, app, "writer.addComment", { find: "hai cung loi", text: "Sửa câu hai" });
  app.ActiveDocument.Comments.Item(2).Done = true;
  const open = run(ctx, app, "writer.listComments", {});
  assert.equal(open.total, 2);
  assert.equal(open.open, 1);
  assert.equal(open.comments.length, 1);
  const first = open.comments[0];
  assert.deepEqual({ id: first.id, author: first.author, text: first.text, anchor: first.anchor, paragraph: first.paragraph,
    done: first.done, replies: first.replies },
  { id: 1, author: "thuyetmt", text: "Sửa câu một", anchor: "mot co loi", paragraph: 1, done: false, replies: [] });
  assert.match(first.date, DATE);
  const all = run(ctx, app, "writer.listComments", { includeDone: true });
  assert.deepEqual(all.comments.map((c) => [c.id, c.done, c.paragraph]), [[1, false, 1], [2, true, 2]]);
  assert.equal(run(ctx, app, "writer.listComments", { includeDone: true, max: 1 }).comments.length, 1);
});

// ---- replaceMany: writer.replaceMany ----

test("replaceMany: thay nhieu cap theo thu tu trong MOT buoc hoan tac; tra {edits, replaced, counts, notFound}", () => {
  const ctx = loadAddin();
  const app = writer("a1 b2 a1 c3.\r");
  const reply = run(ctx, app, "writer.replaceMany", {
    edits: [{ find: "a1", replace: "x" }, { find: "zz", replace: "y" }, { find: "c3", replace: "z" }],
  });
  assert.deepEqual(reply, { edits: 3, replaced: 3, counts: [2, 0, 1], notFound: ["zz"] });
  assert.equal(writerStateOf(app).text, "x b2 x z.\r");
  run(ctx, app, "writer.undo", { count: 1 });
  assert.equal(writerStateOf(app).text, "a1 b2 a1 c3.\r", "ca loat thay la MOT buoc hoan tac");
});

test("replaceMany: moi cap thay deu thi khong co notFound; cap thieu find bi bo qua", () => {
  const ctx = loadAddin();
  const app = writer("mot hai.\r");
  assert.deepEqual(run(ctx, app, "writer.replaceMany", { edits: [{ find: "mot", replace: "1" }, { replace: "bo qua" }] }),
    { edits: 1, replaced: 1, counts: [1] });
});

test("replaceMany: tham so sai thi bao loi ro (rong, qua 200 cap)", () => {
  const ctx = loadAddin();
  const app = writer("abc.\r");
  fails(ctx, app, "writer.replaceMany", {}, /non-empty array of \{find, replace\}/);
  fails(ctx, app, "writer.replaceMany", { edits: [] }, /non-empty array of \{find, replace\}/);
  const many = Array.from({ length: 201 }, (_, i) => ({ find: "k" + i, replace: "v" }));
  fails(ctx, app, "writer.replaceMany", { edits: many }, /at most 200/);
  assert.equal(writerStateOf(app).text, "abc.\r");
});

test("replaceMany: dang theo doi thay doi thi moi cho thay la revision cua tac gia AI", () => {
  const ctx = loadAddin();
  const app = writer("Phong da tuyen dung, quy ba.\r");
  run(ctx, app, "writer.setTracking", { on: true, author: "Axiom AI" });
  run(ctx, app, "writer.replaceMany", { edits: [{ find: "Phong da", replace: "Phòng đã" }, { find: "quy ba", replace: "quý ba" }] });
  const revs = writerReviewOf(app).revisions;
  assert.equal(revs.length, 4);
  assert.ok(revs.every((r) => r.author === "Axiom AI"));
  assert.equal(writerStateOf(app).text, "Phòng đã tuyen dung, quý ba.\r", "Text khong chua chu da xoa (nhu WPS that)");
});

test("replaceMany: dang khoa vung boi den thi chi thay trong vung (giong writer.replaceAll)", () => {
  const ctx = loadAddin();
  const app = writer("a1 b2 a1.\r", { selection: { start: 0, end: 5 } });
  assert.equal(run(ctx, app, "writer.lockSelection", {}).chars, 5);
  const reply = run(ctx, app, "writer.replaceMany", { edits: [{ find: "a1", replace: "x" }] });
  assert.deepEqual(reply.counts, [1], "chi dem cho trong vung khoa");
  assert.equal(writerStateOf(app).text, "x b2 a1.\r");
  run(ctx, app, "writer.unlockSelection", {});
});

// ---- reviewRevisions: writer.reviewRevisions ----
// "Hoan tac luot nay" cua Core khi luot bat Track Changes (core-go/internal/agent/tracking.go UndoRun) goi
// {action: "reject", author: "Axiom AI", since: <luc bat dau luot, UTC>}. Do tren WPS 12.1.2 that (01/11/2026):
// Revision.Accept/Reject chay khi TAT theo doi; tu choi theo tac gia giu nguyen thay doi cua nguoi khac; mot UndoRecord go
// lai ca nhom. Revision.Date mang gio dia phuong gan nhan UTC (REFERENCE.md) - so voi `since` phai doi ve UTC that.

function withTz(tz, body) {
  const saved = process.env.TZ;
  process.env.TZ = tz;
  try { return body(); } finally {
    if (saved === undefined) { delete process.env.TZ; } else { process.env.TZ = saved; }
  }
}

test("reviewRevisions: reject theo author + since chi go thay doi cua luot nay; giu cua nguoi dung va luot AI truoc", () => {
  withTz("Asia/Ho_Chi_Minh", () => {
    const ctx = loadAddin();
    const app = writer("mot hai ba bon nam.\r");
    run(ctx, app, "writer.setTracking", { on: true, author: "Axiom AI" });
    run(ctx, app, "writer.replaceAll", { find: "mot", replace: "một" });          // luot AI truoc: ~09:00 tren may
    run(ctx, app, "writer.setTracking", { on: true, restoreAuthor: true });
    run(ctx, app, "writer.replaceAll", { find: "hai", replace: "HAI" });          // nguoi dung go
    writerAdvanceClock(app, 10 * 60 * 1000);
    run(ctx, app, "writer.setTracking", { on: true, author: "Axiom AI" });
    run(ctx, app, "writer.replaceMany", { edits: [{ find: "ba", replace: "BA" }, { find: "nam", replace: "năm" }] });
    // Luot nay bat dau 09:05 tren may (+07) = 02:05 UTC; luot AI truoc luc 09:00 khong duoc dong toi.
    const reply = run(ctx, app, "writer.reviewRevisions", { action: "reject", author: "axiom ai", since: "2026-10-31T02:05:00Z" });
    assert.deepEqual(reply, { action: "reject", count: 4 });
    assert.equal(writerStateOf(app).text, "một HAI ba bon nam.\r");
    assert.deepEqual(writerReviewOf(app).revisions.map((r) => [r.author, r.text]),
      [["Axiom AI", "mot"], ["Axiom AI", "một"], ["thuyetmt", "hai"], ["thuyetmt", "HAI"]]);
    assert.equal(writerReviewOf(app).track, true, "tra lai trang thai theo doi nhu truoc lenh");
    run(ctx, app, "writer.undo", { count: 1 });
    assert.equal(writerStateOf(app).text, "một HAI BA bon năm.\r", "ca loat tu choi la MOT buoc hoan tac");
    assert.equal(writerReviewOf(app).revisions.length, 8);
  });
});

test("reviewRevisions: accept all giu chu moi va xoa het danh dau; theo ids thi chi cap do", () => {
  const ctx = loadAddin();
  const app = writer("mot hai.\r");
  run(ctx, app, "writer.setTracking", { on: true, author: "Axiom AI" });
  run(ctx, app, "writer.replaceMany", { edits: [{ find: "mot", replace: "một" }, { find: "hai", replace: "hài" }] });
  const ids = run(ctx, app, "writer.listRevisions", {}).revisions.map((r) => r.id);
  assert.deepEqual(ids, ["1:1", "1:2", "1:3", "1:4"]);
  assert.deepEqual(run(ctx, app, "writer.reviewRevisions", { action: "reject", ids: ["1:3", "1:4"] }), { action: "reject", count: 2 });
  assert.equal(writerStateOf(app).text, "một hai.\r");
  assert.deepEqual(run(ctx, app, "writer.reviewRevisions", { action: "ACCEPT", all: true }), { action: "accept", count: 2 });
  assert.equal(writerStateOf(app).text, "một hai.\r");
  assert.deepEqual(writerReviewOf(app).revisions, []);
});

test("reviewRevisions: tham so sai hoac id khong co thi bao loi ro, khong doi tai lieu", () => {
  const ctx = loadAddin();
  const app = writer("mot.\r");
  run(ctx, app, "writer.setTracking", { on: true, author: "Axiom AI" });
  run(ctx, app, "writer.replaceAll", { find: "mot", replace: "một" });
  fails(ctx, app, "writer.reviewRevisions", { action: "xoa", all: true }, /'action' must be 'accept' or 'reject'/);
  fails(ctx, app, "writer.reviewRevisions", { action: "reject" }, /give 'ids' \(from writer.listRevisions\), 'author', or 'all': true/);
  fails(ctx, app, "writer.reviewRevisions", { action: "reject", author: "Axiom AI", since: "hom qua" }, /'since' must be an ISO 8601 time/);
  fails(ctx, app, "writer.reviewRevisions", { action: "reject", ids: ["1:1", "9:9"] }, /unknown revision ids: 9:9/);
  assert.equal(writerStateOf(app).text, "một.\r");
  assert.equal(writerReviewOf(app).revisions.length, 2);
});
