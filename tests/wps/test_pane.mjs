// Spec T4b (docs/architecture/wps-bridge.md muc 7): pane Ask AI trong WPS - logic khong DOM + nut ribbon. Khong can WPS.
//
//   node --test tests/wps/test_pane.mjs
//
// Hop dong:
// - src/AxiomOffice.WPS/pane-core.js (ES5, nhu probe.js) dinh nghia global AxiomPaneCore.create(options):
//     options = { fetch, EventSource, base: "<...>/wps/v1/", onChange(state) }
//     state   = { messages: [{role: "user"|"assistant"|"error", text}], running, steps: [{action, ok}],
//                 confirmation: null | {id, action, reason}, conversationId }
//   controller.send(prompt)   -> POST base+"runs" {prompt, conversationId?} roi mo EventSource(base+"runs/<id>/events");
//                                dang chay thi bo qua (tra false). 409 (add-in chua ket noi) -> message "error" tieng
//                                Viet co cum "Mở một tài liệu".
//   controller.confirm(ok)    -> POST base+"runs/<id>/confirm" {confirmationId, approved: ok}
//   controller.stop()         -> POST base+"runs/<id>/cancel"
//   controller.newConversation() -> xoa conversationId va messages.
//   Su kien SSE: tool.started -> steps += {action}; tool.finished -> step.ok; confirm.required -> confirmation;
//   confirm.resolved -> confirmation = null; run.completed -> message assistant = reply, running = false, dong
//   EventSource; run.failed|run.cancelled|run.stopped|run.timedout -> message "error", running = false.
// - ribbon.xml co nut id="axiomAskAI" (label chua "Ask AI", onAction="ribbon.OnAction"); main.js: OnAction voi
//   control.Id == "axiomAskAI" goi Application.CreateTaskPane(<url tuyet doi .../wps/addin/pane.html?app=<kind>>,
//   kind = app dang chay de pane goi y dung loai tai lieu - xem test_pane_theme.mjs) roi
//   Visible = true; bam lan nua thi dung lai pane cu (khong tao them), Visible = true.
// - pane.html nap pane-core.js.
import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";
import { loCatalog } from "./addin_harness.mjs";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
const ADDIN = path.join(ROOT, "src", "AxiomOffice.WPS");
const BASE = "http://127.0.0.1:47840/wps/v1/";

function load(files, extra = {}) {
  const context = { console, setTimeout, clearTimeout, setInterval, clearInterval, Promise, JSON, ...extra };
  context.window = context;
  context.globalThis = context;
  vm.createContext(context);
  for (const file of files) {
    vm.runInContext(fs.readFileSync(path.join(ADDIN, file), "utf8"), context, { filename: file });
  }
  return context;
}

class FakeSource {
  constructor(url) { this.url = url; this.handlers = {}; this.closed = false; FakeSource.last = this; FakeSource.all.push(this); }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  close() { this.closed = true; }
  emit(type, data) { for (const fn of this.handlers[type] || []) fn({ type, data: JSON.stringify(data) }); }
}
FakeSource.all = [];

function harness(responses = {}, fetchOverride = null, kind = "wps") {
  const calls = [];
  const defaultFetch = (url, options = {}) => {
    const body = options.body ? JSON.parse(options.body) : null;
    calls.push({ url: String(url), method: options.method || "GET", body });
    const key = Object.keys(responses).find((k) => String(url).endsWith(k));
    const [status, payload] = key ? responses[key] : [200, { ok: true, result: {} }];
    return Promise.resolve({ ok: status < 400, status, json: () => Promise.resolve(payload),
      text: () => Promise.resolve(JSON.stringify(payload)) });
  };
  const fetch = fetchOverride || defaultFetch;
  const ctx = load(["pane-core.js"]);
  assert.equal(typeof ctx.AxiomPaneCore, "object", "thieu global AxiomPaneCore (src/AxiomOffice.WPS/pane-core.js)");
  let state = null;
  const pane = ctx.AxiomPaneCore.create({ fetch, EventSource: FakeSource, base: BASE, kind,
    onChange: (s) => { state = JSON.parse(JSON.stringify(s)); } });
  return { pane, calls, ctx, get state() { return state; } };
}

const tick = () => new Promise((resolve) => setTimeout(resolve, 10));

test("send -> POST runs, EventSource, steps, run.completed", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r1", conversationId: "c1", status: "running" } }] });
  assert.notEqual(await h.pane.send("Thêm dòng chào"), false);
  await tick();
  assert.equal(h.calls[0].url, BASE + "runs");
  assert.equal(h.calls[0].method, "POST");
  assert.equal(h.calls[0].body.prompt, "Thêm dòng chào");
  assert.equal(FakeSource.last.url, BASE + "runs/r1/events");
  assert.equal(h.state.running, true);
  assert.deepEqual(h.state.messages[0], { role: "user", text: "Thêm dòng chào" });
  assert.equal(await h.pane.send("lần hai khi đang chạy"), false, "dang chay thi bo qua");
  FakeSource.last.emit("tool.started", { data: { callId: "k1", action: "writer.appendText" } });
  FakeSource.last.emit("tool.finished", { data: { callId: "k1", action: "writer.appendText", ok: true } });
  FakeSource.last.emit("run.completed", { data: { reply: "Đã thêm dòng" } });
  await tick();
  assert.equal(h.state.running, false);
  assert.equal(h.state.steps[0].action, "writer.appendText");
  assert.equal(h.state.steps[0].ok, true);
  assert.deepEqual(h.state.messages[h.state.messages.length - 1], { role: "assistant", text: "Đã thêm dòng" });
  assert.equal(FakeSource.last.closed, true, "xong luot thi dong EventSource");
  assert.equal(h.state.conversationId, "c1");
  await h.pane.send("Tiếp");
  await tick();
  assert.equal(h.calls[h.calls.length - 1].body.conversationId, "c1", "luot sau gui conversationId");
});

test("409 add-in chua ket noi -> loi tieng Viet, khong chay", async () => {
  const h = harness({ "/runs": [409, { ok: false, error: "WPS add-in is not connected" }] });
  await h.pane.send("Xin chào");
  await tick();
  assert.equal(h.state.running, false);
  const last = h.state.messages[h.state.messages.length - 1];
  assert.equal(last.role, "error");
  assert.match(last.text, /Mở một tài liệu/);
  assert.doesNotMatch(last.text, /Writer/);
});

test("the xac nhan: confirm.required -> confirm(false) POST /confirm; confirm.resolved xoa the", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r2", conversationId: "c2" } }] });
  await h.pane.send("Sửa chính tả");
  await tick();
  FakeSource.last.emit("confirm.required", { data: { confirmationId: "cf_1", action: "writer.save", reason: "AI muốn lưu" } });
  await tick();
  assert.deepEqual(h.state.confirmation, { id: "cf_1", action: "writer.save", reason: "AI muốn lưu" });
  await h.pane.confirm(false);
  const call = h.calls.find((c) => c.url === BASE + "runs/r2/confirm");
  assert.ok(call, "phai POST runs/<id>/confirm");
  assert.deepEqual(call.body, { confirmationId: "cf_1", approved: false });
  FakeSource.last.emit("confirm.resolved", { data: { confirmationId: "cf_1", approved: false } });
  await tick();
  assert.equal(h.state.confirmation, null);
});

test("the nhieu lua chon: choices -> state; choose(id) POST {confirmationId, choice}", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r9", conversationId: "c9" } }] });
  await h.pane.send("Trình bày lại giáo án");
  await tick();
  FakeSource.last.emit("confirm.required", { data: { confirmationId: "cf_9", action: "doc_design", reason: "Kiểu nào?",
    choices: [{ id: "full", label: "Có khối", detail: "Định dạng + khối" }, { id: "keep", label: "Giữ kiểu" }] } });
  await tick();
  assert.deepEqual(h.state.confirmation.choices, [
    { id: "full", label: "Có khối", detail: "Định dạng + khối" }, { id: "keep", label: "Giữ kiểu", detail: "" }]);
  await h.pane.choose("keep");
  const call = h.calls.find((c) => c.url === BASE + "runs/r9/confirm");
  assert.ok(call, "phai POST runs/<id>/confirm");
  assert.deepEqual(call.body, { confirmationId: "cf_9", choice: "keep" });
});

test("the nhieu lua chon: o tu go -> choose(id, text) gui kem chu", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r10", conversationId: "c10" } }] });
  await h.pane.send("Trình bày lại giáo án");
  await tick();
  FakeSource.last.emit("confirm.required", { data: { confirmationId: "cf_10", action: "doc_design", reason: "Kiểu nào?",
    choices: [{ id: "full", label: "Có khối" }, { id: "custom", label: "Kiểu khác (tự gõ)", detail: "Ví dụ…", input: true }] } });
  await tick();
  assert.equal(h.state.confirmation.choices[1].input, true);
  await h.pane.choose("custom", "  tiêu đề xanh lá  ");
  const call = h.calls.find((c) => c.url === BASE + "runs/r10/confirm");
  assert.deepEqual(call.body, { confirmationId: "cf_10", choice: "custom", text: "tiêu đề xanh lá" });
});

test("stop -> POST /cancel; run.cancelled -> het chay + loi", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r3", conversationId: "c3" } }] });
  await h.pane.send("Làm lâu");
  await tick();
  await h.pane.stop();
  assert.ok(h.calls.find((c) => c.url === BASE + "runs/r3/cancel" && c.method === "POST"), "phai POST runs/<id>/cancel");
  FakeSource.last.emit("run.cancelled", { data: { seconds: 1 } });
  await tick();
  assert.equal(h.state.running, false);
  assert.equal(h.state.messages[h.state.messages.length - 1].role, "error");
});

test("newConversation xoa hoi thoai", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r4", conversationId: "c4" } }] });
  await h.pane.send("A");
  await tick();
  FakeSource.last.emit("run.completed", { data: { reply: "B" } });
  await tick();
  h.pane.newConversation();
  await tick();
  assert.equal(h.state.conversationId, null);
  assert.equal(h.state.messages.length, 0);
});

test("ribbon co nut Ask AI; bam mo task pane pane.html va dung lai lan sau", () => {
  const ribbon = fs.readFileSync(path.join(ADDIN, "ribbon.xml"), "utf8");
  assert.match(ribbon, /id="axiomAskAI"[^>]*label="[^"]*Ask AI/, "ribbon.xml thieu nut axiomAskAI 'Ask AI'");
  const created = [];
  const Application = {
    CreateTaskPane(url) { const pane = { url, Visible: false }; created.push(pane); return pane; },
  };
  const ctx = load(["probe.js", "commands.js", "main.js"], {
    Application, location: { href: "http://127.0.0.1:47840/wps/addin/index.html" },
    fetch: () => new Promise(() => {}),
  });
  ctx.ribbon.OnAction({ Id: "axiomAskAI" });
  assert.equal(created.length, 1, "phai goi Application.CreateTaskPane");
  // Application gia khong co Name -> appKind mac dinh "wps" (xem AxiomCommands.appKind).
  // ?client=<id cua ban add-in nay>: pane gui lai cho Core de chay tren DUNG app khi WPS mo nhieu app.
  const paneClient = /^http:\/\/127\.0\.0\.1:47840\/wps\/addin\/pane\.html\?app=wps&client=(c[0-9a-z]+)$/.exec(created[0].url);
  assert.ok(paneClient, "url pane: " + created[0].url);
  assert.equal(created[0].Visible, true);
  created[0].Visible = false;
  ctx.ribbon.OnAction({ Id: "axiomAskAI" });
  assert.equal(created.length, 1, "bam lan nua dung lai pane cu");
  assert.equal(created[0].Visible, true);
  if (ctx.AxiomBridge) ctx.AxiomBridge.stop();
});

test("pane.html nap pane-core.js", () => {
  const html = fs.readFileSync(path.join(ADDIN, "pane.html"), "utf8");
  assert.match(html, /pane-core\.js/);
});

test("Core tat (fetch bi tu choi) -> loi noi ro phai kiem tra gi", async () => {
  const failingFetch = () => Promise.reject(new Error("connect refused"));
  const h = harness({}, failingFetch);
  await h.pane.send("Xin chao Core oi");
  await tick();
  assert.equal(h.state.running, false);
  const last = h.state.messages[h.state.messages.length - 1];
  assert.equal(last.role, "error");
  assert.match(last.text, /Agent Core/);
  assert.match(last.text, /kiểm tra|khởi động|chạy/i);
});

// ---------------------------------------------------------------- ngang ban Windows (dot B)

test("run.started/run.progress: model + dong 'dang lam gi'", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r20", conversationId: "c20" } }] });
  await h.pane.send("Soạn thư mời");
  await tick();
  FakeSource.last.emit("run.started", { data: { conversationId: "c20", model: "claude-sonnet-5-5", app: "wps", mode: "act" } });
  await tick();
  assert.equal(h.state.model, "claude-sonnet-5-5");
  assert.equal(h.state.mode, "act");
  assert.equal(h.state.status, "Đang chạy · claude-sonnet-5-5", "footer noi dang chay bang model nao");
  FakeSource.last.emit("run.progress", { data: { round: 2, text: "Đang thiết kế phần 1–15: 7 slide…" } });
  await tick();
  assert.equal(h.state.progress, "Đang thiết kế phần 1–15: 7 slide…");
  FakeSource.last.emit("run.completed", { data: { reply: "Xong", seconds: 12.4, rounds: 3, model: "claude-sonnet-5-5" } });
  await tick();
  assert.equal(h.state.progress, "", "xong luot thi bo dong 'dang lam gi'");
});

test("suy luan: MOT muc cho ca luot, dai thi thu lai, toggle mo ra", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r21" } }] });
  await h.pane.send("Sửa giúp tôi");
  await tick();
  FakeSource.last.emit("run.reasoning", { data: { round: 1, text: "Ngắn gọn." } });
  await tick();
  assert.deepEqual(h.state.reasoning, { round: 1, text: "Ngắn gọn.", live: false, expanded: true },
    "suy luan ngan thi hien thang");
  const long = "x".repeat(400);
  FakeSource.last.emit("run.reasoning", { data: { round: 2, text: long } });
  await tick();
  assert.equal(h.state.reasoning.round, 2);
  assert.equal(h.state.reasoning.text, long);
  assert.equal(h.state.reasoning.expanded, false, "suy luan dai hon 320 ky tu phai thu lai");
  h.pane.toggleReasoning();
  await tick();
  assert.equal(h.state.reasoning.expanded, true, "bam 'Hiện' thi mo ra");
  FakeSource.last.emit("run.reasoning.live", { data: { round: 3, text: "…đang nghĩ tiếp" } });
  await tick();
  assert.equal(h.state.reasoning.round, 3);
  assert.equal(h.state.reasoning.live, true);
  assert.equal(h.state.reasoning.expanded, false, "ban live chi la phan duoi -> luon thu lai");
});

test("thao tac: dem lenh SUA tai lieu, load_skill/remember thanh dong rieng", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r22" } }] });
  await h.pane.send("Trình bày lại");
  await tick();
  const src = FakeSource.last;
  src.emit("tool.started", { data: { callId: "k1", action: "writer.getText" } });
  src.emit("tool.finished", { data: { callId: "k1", action: "writer.getText", ok: true } });
  src.emit("tool.started", { data: { callId: "k2", action: "writer.appendText" } });
  src.emit("tool.finished", { data: { callId: "k2", action: "writer.appendText", ok: true } });
  src.emit("tool.started", { data: { callId: "k3", action: "writer.save" } });
  src.emit("tool.finished", { data: { callId: "k3", action: "writer.save", ok: true } });
  src.emit("tool.started", { data: { callId: "k4", action: "writer.insertTable" } });
  src.emit("tool.finished", { data: { callId: "k4", action: "writer.insertTable", ok: false, error: "ArgumentException: bang qua rong" } });
  src.emit("tool.started", { data: { callId: "k5", action: "load_skill" } });
  src.emit("tool.finished", { data: { callId: "k5", tool: "load_skill", ok: true } });
  src.emit("skill.loaded", { data: { name: "soan-thao", source: "user" } });
  src.emit("tool.started", { data: { callId: "k6", action: "remember" } });
  src.emit("tool.finished", { data: { callId: "k6", tool: "remember", ok: true } });
  src.emit("memory.written", { data: { id: "m1", event: "ADD", text: "Người dùng thích văn phong ngắn" } });
  await tick();
  assert.equal(h.state.edits, 1, "chi writer.appendText la thao tac sua (getText/save/doc lai)");
  assert.deepEqual(h.state.steps.map((s) => s.action), ["writer.getText", "writer.appendText", "writer.save", "writer.insertTable"],
    "load_skill/remember co dong rieng -> khong hien hai lan");
  assert.equal(h.state.steps[3].ok, false);
  assert.equal(h.state.steps[3].error, "ArgumentException: bang qua rong");
  assert.deepEqual(h.state.skills, [{ name: "soan-thao" }]);
  assert.deepEqual(h.state.notes, [{ id: "m1", text: "Người dùng thích văn phong ngắn", state: "saved" }]);
});

test("footer: 'Xong trong Ns · N thao tác' va loi thi chi ve Cai dat", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r23" } }] });
  const core = h.ctx.AxiomPaneCore;
  await h.pane.send("Sửa giúp");
  await tick();
  FakeSource.last.emit("tool.finished", { data: { callId: "k1", action: "writer.appendText", ok: true } });
  FakeSource.last.emit("run.completed", { data: { reply: "Rồi", seconds: 7.8, rounds: 2 } });
  await tick();
  assert.equal(core.summary(h.state), "Xong trong 7s · 1 thao tác");
  FakeSource.last.emit("run.failed", { data: { error: "hết hạn mức" } });
  await tick();
  assert.equal(core.summary(h.state), "Lỗi · nhấn Cài đặt để kiểm tra Agent Core");
  const h2 = harness({ "/runs": [200, { ok: true, result: { runId: "r24" } }] });
  assert.equal(h2.ctx.AxiomPaneCore.summary(h2.state), "Sẵn sàng", "pane moi mo thi bao San sang");
  await h2.pane.send("A");
  await tick();
  assert.equal(h2.ctx.AxiomPaneCore.summary(h2.state), "Đang chạy…", "dang chay thi bao dang chay");
});

test("Hoan tac luot nay: exec <kind>.undo {count}, xong thi thoi dem", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r25" } }], "/exec": [200, { ok: true, result: { undone: 2 } }] });
  await h.pane.send("Trình bày lại");
  await tick();
  assert.equal(h.state.canUndo, false, "chua sua gi thi chua co nut");
  FakeSource.last.emit("tool.finished", { data: { callId: "k1", action: "writer.appendText", ok: true } });
  FakeSource.last.emit("tool.finished", { data: { callId: "k2", action: "writer.insertTable", ok: true } });
  FakeSource.last.emit("run.completed", { data: { reply: "Xong", seconds: 3 } });
  await tick();
  assert.equal(h.state.canUndo, true);
  await h.pane.undoLastTurn();
  const call = h.calls.find((c) => c.url === BASE + "exec");
  assert.ok(call, "phai POST /wps/v1/exec");
  assert.deepEqual(call.body, { action: "writer.undo", params: { count: 2 }, app: "wps" });
  assert.equal(h.state.edits, 0);
  assert.equal(h.state.canUndo, false, "hoan tac roi thi thoi hien nut");
  const last = h.state.messages[h.state.messages.length - 1];
  assert.equal(last.role, "info");
  assert.match(last.text, /Đã hoàn tác 2 thao tác/);
});

test("Hoan tac luot nay: dung undoSteps cua Core (da tru lan agent tu undo), khong tu dem", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r26" } }], "/exec": [200, { ok: true, result: { undone: 1 } }] });
  await h.pane.send("Viết lại");
  await tick();
  FakeSource.last.emit("tool.finished", { data: { callId: "k1", action: "writer.appendText", ok: true } });
  FakeSource.last.emit("tool.finished", { data: { callId: "k2", action: "writer.appendText", ok: true } });
  FakeSource.last.emit("tool.finished", { data: { callId: "k3", action: "writer.undo", ok: true } });
  FakeSource.last.emit("run.completed", { data: { reply: "Xong", seconds: 3, undoSteps: 1 } });
  await tick();
  assert.equal(h.state.edits, 1, "Core bao con 1 buoc cua luot - khong lui vao noi dung co tu truoc");
  await h.pane.undoLastTurn();
  const call = h.calls.find((c) => c.url === BASE + "exec");
  assert.deepEqual(call.body, { action: "writer.undo", params: { count: 1 }, app: "wps" });
});

test("Hoan tac (bang tinh): noi ro thao tac WPS chua hoan tac duoc thay vi im lang", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r25b" } }],
    "/exec": [200, { ok: true, result: { undone: 1, restored: [{}], skipped: ["et.formatRange"] } }] }, null, "et");
  await h.pane.send("Tô đậm tiêu đề");
  await tick();
  FakeSource.last.emit("tool.finished", { data: { callId: "k1", action: "et.writeRange", ok: true } });
  FakeSource.last.emit("tool.finished", { data: { callId: "k2", action: "et.formatRange", ok: true } });
  FakeSource.last.emit("run.completed", { data: { reply: "Xong", seconds: 2 } });
  await tick();
  await h.pane.undoLastTurn();
  assert.deepEqual(h.calls.find((c) => c.url === BASE + "exec").body, { action: "et.undo", params: { count: 2 }, app: "et" });
  const last = h.state.messages[h.state.messages.length - 1];
  assert.match(last.text, /Đã hoàn tác 1 thao tác/);
  assert.match(last.text, /1 thao tác chưa hoàn tác được/);
});

test("Hoan tac: PowerPoint khong co wpp.undo nen khong hien nut", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r26" } }] }, null, "wpp");
  await h.pane.send("Thêm slide");
  await tick();
  FakeSource.last.emit("tool.finished", { data: { callId: "k1", action: "wpp.addSlide", ok: true } });
  FakeSource.last.emit("run.completed", { data: { reply: "Xong", seconds: 2 } });
  await tick();
  assert.equal(h.state.edits, 1);
  assert.equal(h.state.canUndo, false, "chua co lenh hoan tac cho wpp");
  assert.equal(await h.pane.undoLastTurn(), false);
  assert.equal(h.calls.filter((c) => c.url === BASE + "exec").length, 0);
});

test("Chen tra loi: exec writer.insertStyledText {text} (chi o Word)", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r27" } }], "/exec": [200, { ok: true, result: {} }] });
  await h.pane.send("Viết giúp tôi");
  await tick();
  FakeSource.last.emit("run.completed", { data: { reply: "**Kính gửi** anh", seconds: 2 } });
  await tick();
  assert.equal(h.state.canInsert, true);
  await h.pane.insertReply();
  const call = h.calls.find((c) => c.url === BASE + "exec");
  assert.deepEqual(call.body, { action: "writer.insertStyledText", params: { text: "**Kính gửi** anh" }, app: "wps" });
  assert.equal(h.state.status, "Đã chèn vào tài liệu");
  const excel = harness({}, null, "et");
  await excel.pane.send("A");
  await tick();
  FakeSource.last.emit("run.completed", { data: { reply: "x" } });
  await tick();
  assert.equal(excel.state.canInsert, false, "Excel khong co 'Chen tra loi'");
});

test("Che do ke hoach: options.mode=plan, the ke hoach, 'Thuc hien ke hoach'", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r28" } }] });
  h.pane.setPlan(true);
  assert.equal(h.state.plan, true);
  await h.pane.send("Dọn lại bố cục giáo án");
  await tick();
  assert.deepEqual(h.calls[0].body.options, { mode: "plan" }, "luot ke hoach gui options.mode=plan");
  FakeSource.last.emit("run.started", { data: { model: "m", mode: "plan" } });
  FakeSource.last.emit("run.completed", { data: { reply: "1. Đổi tiêu đề\n2. Kẻ bảng", seconds: 5 } });
  await tick();
  assert.equal(h.state.planReady, true);
  assert.equal(h.state.status, "Đã lập kế hoạch trong 5s · chưa sửa gì");
  await h.pane.executePlan();
  await tick();
  const last = h.calls[h.calls.length - 1];
  assert.equal(last.url, BASE + "runs");
  assert.equal(last.body.options, undefined, "chay that thi khong con che do ke hoach");
  assert.match(last.body.prompt, /^Thực hiện kế hoạch ở trên\./);
  assert.equal(h.state.planReady, false, "the ke hoach chi chay mot lan");
  assert.equal(h.state.plan, false);
});

test("Xoa ghi nho vua ghi: DELETE /memory/<id>", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r29" } }], "/memory/m1": [200, { ok: true, result: { deleted: "m1" } }] });
  await h.pane.send("Nhớ giúp tôi");
  await tick();
  FakeSource.last.emit("memory.written", { data: { id: "m1", text: "Thích văn phong ngắn" } });
  await tick();
  assert.equal(await h.pane.forget("m1"), true);
  const call = h.calls.find((c) => c.url === BASE + "memory/m1");
  assert.equal(call.method, "DELETE");
  assert.equal(h.state.notes[0].state, "deleted");
});

test("pane-core: isEdit/reasoningPreview/errorSummary giong chat.py + theme.py", () => {
  const core = harness({}).ctx.AxiomPaneCore;
  for (const action of ["writer.getText", "writer.selection", "writer.checkTables", "writer.save", "writer.saveAs",
    "writer.exportPdf", "writer.undo", "et.readRange", "et.listSheets", "et.checkRange", "wpp.listSlides",
    "wpp.checkLayout", "wpp.save", "load_skill", "remember", ""]) {
    assert.equal(core.isEdit(action), false, "khong phai thao tac sua: " + action);
  }
  for (const action of ["writer.appendText", "writer.replaceAll", "writer.insertTable", "writer.restyle",
    "et.writeRange", "et.formatRange", "wpp.addSlide", "wpp.addText"]) {
    assert.equal(core.isEdit(action), true, "la thao tac sua: " + action);
  }
  // Lenh khong tao buoc Undo (doc/mo/dong/khoa) khong duoc tinh: dem nham -> "Hoan tac luot nay" go qua tay.
  for (const action of ["writer.outline", "writer.open", "writer.lockSelection", "et.sheetInfo", "et.listCharts",
    "et.listConditionalFormats", "et.closeAll", "wpp.sourceManifest"]) {
    assert.equal(core.isEdit(action), false, "khong tao buoc Undo: " + action);
  }
  // Mot nguon: isEdit == co undo cua registry LibreOffice (sinh vao pane-core.js boi generate_mcp_commands.py).
  for (const item of loCatalog()) {
    assert.equal(core.isEdit(item.name), !!item.undo, "isEdit khop co undo: " + item.name);
  }
  assert.equal(core.reasoningPreview("  một   hai ba  "), "một hai ba");
  assert.equal(core.reasoningPreview("x".repeat(250), 200), "x".repeat(199) + "…");
  assert.equal(core.errorSummary("ArgumentException: bảng quá rộng"), "bảng quá rộng");
  assert.equal(core.errorSummary(""), "lỗi");
  assert.equal(core.REASONING_INLINE_LIMIT, 320, "cung nguong voi chat.py");
  assert.match(core.PLAN_EXECUTE_PROMPT, /^Thực hiện kế hoạch ở trên\./);
});

test("SSE dut giua luot -> khong im lang", async () => {
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "r9", conversationId: "c9" } }] });
  await h.pane.send("Lam viec lau");
  await tick();
  assert.equal(h.state.running, true);
  FakeSource.last.emit("error", {});
  await tick();
  assert.equal(h.state.running, false);
  const last = h.state.messages[h.state.messages.length - 1];
  assert.equal(last.role, "error");
  assert.match(last.text, /Agent Core/);
});

// ---- Icon ribbon: ribbon.xml va images/ phai khop nhau tung tag ----
// Ban Windows ve icon bang imageMso=; ban WPS tra ve duong dan tuong doi "images/<tag>.svg" tu getImage
// (khong phai data URI - WPS dem chuoi do di phan giai nhu mot duong dan, truot thi ve dau "?").
// Hai ben phai di cung nhau: them mot nut ma quen ve anh, hoac de lai mot tep anh khong nut nao dung,
// deu la spec do ngay.

// ribbon.xml gom hai phan: <ribbon> (tab Axiom Office) va <contextMenus> (menu chuot phai). part = "ribbon" | "context".
function ribbonControls(part = "ribbon") {
  const all = fs.readFileSync(path.join(ADDIN, "ribbon.xml"), "utf8").split("-->").slice(1).join("-->");
  const cut = all.indexOf("<contextMenus>");
  const xml = part === "ribbon" ? (cut >= 0 ? all.slice(0, cut) : all) : (cut >= 0 ? all.slice(cut) : "");
  const out = [];
  for (const found of xml.matchAll(/<(button|menu|toggleButton|dropDown|dynamicMenu|menuSeparator|contextMenu)\s([^>]*)>/g)) {
    const attrs = found[2];
    const one = (name) => { const m = new RegExp("\\b" + name + '="([^"]*)"').exec(attrs); return m ? m[1] : ""; };
    out.push({ kind: found[1], id: one("id"), tag: one("tag"), getImage: one("getImage"), label: one("label"),
      onAction: one("onAction"), idMso: one("idMso"), imageMso: one("imageMso") });
  }
  return part === "ribbon" ? out.filter((c) => c.kind !== "menuSeparator") : out;
}

function iconFiles() {
  const dir = path.join(ADDIN, "images");
  assert.ok(fs.existsSync(dir), "thieu thu muc src/AxiomOffice.WPS/images");
  return fs.readdirSync(dir).filter((n) => n.endsWith(".svg")).map((n) => n.slice(0, -4)).sort();
}

// Icon nut ribbon = imageMso, bo icon co san cua WPS (do tren WPS 12.1.2 that 20/10/2026: hien dung icon). WPS tra
// ten kieu Office qua bang lenh cua no (/opt/kingsoft/wps-office/office6/res/commands/<wps|et|wpp>/ongmani/*.kuip:
// idMso="..." icon="..._kd"); ten khong co trong bang thi khong co icon. Bang duoi la DUNG bo ten da do - doi ten
// nao thi phai tra lai bang lenh cua WPS (docs/architecture/ribbon-ui.md muc 5.3) va thu lai tren WPS that.
const RIBBON_IMAGE_MSO = {
  axiomAskAI: "Help", // ca ba app (question_kd)
  axiomCheckLayout: "SelectionPane", // chi trinh chieu; SlideShowFromCurrent ra "?" tren WPP that (01/11/2026)
  axiomCheckTables: "TableInsertDialogWord", // chi van ban (TableInsert khong co trong Writer cua WPS)
  axiomCheckRange: "DataValidation", // chi bang tinh
  // Do 01/11/2026: ten co trong bang lenh van co the ra dau "?" (SpellingMenu, TableAutoFitWindow, BordersAll,
  // FieldCheckBox, StylesPane...) - chi dung ten DA THAY icon tren WPS that (tests/wps/REFERENCE.md).
  axiomProofread: "ReviewTrackChanges", // chi van ban (but sua - lenh sua bang Track Changes)
  axiomTableTidy: "TableAutoFitContents", // chi van ban
  axiomMemory: "FileSave", // ca ba app; ban Windows dung RecordsSaveRecord - WPS khong co
  axiomSkills: "AddInManager", // da thay icon o ca ba app (01/11/2026); JsAddInsDialog ra "?"
  axiomSetup: "ApplicationOptionsDialog", // ca ba app; ban Windows dung ControlWizards - WPS khong co
  axiomStatus: "About", // ca ba app; ban Windows dung Info - WPS khong co
  axiomLog: "FileOpen", // ca ba app
  axiomMoreMenu: "FileProperties", // ca ba app; ban Windows dung PropertySheet - WPS khong co
  axiomTestModel: "ReviewAcceptChange", // da thay icon o ca ba app (01/11/2026); ReviewNewComment ra "?"
  axiomCopyUrl: "Copy", // ca ba app
};

// Nut AI (nhom Tao moi + Viet voi AI, 01/11/2026): imageMso hop nghia deu ra "?" tren WPS that (do 32 ten), nen dung bo
// SVG rieng theo phong cach icon net cua tab WPS AI - getImage, KHONG imageMso. Bon muc con cua menu Doi giong: khong icon.
const RIBBON_AI = ["axiomSmartCreate", "axiomSlidesFromDoc", "axiomRb_polish", "axiomRb_rewrite", "axiomRb_continue",
  "axiomRb_expand", "axiomRb_shorten", "axiomRb_fix", "axiomRbTone", "axiomRb_bilingual", "axiomRb_translate",
  "axiomRb_summarize", "axiomRb_explain"];
const RIBBON_TONE = ["axiomRb_formal", "axiomRb_concise", "axiomRb_plain", "axiomRb_positive"];
const iconName = (tag) => tag.replace(":", "_");

test("ribbon.xml: nut cu dung imageMso dung bang da do; nut AI dung getImage + SVG rieng; moi nut MOT nguon icon", () => {
  const controls = ribbonControls();
  assert.deepEqual(controls.map((c) => c.id).sort(), [...Object.keys(RIBBON_IMAGE_MSO), ...RIBBON_AI, ...RIBBON_TONE].sort(),
    "them / bot nut ribbon thi phai chon icon (ten co trong bang lenh cua WPS, hoac SVG rieng cho nut AI)");
  for (const control of controls) {
    if (RIBBON_AI.includes(control.id)) {
      assert.equal(control.imageMso, "", "#" + control.id + ": nut AI khong dung imageMso");
      assert.equal(control.getImage, "ribbon.OnGetImage", "#" + control.id + ": nut AI dung getImage");
    } else if (RIBBON_TONE.includes(control.id)) {
      assert.deepEqual([control.imageMso, control.getImage], ["", ""], "#" + control.id + ": muc menu con khong icon");
    } else {
      assert.equal(control.imageMso, RIBBON_IMAGE_MSO[control.id], "#" + control.id + ": imageMso");
      assert.equal(control.getImage, "", "#" + control.id + ": khong kem getImage (hai nguon icon tren mot nut)");
    }
  }
});

test("ribbon + menu chuot phai: control co getImage <-> tep images/<tag ':'->'_'>.svg khop hai chieu, va khop ICON_FILES", () => {
  const withImage = [...ribbonControls(), ...ribbonControls("context")].filter((c) => c.getImage);
  assert.ok(withImage.length >= 2 + RIBBON_AI.length, "menu chuot phai (Sua bang AI, Hoi AI) + nut AI tren ribbon co icon");
  const files = iconFiles();
  for (const control of withImage) {
    assert.equal(control.getImage, "ribbon.OnGetImage");
    assert.ok(files.indexOf(iconName(control.tag)) >= 0, "images/: thieu tep cho tag '" + control.tag + "' (#" + control.id + ")");
  }
  const tags = withImage.map((c) => iconName(c.tag));
  for (const name of files) {
    assert.ok(tags.indexOf(name) >= 0, "images/: co '" + name + ".svg' nhung khong control nao dung");
  }
  const main = fs.readFileSync(path.join(ADDIN, "main.js"), "utf8");
  const known = /var ICON_FILES = \{([^}]*)\}/.exec(main);
  assert.ok(known, "main.js phai khai ICON_FILES");
  assert.deepEqual([...known[1].matchAll(/(\w+):/g)].map((m) => m[1]).sort(), files,
    "ICON_FILES cua main.js phai dung bang danh sach tep trong images/");
});

test("OnGetImage: tra duong dan tuong doi images/<tag>.svg; khong biet tag thi tra chuoi rong", () => {
  const accent = "#0F6CBD"; // theme.py ACCENT - mau cua he thiet ke pane
  const ctx = load(["probe.js", "commands.js", "main.js"], {
    Application: { CreateTaskPane: () => ({ Visible: false }) },
  });
  const rel = ctx.ribbon.OnGetImage({ Id: "axiomCtxAsk_t" });
  assert.equal(rel, "images/askAi.svg", "getImage phai tra duong dan tuong doi, khong phai data URI");
  // WPS co ban chi dien Tag (khong dien Id) - hai duong phai ra cung mot duong dan.
  assert.equal(ctx.ribbon.OnGetImage({ Tag: "askAi" }), rel);
  // Nut ribbon da chuyen sang imageMso: khong co tep -> chuoi rong, KHONG phai duong dan 404 (WPS ve "?").
  assert.equal(ctx.ribbon.OnGetImage({ Id: "axiomSetup" }), "");
  // Nut AI tren ribbon: tag "quick:polish" -> tep quick_polish.svg (':' khong dung trong ten tep); WPS chi dien Id thi suy
  // tag tu Id (axiomRb_<muc> -> quick:<muc>).
  assert.equal(ctx.ribbon.OnGetImage({ Tag: "quick:polish" }), "images/quick_polish.svg");
  assert.equal(ctx.ribbon.OnGetImage({ Id: "axiomRb_polish" }), "images/quick_polish.svg");
  assert.equal(ctx.ribbon.OnGetImage({ Id: "axiomSmartCreate" }), "images/doc_smartCreate.svg");
  assert.equal(ctx.ribbon.OnGetImage({ Id: "axiomRbTone" }), "images/toneMenu.svg");
  assert.equal(ctx.ribbon.OnGetImage({ Tag: "doc:proofread" }), "", "nut doc:<id> dung imageMso thi khong co tep");
  // Control la thi tra chuoi rong: WPS ve nut khong anh (dung nhu ban cu), khong lam hong ribbon.
  // Tra ve mot duong dan 404 thi WPS con te hon - no ve dau "?" (dung cai da gap voi data URI).
  assert.equal(ctx.ribbon.OnGetImage({ Id: "khong-co-that" }), "");
  assert.equal(ctx.ribbon.OnGetImage(null), "");
  // Moi tep anh phai la MOT the <svg> khap kin, chi viewBox (de WPS tu co gian theo co nut: nut lon
  // 32px, nut nho 16px - y nhu tep mau cua SDK @wps-jsapi/wps/images/1.svg), va dung mau accent.
  for (const control of ribbonControls("context").filter((c) => c.getImage)) {
    assert.equal(ctx.ribbon.OnGetImage({ Tag: control.tag }), "images/" + control.tag + ".svg");
    const text = fs.readFileSync(path.join(ADDIN, "images", control.tag + ".svg"), "utf8");
    assert.match(text, /<svg[^>]*viewBox="0 0 20 20"[^>]*>[\s\S]*<\/svg>\s*$/,
      "anh '" + control.tag + "' phai la mot the <svg> khap kin voi viewBox 0 0 20 20");
    // Chi the <svg> mo dau: than icon co <rect width=... height=...> la chuyen binh thuong.
    const open = /<svg[^>]*>/.exec(text)[0];
    assert.ok(!/\swidth=/.test(open) && !/\sheight=/.test(open),
      "the <svg> cua anh '" + control.tag + "' khong duoc co width/height - WPS tu co gian");
    assert.ok(text.indexOf('stroke="' + accent + '"') >= 0,
      "anh '" + control.tag + "' phai dung mau accent cua he thiet ke pane (theme.py ACCENT)");
    assert.equal((text.match(/</g) || []).length, (text.match(/>/g) || []).length,
      "anh '" + control.tag + "' lech the mo/dong");
  }
  // Icon nut AI tren ribbon: phong cach tab WPS AI - viewBox 24, net xam dam #3B3B3B, diem nhan mau accent.
  for (const control of ribbonControls().filter((c) => c.getImage)) {
    const text = fs.readFileSync(path.join(ADDIN, "images", iconName(control.tag) + ".svg"), "utf8");
    assert.match(text, /<svg[^>]*viewBox="0 0 24 24"[^>]*>[\s\S]*<\/svg>\s*$/, iconName(control.tag) + ": viewBox 0 0 24 24");
    const open = /<svg[^>]*>/.exec(text)[0];
    assert.ok(!/\swidth=/.test(open) && !/\sheight=/.test(open), iconName(control.tag) + ": khong width/height");
    assert.ok(text.indexOf('stroke="#3B3B3B"') >= 0 && text.indexOf(accent) >= 0, iconName(control.tag) + ": net #3B3B3B + accent");
  }
  if (ctx.AxiomBridge) { ctx.AxiomBridge.stop(); }
});

// ---- Lenh ca tai lieu (docs/research/doi-chieu-wps-ai.md P1-1, P1-3): nut ribbon trong nhom Kiem tra tai lieu cua
// Writer, tag "doc:<id>" voi <id> trong DocumentActions cua Core (core-go/internal/agent/quickactions.go). Bam nut: hien
// khung chat LON (khong phai khung mini canh doan boi den - lenh lam ca tai lieu) roi nho Core chuyen lenh toi no
// (POST pane/request {quickAction: <id>}), giong duong du phong cua menu chuot phai.

test("ribbon.xml: nut Soat chinh ta + Lam dep bang trong nhom Kiem tra tai lieu, tag doc:<id>, chi hien o Writer", () => {
  const xml = fs.readFileSync(path.join(ADDIN, "ribbon.xml"), "utf8");
  const group = /<group id="axiomCheckGroup"[^>]*>([\s\S]*?)<\/group>/.exec(xml)[1];
  for (const [id, tag, label] of [["axiomProofread", "doc:proofread", "Soát chính tả"], ["axiomTableTidy", "doc:tableTidy", "Làm đẹp bảng"]]) {
    const button = new RegExp('<button id="' + id + '"([^>]*)>').exec(group);
    assert.ok(button, "#" + id + " phai nam trong nhom axiomCheckGroup");
    assert.match(button[1], new RegExp('\\stag="' + tag + '"'));
    assert.match(button[1], new RegExp('\\slabel="' + label + '"'));
    assert.match(button[1], /\sonAction="ribbon\.OnAction"/);
    assert.match(button[1], /\sgetVisible="ribbon\.OnGetVisible"/);
    assert.match(button[1], /\ssupertip="[^"]+"/);
  }
  const ctx = load(["probe.js", "commands.js", "main.js"], { Application: { Name: "WPS Spreadsheets" } });
  assert.equal(ctx.ribbon.OnGetVisible({ Id: "axiomProofread", Tag: "doc:proofread" }), false, "an o bang tinh");
  assert.equal(ctx.ribbon.OnGetVisible({ Id: "axiomTableTidy" }), false, "WPS chi dien Id: suy tag tu Id");
  const writer = load(["probe.js", "commands.js", "main.js"], { Application: { Name: "WPS Writer" } });
  assert.equal(writer.ribbon.OnGetVisible({ Id: "axiomProofread", Tag: "doc:proofread" }), true);
});

test("nut doc:<id> -> hien khung chat LON + POST pane/request {quickAction}; KHONG mo khung mini du co ShowDialog", () => {
  const dialogs = [];
  const panes = [];
  const app = {
    Name: "WPS Writer",
    ShowDialog(url) { dialogs.push(url); },
    CreateTaskPane(url) { const pane = { url, Visible: false }; panes.push(pane); return pane; },
  };
  const { ctx, posts } = mainWith(app);
  ctx.ribbon.OnAction({ Id: "axiomProofread" });
  assert.equal(dialogs.length, 0, "lenh ca tai lieu khong dung khung mini");
  assert.equal(panes.length, 1);
  assert.equal(panes[0].Visible, true);
  const client = /[?&]client=(c[0-9a-z]+)/.exec(panes[0].url)[1];
  assert.deepEqual(posts, [{ quickAction: "proofread", client }]);
  ctx.ribbon.OnAction({ Id: "axiomTableTidy", Tag: "doc:tableTidy" });
  assert.equal(panes.length, 1, "dung lai khung chat dang mo");
  assert.deepEqual(posts[1], { quickAction: "tableTidy", client });
  if (ctx.AxiomBridge) { ctx.AxiomBridge.stop(); }
});

test("loadQuickActions: documentActions cua Core -> nhan cho bong bong; khong lot vao menu Sua bang AI", async () => {
  const withDocs = { ok: true, result: { actions: QUICK.result.actions,
    documentActions: [{ id: "proofread", label: "Soát chính tả cả bài", edits: true }] } };
  const h = harness({ "/quick-actions": [200, withDocs],
    "/runs": [200, { ok: true, result: { runId: "d1", conversationId: "c1" } }] });
  assert.equal(await h.pane.loadQuickActions(), true);
  assert.deepEqual(h.state.quickActions.map((a) => a.id), ["polish", "summarize"], "menu boi den giu nguyen");
  assert.deepEqual(h.state.documentActions.map((a) => [a.id, a.label]), [["proofread", "Soát chính tả cả bài"]]);
  await h.pane.quick("proofread");
  await tick();
  assert.deepEqual(h.calls.find((c) => c.method === "POST").body, { quickAction: "proofread", app: "wps" });
  assert.deepEqual(h.state.messages[0], { role: "user", text: "Soát chính tả cả bài" });
});

// ---- Menu "Sua bang AI" (lenh nhanh): danh sach lay tu Core, chi gui id - cau lenh nam trong Core.
const QUICK = { ok: true, result: { actions: [
  { id: "polish", label: "Trau chuốt", edits: true },
  { id: "summarize", label: "Tóm tắt", edits: false },
] } };

test("loadQuickActions: GET quick-actions -> state.quickActions; app khac Writer khong hoi", async () => {
  const h = harness({ "/quick-actions": [200, QUICK] });
  assert.equal(await h.pane.loadQuickActions(), true);
  assert.deepEqual(h.calls[0], { url: BASE + "quick-actions", method: "GET", body: null });
  assert.deepEqual(h.state.quickActions.map((a) => a.id), ["polish", "summarize"]);
  assert.equal(h.state.quickActions[1].edits, false);

  const calc = harness({ "/quick-actions": [200, QUICK] }, null, "et");
  assert.equal(await calc.pane.loadQuickActions(), false);
  assert.equal(calc.calls.length, 0, "Calc/Impress khong co doan van de boi den: khong hoi Core");

  // Core cu chua co route (404) / loi mang: danh sach rong, KHONG day the loi vao khung chat.
  const old = harness({ "/quick-actions": [404, { ok: false, error: "not found" }] });
  assert.equal(await old.pane.loadQuickActions(), false);
  assert.deepEqual(old.state.quickActions, []);
  assert.equal(old.state.messages.length, 0, "menu an di la du, khong bao loi");
});

test("quick(id): POST runs {quickAction} khong kem prompt, tin nhan hien NHAN cua muc, bo qua ke hoach", async () => {
  const h = harness({ "/quick-actions": [200, QUICK],
    "/runs": [200, { ok: true, result: { runId: "q1", conversationId: "c9" } }] });
  await h.pane.loadQuickActions();
  h.pane.setPlan(true);
  await h.pane.quick("polish");
  await tick();
  const post = h.calls.find((c) => c.method === "POST");
  assert.deepEqual(post.body, { quickAction: "polish", app: "wps" },
    "chi gui id: cau lenh do Core lay, va lenh nhanh khong di cung lap ke hoach");
  assert.deepEqual(h.state.messages[0], { role: "user", text: "Trau chuốt" });
  assert.equal(h.state.running, true);
  assert.equal(await h.pane.quick("summarize"), false, "dang chay thi bo qua, giong send");
  FakeSource.last.emit("run.completed", { data: { reply: "Đã trau chuốt" } });
  await tick();
  await h.pane.quick("summarize");
  await tick();
  assert.deepEqual(h.calls.filter((c) => c.method === "POST").pop().body,
    { quickAction: "summarize", conversationId: "c9", app: "wps" }, "luot sau giu hoi thoai");
});

test("run.failed kind 'selection'/'selection-lock' -> cau tieng Viet rieng, khong phai chuoi cua Core", async () => {
  for (const [kind, want] of [["selection", /Hãy bôi đen đoạn văn cần làm trước/],
    ["selection-lock", /không khoá được vùng bôi đen/]]) {
    const h = harness({ "/runs": [200, { ok: true, result: { runId: "q2" } }] });
    await h.pane.quick("polish");
    await tick();
    FakeSource.last.emit("run.failed", { data: { error: "quick action \"polish\" works on the selected text", kind } });
    await tick();
    const last = h.state.messages[h.state.messages.length - 1];
    assert.equal(last.role, "error");
    assert.match(last.text, want);
    assert.equal(last.text.includes("quick action"), false, "khong lo chuoi tieng Anh cua Core");
    assert.equal(h.state.running, false);
  }
  // kind khac: giu nguyen chuoi loi nhu truoc.
  const h = harness({ "/runs": [200, { ok: true, result: { runId: "q3" } }] });
  await h.pane.send("x");
  await tick();
  FakeSource.last.emit("run.failed", { data: { error: "the bridge is not answering", kind: "office" } });
  await tick();
  assert.equal(h.state.messages[h.state.messages.length - 1].text, "the bridge is not answering");
});

// ---- Menu chuot phai (contextMenus): cung muc, cung thu tu va nhan voi bang cua Core.
function coreQuickActions() {
  const source = fs.readFileSync(path.join(ROOT, "core-go", "internal", "agent", "quickactions.go"), "utf8");
  // Chi bang QuickActions (menu chuot phai): DocumentActions cung file la lenh ca tai lieu cua nut ribbon.
  const start = source.indexOf("var QuickActions = []QuickAction{");
  const go = source.slice(start, source.indexOf("\n}\n", start));
  return [...go.matchAll(/\{ID: "([a-z]+)", Label: "([^"]+)"(, Edits: true)?(, Group: "([a-z]+)")?/g)]
    .map((m) => ({ id: m[1], label: m[2], edits: !!m[3], group: m[5] || "" }));
}

test("ribbon.xml contextMenus: chu thuong + dong danh sach, 'Sua bang AI' khop DUNG bang cua Core", () => {
  const core = coreQuickActions();
  assert.equal(core.length, 14, "doc duoc bang QuickActions cua Core (P1-4: 14 muc)");
  const context = ribbonControls("context");
  const menus = context.filter((c) => c.kind === "contextMenu").map((c) => c.idMso);
  assert.deepEqual(menus, ["ContextMenuText", "ContextMenuList"]);
  for (const suffix of ["t", "l"]) {
    const items = context.filter((c) => c.kind === "button" && c.id.startsWith("axiomQuick_") && c.id.endsWith("_" + suffix));
    assert.deepEqual(items.map((c) => [c.tag, c.label]), core.map((a) => ["quick:" + a.id, a.label]),
      "menu chuot phai (_" + suffix + ") phai cung muc, cung thu tu, cung nhan voi agent/quickactions.go");
    for (const item of items) {
      assert.equal(item.id, "axiomQuick_" + item.tag.slice(6) + "_" + suffix, "id theo mau axiomQuick_<muc>_<t|l>");
      assert.equal(item.onAction, "ribbon.OnAction");
      assert.equal(item.getImage, "", "muc con khong co tep anh rieng - getImage se ra dau '?'");
    }
    // Vach ngan nam dung truoc muc chi-tra-loi dau tien (giong menu trong pane).
    const block = context.slice(context.findIndex((c) => c.id === "axiomCtxQuick_" + suffix));
    const sep = block.findIndex((c) => c.kind === "menuSeparator");
    const firstAnswer = core.find((a) => !a.edits).id;
    assert.equal(block[sep + 1].tag, "quick:" + firstAnswer);
    // P1-4 (docs/research/doi-chieu-wps-ai.md): cac muc nhom "tone" nam trong menu con "Doi giong" - do tren WPS 12.1.2
    // that 31/10/2026: menu long trong contextMenu hien va bam duoc (OnAction toi add-in nhu muc thuong).
    const xml = fs.readFileSync(path.join(ADDIN, "ribbon.xml"), "utf8");
    const tone = new RegExp('<menu id="axiomCtxTone_' + suffix + '"([^>]*)>([\\s\\S]*?)</menu>').exec(xml);
    assert.ok(tone, "thieu menu con <menu id=\"axiomCtxTone_" + suffix + "\"> (Doi giong) trong menu Sua bang AI");
    assert.match(tone[1], /label="Đổi giọng"/, "menu con co nhan 'Đổi giọng'");
    assert.doesNotMatch(tone[1], /getImage=/, "menu con khong co tep anh rieng - getImage se ra dau '?'");
    const inTone = [...tone[2].matchAll(/tag="quick:([a-z]+)"/g)].map((m) => m[1]);
    assert.deepEqual(inTone, core.filter((a) => a.group === "tone").map((a) => a.id),
      "menu con Doi giong chua DUNG cac muc nhom tone cua Core, dung thu tu");
    const quickMenu = new RegExp('<menu id="axiomCtxQuick_' + suffix + '"[\\s\\S]*?<menuSeparator').exec(xml);
    assert.ok(quickMenu && quickMenu[0].indexOf('id="axiomCtxTone_' + suffix + '"') > 0,
      "menu con Doi giong nam TRONG menu Sua bang AI, truoc vach ngan");
    const ask = context.find((c) => c.id === "axiomCtxAsk_" + suffix);
    assert.ok(ask && ask.tag === "askAi" && ask.label === "Hỏi AI về đoạn này", "con co muc 'Hoi AI ve doan nay'");
  }
});

test("menu chuot phai: muc quick:<id> -> hien khung chat (dung lai cai dang mo) + POST pane/request; khong ve anh '?'", async () => {
  const created = [];
  const posts = [];
  const ctx = load(["probe.js", "commands.js", "main.js"], {
    Application: { CreateTaskPane(url) { const pane = { url, Visible: false }; created.push(pane); return pane; } },
    location: { href: "http://127.0.0.1:47840/wps/addin/index.html" },
    fetch: (url, options = {}) => {
      if (String(url).endsWith("/pane/request")) { posts.push({ url: String(url), body: JSON.parse(options.body) }); }
      return new Promise(() => {});
    },
  });
  ctx.ribbon.OnAction({ Id: "axiomAskAI" });
  assert.equal(created.length, 1);
  created[0].Visible = false;
  // WPS co ban chi dien Id: tag suy ra tu id axiomQuick_<muc>_<t|l>.
  ctx.ribbon.OnAction({ Id: "axiomQuick_polish_t" });
  assert.equal(created.length, 1, "dung lai khung chat dang mo - giu nguyen hoi thoai");
  assert.equal(created[0].Visible, true, "khung chat phai hien ra");
  // client: id cua ban add-in nay - Core chi giao yeu cau cho khung chat cua CUNG app (api/wpspane.go).
  const client = /[?&]client=(c[0-9a-z]+)/.exec(created[0].url)[1];
  assert.deepEqual(posts, [{ url: "http://127.0.0.1:47840/wps/v1/pane/request", body: { quickAction: "polish", client } }]);
  ctx.ribbon.OnAction({ Tag: "quick:summarize" });
  assert.deepEqual(posts[1].body, { quickAction: "summarize", client });
  // Muc menu chuot phai khong khai getImage nen WPS khong hoi; tag dung chung voi nut ribbon "Trau chuot" (co icon) - neu
  // co hoi thi tra tep CO THAT, khong phai duong dan 404 (WPS ve "?").
  const rel = ctx.ribbon.OnGetImage({ Id: "axiomQuick_polish_t" });
  assert.equal(rel, "images/quick_polish.svg");
  assert.ok(fs.existsSync(path.join(ADDIN, rel)), "muc con: tep icon co that");
  assert.equal(ctx.ribbon.OnGetImage({ Id: "axiomCtxQuick_t" }), "images/quickMenu.svg");
  if (ctx.AxiomBridge) { ctx.AxiomBridge.stop(); }
});

// ---- Menu chuot phai -> khung chat mini (mini.html) bang Application.ShowDialog; khong co ShowDialog thi khung lon.
// Moi ban add-in (moi tien trinh WPS) co mot client id: poll gui no, pane mo ra mang dung id do (?client=), va hai ban
// add-in khac nhau thi id khac nhau - Core nho vay tach phien Writer/ET (core-go/internal/wpsbridge).
test("client id: poll gui clientId, pane mang dung id do; hai ban add-in khac id", () => {
  const ids = [];
  for (let i = 0; i < 2; i++) {
    const polls = [];
    const created = [];
    const ctx = load(["probe.js", "commands.js", "main.js"], {
      Application: { CreateTaskPane(url) { const pane = { url, Visible: false }; created.push(pane); return pane; } },
      location: { href: "http://127.0.0.1:47840/wps/addin/index.html" },
      fetch: (url, options = {}) => {
        if (String(url).endsWith("/poll")) { polls.push(JSON.parse(options.body)); }
        return new Promise(() => {});
      },
    });
    ctx.ribbon.OnAddinLoad();
    ctx.ribbon.OnAction({ Id: "axiomAskAI" });
    assert.equal(polls.length, 1, "OnAddinLoad phai poll ngay");
    const id = polls[0].clientId;
    assert.match(String(id), /^c[0-9a-z]+$/, "poll phai gui clientId");
    assert.equal(polls[0].client, "wps-jsaddon", "client van la loai add-in (khong doi hop dong cu)");
    assert.equal(/[?&]client=([^&]+)/.exec(created[0].url)[1], id, "pane phai mang dung client id cua poll");
    ids.push(id);
    ctx.AxiomBridge.stop();
  }
  assert.notEqual(ids[0], ids[1], "hai ban add-in phai khac client id");
});

function mainWith(app, ratio = 1) {
  const posts = [];
  const ctx = load(["probe.js", "commands.js", "main.js"], {
    Application: app,
    location: { href: "http://127.0.0.1:47840/wps/addin/index.html" },
    devicePixelRatio: ratio,
    fetch: (url, options = {}) => {
      if (String(url).endsWith("/pane/request")) { posts.push(JSON.parse(options.body)); }
      return new Promise(() => {});
    },
  });
  return { ctx, posts };
}

test("chuot phai: co ShowDialog -> mo khung chat mini canh doan boi den (khong modal), KHONG mo khung lon", () => {
  const dialogs = [];
  const panes = [];
  const app = {
    ShowDialog(url, title, w, h, modal) { dialogs.push({ url, title, w, h, modal }); },
    CreateTaskPane(url) { const pane = { url, Visible: false }; panes.push(pane); return pane; },
  };
  const { ctx, posts } = mainWith(app, 2);
  ctx.ribbon.OnAction({ Id: "axiomQuick_polish_t", Tag: "quick:polish" });
  const mini = /^http:\/\/127\.0\.0\.1:47840\/wps\/addin\/mini\.html\?app=wps&client=(c[0-9a-z]+)&quick=polish$/.exec(dialogs[0].url);
  assert.ok(mini, "khung mini mang ?client= cua add-in de Core chay dung app: " + dialogs[0].url);
  assert.deepEqual({ ...dialogs[0], url: "" }, { url: "", title: "Sửa bằng AI", w: 880, h: 760, modal: false },
    "kich thuoc nhan devicePixelRatio nhu SDK");
  assert.equal(panes.length, 0, "khung mini thay cho khung lon");
  assert.equal(posts.length, 0, "khong di vong qua Core khi da mo duoc khung mini");
  // "Hoi AI ve doan nay": khung mini trong, cho chon/go.
  ctx.ribbon.OnAction({ Id: "axiomCtxAsk_l", Tag: "askAi" });
  assert.equal(dialogs[1].url, "http://127.0.0.1:47840/wps/addin/mini.html?app=wps&client=" + mini[1]);
  // Nut Ask AI tren ribbon (cung tag askAi) van mo khung chat LON.
  ctx.ribbon.OnAction({ Id: "axiomAskAI", Tag: "askAi" });
  assert.equal(panes.length, 1);
  assert.equal(dialogs.length, 2);
  if (ctx.AxiomBridge) { ctx.AxiomBridge.stop(); }
});

test("chuot phai: ShowDialog loi -> quay ve khung chat lon + Core chuyen muc (khong mat viec nguoi dung chon)", () => {
  const panes = [];
  const app = {
    ShowDialog() { throw new Error("not supported"); },
    CreateTaskPane(url) { const pane = { url, Visible: false }; panes.push(pane); return pane; },
  };
  const { ctx, posts } = mainWith(app);
  ctx.ribbon.OnAction({ Id: "axiomQuick_fix_t", Tag: "quick:fix" });
  assert.equal(panes.length, 1);
  assert.deepEqual(posts, [{ quickAction: "fix", client: /[?&]client=(c[0-9a-z]+)/.exec(panes[0].url)[1] }]);
  ctx.ribbon.OnAction({ Id: "axiomCtxAsk_t", Tag: "askAi" });
  assert.equal(panes[0].Visible, true, "Hoi AI ve doan nay: khung lon hien ra");
  if (ctx.AxiomBridge) { ctx.AxiomBridge.stop(); }
});

test("trich dan: citationCommand -> lenh goto; gotoCitation POST /exec, loi thi noi ro", async () => {
  const h = harness({ "/exec": [200, { ok: false, error: "'paragraph' must be between 1 and 3" }] });
  const map = (t) => JSON.parse(JSON.stringify(h.ctx.AxiomPaneCore.citationCommand(t)));
  assert.deepEqual(map("§12"), { action: "writer.goto", params: { paragraph: 12 } });
  assert.deepEqual(map("§ 4"), { action: "writer.goto", params: { paragraph: 4 } });
  assert.deepEqual(map("slide 3"), { action: "wpp.goto", params: { slide: 3 } });
  assert.deepEqual(map("'Báo cáo'!A1:B5"), { action: "et.goto", params: { range: "'Báo cáo'!A1:B5" } });
  assert.equal(h.ctx.AxiomPaneCore.citationCommand("ghi chú"), null);
  assert.equal(await h.pane.gotoCitation("§9"), false);
  const call = h.calls.find((c) => c.url === BASE + "exec");
  assert.deepEqual(call.body, { action: "writer.goto", params: { paragraph: 9 }, app: "wps" });
  const last = h.state.messages[h.state.messages.length - 1];
  assert.match(last.text, /Không nhảy tới được §9/);
});

test("trich dan: addCitedText tach link bam duoc, link Markdown thuong giu nguyen chu", () => {
  const nodes = [];
  const makeNode = (tag) => ({ tag, attrs: {}, children: [], className: "",
    setAttribute(k, v) { this.attrs[k] = v; }, appendChild(c) { this.children.push(c); } });
  const ctx = load(["pane-dom.js"], {
    document: { createTextNode: (t) => ({ text: t }), createElement: (tag) => makeNode(tag) },
  });
  const el = makeNode("div");
  ctx.AxiomPaneDom.addCitedText(el, "Mức trần ở [§12] và [Data!C42]; xem [web](http://a) hoặc [slide 2].");
  const links = el.children.filter((c) => c.tag === "span");
  assert.deepEqual(links.map((l) => l.attrs["data-value"]), ["§12", "Data!C42", "slide 2"]);
  assert.ok(links.every((l) => l.attrs["data-act"] === "cite" && l.className === "cite"));
  const text = el.children.map((c) => c.text || (c.children[0] && c.children[0].text)).join("");
  assert.equal(text, "Mức trần ở [§12] và [Data!C42]; xem [web](http://a) hoặc [slide 2].");
  nodes.length = 0;
});

// ---- Nhom Tao moi + Viet voi AI (01/11/2026): nut theo ung dung, lenh can go yeu cau (Soan van ban).
// Bang Core: QuickActions (14 muc menu Sua bang AI) + DocumentActions (smartCreate: app wps + input; slidesFromDoc: app wpp).

test("ribbon.xml: nhom Tao moi + Viet voi AI dung thu tu; 14 muc Viet voi AI khop DUNG bang Core (tag, nhan)", () => {
  const xml = fs.readFileSync(path.join(ADDIN, "ribbon.xml"), "utf8");
  const groups = [...xml.matchAll(/<group id="([A-Za-z]+)"/g)].map((m) => m[1]);
  assert.deepEqual(groups, ["axiomAssistantGroup", "axiomCreateGroup", "axiomWriteGroup", "axiomCheckGroup",
    "axiomManageGroup", "axiomSystemGroup"]);
  const write = /<group id="axiomWriteGroup"[^>]*>([\s\S]*?)<\/group>/.exec(xml)[1];
  const items = [...write.matchAll(/<button id="axiomRb_([a-z]+)" label="([^"]+)"[^>]*tag="([^"]+)"/g)];
  const core = coreQuickActions();
  assert.deepEqual(items.map((m) => [m[1], m[2], m[3]]).sort(), core.map((a) => [a.id, a.label, "quick:" + a.id]).sort());
}
);

test("OnGetVisible: Soan van ban + Viet voi AI chi o Writer, Tao slide tu tai lieu chi o Presentation; WPS chi dien Id van dung", () => {
  const seen = {};
  for (const name of ["WPS Writer", "WPS Presentation", "WPS Spreadsheets"]) {
    const ctx = load(["probe.js", "commands.js", "main.js"], { Application: { Name: name } });
    const v = (control) => ctx.ribbon.OnGetVisible(control);
    seen[name] = [v({ Id: "axiomSmartCreate" }), v({ Id: "axiomSlidesFromDoc", Tag: "doc:slidesFromDoc" }),
      v({ Id: "axiomRb_polish" }), v({ Tag: "quick:explain" }), v({ Id: "axiomRbTone" }), v({ Id: "axiomProofread" })];
  }
  assert.deepEqual(seen, {
    "WPS Writer": [true, false, true, true, true, true],
    "WPS Presentation": [false, true, false, false, false, false],
    "WPS Spreadsheets": [false, false, false, false, false, false],
  });
});

test("OnAction: nut Viet voi AI (chi co Id) -> khung chat mini nhu menu chuot phai; Tao slide o Presentation -> khung lon + POST", () => {
  const dialogs = [];
  const writer = mainWith({ Name: "WPS Writer", ShowDialog(url) { dialogs.push(url); }, CreateTaskPane: () => ({ Visible: false }) });
  writer.ctx.ribbon.OnAction({ Id: "axiomRb_polish" });
  writer.ctx.ribbon.OnAction({ Id: "axiomRb_formal" });
  assert.match(dialogs[0], /mini\.html\?app=wps&client=c[0-9a-z]+&quick=polish$/);
  assert.match(dialogs[1], /&quick=formal$/, "muc trong menu Doi giong cung chay");
  if (writer.ctx.AxiomBridge) { writer.ctx.AxiomBridge.stop(); }

  const panes = [];
  const wpp = mainWith({ Name: "WPS Presentation", ShowDialog(url) { dialogs.push(url); },
    CreateTaskPane(url) { const pane = { url, Visible: false }; panes.push(pane); return pane; } });
  wpp.ctx.ribbon.OnAction({ Id: "axiomSlidesFromDoc" });
  assert.equal(dialogs.length, 2, "lenh ca tai lieu khong dung khung mini");
  assert.equal(panes.length, 1);
  assert.match(panes[0].url, /pane\.html\?app=wpp/);
  const client = /[?&]client=(c[0-9a-z]+)/.exec(panes[0].url)[1];
  assert.deepEqual(wpp.posts, [{ quickAction: "slidesFromDoc", client }]);
  if (wpp.ctx.AxiomBridge) { wpp.ctx.AxiomBridge.stop(); }
});

const CREATE = { ok: true, result: { actions: QUICK.result.actions, documentActions: [
  { id: "proofread", label: "Soát chính tả cả bài", edits: true },
  { id: "smartCreate", label: "Soạn văn bản", edits: true, app: "wps", input: "Bạn muốn soạn văn bản gì?" },
  { id: "slidesFromDoc", label: "Tạo slide từ tài liệu", edits: true, app: "wpp" },
] } };

test("loadQuickActions: Presentation hoi Core, chi giu lenh ca tai lieu cua wpp; Writer bo lenh cua wpp", async () => {
  const wpp = harness({ "/quick-actions": [200, CREATE] }, null, "wpp");
  assert.equal(await wpp.pane.loadQuickActions(), true);
  assert.deepEqual(wpp.state.quickActions, [], "Presentation khong co menu boi den");
  assert.deepEqual(wpp.state.documentActions.map((a) => a.id), ["slidesFromDoc"]);
  const wps = harness({ "/quick-actions": [200, CREATE] });
  await wps.pane.loadQuickActions();
  assert.deepEqual(wps.state.documentActions.map((a) => [a.id, a.app, a.input]),
    [["proofread", "", ""], ["smartCreate", "wps", "Bạn muốn soạn văn bản gì?"]], "app rong = Writer (nut doc:<id> cu)");
});

test("quick(id) voi lenh can go (input): KHONG chay ngay - cho cau go; send(cau) -> POST {quickAction, prompt}; huy duoc", async () => {
  const h = harness({ "/quick-actions": [200, CREATE], "/runs": [200, { ok: true, result: { runId: "s1", conversationId: "c1" } }] });
  await h.pane.loadQuickActions();
  assert.equal(await h.pane.quick("smartCreate"), true);
  await tick();
  assert.equal(h.calls.filter((c) => c.method === "POST").length, 0, "chua gui gi - cho nguoi dung go yeu cau");
  assert.deepEqual(h.state.pendingAction, { id: "smartCreate", label: "Soạn văn bản", input: "Bạn muốn soạn văn bản gì?" });
  assert.equal(h.state.running, false);
  h.pane.setPlan(true);
  assert.equal(await h.pane.send("  Thông báo nghỉ Tết cho nhân viên  "), true);
  await tick();
  assert.deepEqual(h.calls.find((c) => c.method === "POST").body,
    { quickAction: "smartCreate", prompt: "Thông báo nghỉ Tết cho nhân viên", app: "wps" }, "lenh nhanh khong di cung lap ke hoach");
  assert.deepEqual(h.state.messages[0], { role: "user", text: "Soạn văn bản: Thông báo nghỉ Tết cho nhân viên" });
  assert.equal(h.state.pendingAction, null, "gui xong thi het cho");
  FakeSource.last.emit("run.completed", { data: { reply: "Đã soạn" } });
  await tick();
  // Huy: quay ve o soan thuong, cau go sau la luot binh thuong.
  await h.pane.quick("smartCreate");
  h.pane.cancelPending();
  assert.equal(h.state.pendingAction, null);
  h.pane.setPlan(false);
  await h.pane.send("Câu thường");
  await tick();
  const last = h.calls.filter((c) => c.method === "POST").pop().body;
  assert.equal(last.quickAction, undefined);
  assert.equal(last.prompt, "Câu thường");
  // Lenh khong co input (Soat chinh ta) van chay ngay nhu cu.
  await h.pane.quick("proofread");
  assert.equal(h.state.pendingAction, null);
});

test("lenh dang cho cau go + dang co luot chay: send bi tu choi nhung KHONG mat lenh dang cho", async () => {
  const h = harness({ "/quick-actions": [200, CREATE], "/runs": [200, { ok: true, result: { runId: "b1" } }] });
  await h.pane.loadQuickActions();
  await h.pane.send("Việc khác");
  await tick();
  await h.pane.quick("smartCreate");
  assert.equal(await h.pane.send("Thông báo"), false, "dang chay thi khong gui");
  h.pane.setPlan(false); // phat lai state (onChange) de doc trang thai that, khong phai ban chup cu
  assert.equal(h.state.pendingAction && h.state.pendingAction.id, "smartCreate", "van cho - nguoi dung gui lai duoc");
});
