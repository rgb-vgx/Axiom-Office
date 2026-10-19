// Spec T4b (docs/wps-bridge.md muc 7): pane Ask AI trong WPS - logic khong DOM + nut ribbon. Khong can WPS.
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
  assert.equal(created[0].url, "http://127.0.0.1:47840/wps/addin/pane.html?app=wps");
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
  assert.deepEqual(call.body, { action: "writer.undo", params: { count: 2 } });
  assert.equal(h.state.edits, 0);
  assert.equal(h.state.canUndo, false, "hoan tac roi thi thoi hien nut");
  const last = h.state.messages[h.state.messages.length - 1];
  assert.equal(last.role, "info");
  assert.match(last.text, /Đã hoàn tác 2 thao tác/);
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
  assert.deepEqual(call.body, { action: "writer.insertStyledText", params: { text: "**Kính gửi** anh" } });
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

function ribbonControls() {
  const xml = fs.readFileSync(path.join(ADDIN, "ribbon.xml"), "utf8");
  const out = [];
  for (const found of xml.matchAll(/<(button|menu|toggleButton|dropDown|dynamicMenu)\s([^>]*)>/g)) {
    const attrs = found[2];
    const one = (name) => { const m = new RegExp(name + '="([^"]*)"').exec(attrs); return m ? m[1] : ""; };
    out.push({ kind: found[1], id: one("id"), tag: one("tag"), getImage: one("getImage") });
  }
  return out;
}

function iconFiles() {
  const dir = path.join(ADDIN, "images");
  assert.ok(fs.existsSync(dir), "thieu thu muc src/AxiomOffice.WPS/images");
  return fs.readdirSync(dir).filter((n) => n.endsWith(".svg")).map((n) => n.slice(0, -4)).sort();
}

test("ribbon.xml: moi control co getImage, va moi tag co tep anh images/<tag>.svg", () => {
  const controls = ribbonControls();
  assert.ok(controls.length >= 11, "doc duoc control cua ribbon.xml: " + controls.length);
  const files = iconFiles();
  assert.ok(files.length >= 11, "doc duoc tep anh cua images/: " + files.length);
  for (const control of controls) {
    assert.equal(control.getImage, "ribbon.OnGetImage", "#" + control.id + " thieu getImage - nut khong co icon");
    assert.ok(control.tag, "#" + control.id + " thieu tag - khong biet tra anh nao");
    assert.ok(files.indexOf(control.tag) >= 0,
      "images/: thieu tep cho tag '" + control.tag + "' (#" + control.id + ")");
  }
  // Chieu nguoc lai: tep anh chet (khong nut nao dung) cung la loi.
  const tags = controls.map((c) => c.tag);
  for (const name of files) {
    assert.ok(tags.indexOf(name) >= 0, "images/: co '" + name + ".svg' nhung ribbon.xml khong co nut nao dung");
  }
});

test("OnGetImage: tra duong dan tuong doi images/<tag>.svg; khong biet tag thi tra chuoi rong", () => {
  const accent = "#0F6CBD"; // theme.py ACCENT - mau cua he thiet ke pane
  const ctx = load(["probe.js", "commands.js", "main.js"], {
    Application: { CreateTaskPane: () => ({ Visible: false }) },
  });
  const rel = ctx.ribbon.OnGetImage({ Id: "axiomAskAI" });
  assert.equal(rel, "images/askAi.svg", "getImage phai tra duong dan tuong doi, khong phai data URI");
  // WPS co ban chi dien Tag (khong dien Id) - hai duong phai ra cung mot duong dan.
  assert.equal(ctx.ribbon.OnGetImage({ Tag: "askAi" }), rel);
  // Control la thi tra chuoi rong: WPS ve nut khong anh (dung nhu ban cu), khong lam hong ribbon.
  // Tra ve mot duong dan 404 thi WPS con te hon - no ve dau "?" (dung cai da gap voi data URI).
  assert.equal(ctx.ribbon.OnGetImage({ Id: "khong-co-that" }), "");
  assert.equal(ctx.ribbon.OnGetImage(null), "");
  // Moi tep anh phai la MOT the <svg> khap kin, chi viewBox (de WPS tu co gian theo co nut: nut lon
  // 32px, nut nho 16px - y nhu tep mau cua SDK @wps-jsapi/wps/images/1.svg), va dung mau accent.
  for (const control of ribbonControls()) {
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
  if (ctx.AxiomBridge) { ctx.AxiomBridge.stop(); }
});
