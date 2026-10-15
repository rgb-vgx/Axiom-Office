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

function harness(responses = {}, fetchOverride = null) {
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
  const pane = ctx.AxiomPaneCore.create({ fetch, EventSource: FakeSource, base: BASE, onChange: (s) => { state = JSON.parse(JSON.stringify(s)); } });
  return { pane, calls, get state() { return state; } };
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
