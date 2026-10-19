// Spec T4d (docs/wps-bridge.md muc 7 + docs/ribbon-ui.md §2): PHAN GIAO DIEN cua pane Ask AI trong WPS.
// test_pane.mjs kiem logic khong DOM (pane-core.js); file nay kiem chinh pane.html: cong tac ke hoach, the
// suy luan / ghi nho / ke hoach, nut Hoan tac + Chen tra loi o footer, the loi, va bay man hinh phu
// (Ghi nho / Nhat ky / Thiet lap / Ky nang / Trang thai / Soat tai lieu / Nhat ky Core) - doi chieu voi
// ban Windows (AskAiPane.cs, PaneControls.cs, MemoryForm.cs, AdminForms.cs, RibbonUi.cs) va ban
// LibreOffice (dialogs.py, checks.py).
//
//   node --test tests/wps/test_pane_view.mjs
//
// DOM gia o tests/wps/pane_dom.mjs - KHONG mo trinh duyet (pane chi song trong CEF cua WPS).
import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { loadPane, press, ADDIN, ROOT } from "./pane_dom.mjs";

const PANE = path.join(ADDIN, "pane.html");
const CHECKS_PY = path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath", "axiom", "checks.py");
const BASE = "http://127.0.0.1:47840/wps/v1/";
const tick = () => new Promise((resolve) => setTimeout(resolve, 10));

// Dinh tuyen fetch theo "METHOD duong-dan-cuoi": gia tri la [status, payload] hoac ham.
function router(routes = {}) {
  const calls = [];
  const fetch = (url, options = {}) => {
    const method = (options.method || "GET").toUpperCase();
    const body = options.body ? JSON.parse(options.body) : null;
    const full = String(url);
    calls.push({ url: full, method, body });
    for (const key of Object.keys(routes)) {
      const [wantMethod, wantPath] = key.split(" ");
      if (method !== wantMethod.toUpperCase() || !full.split("?")[0].endsWith(wantPath)) { continue; }
      const reply = typeof routes[key] === "function" ? routes[key]({ url: full, method, body }) : routes[key];
      const [status, payload] = reply;
      return Promise.resolve({ ok: status < 400, status, json: () => Promise.resolve(payload) });
    }
    return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ ok: true, result: {} }) });
  };
  return { fetch, calls };
}

function paneWith(routes, { search = "?app=wps", confirm, navigator } = {}) {
  const { fetch, calls } = router(routes);
  const pane = loadPane({ search, fetch, confirm, navigator });
  return { pane, calls, url: (suffix) => calls.filter((c) => c.url.split("?")[0].endsWith(suffix)) };
}

// Bat dau mot luot chay: gui cau hoi roi mo SSE (giong nguoi dung bam Gui).
async function startRun(pane, prompt = "Sửa giúp tôi") {
  pane.byId("input").value = prompt;
  pane.click("sendBtn");
  await tick();
}

test("man hinh trong: tieu de + goi y theo app, nut footer an", () => {
  const { pane } = paneWith({});
  assert.equal(pane.text("paneTitle"), "Writer", "tieu de = ten app dang mo");
  assert.equal(pane.text("emptyTitle"), "Bạn muốn làm gì với văn bản này?");
  assert.match(pane.text("chat"), /GỢI Ý/);
  assert.equal(pane.text("statusText"), "Sẵn sàng");
  assert.ok(pane.byId("undoBtn").hasAttribute("hidden"), "chua sua gi thi khong co nut Hoan tac");
  assert.ok(pane.byId("insertBtn").hasAttribute("hidden"));
  assert.equal(pane.text("paneSubtitle"), "Axiom Office", "chua biet model thi phu de la ten san pham");
});

test("cong tac 'Lap ke hoach truoc': doi chu, doi goi y o soan, gui kem options.mode=plan", async () => {
  const { pane, calls } = paneWith({ "POST /runs": [200, { ok: true, result: { runId: "r1" } }] });
  assert.equal(pane.text("planToggle"), "○ Lập kế hoạch trước");
  pane.click("planToggle");
  assert.equal(pane.byId("planToggle").getAttribute("aria-pressed"), "true");
  assert.equal(pane.text("planToggle"), "● Lập kế hoạch trước");
  assert.match(pane.byId("input").getAttribute("placeholder"), /AI lập kế hoạch trước, chưa sửa gì/,
    "bat ke hoach thi o soan noi ro chua sua gi");
  await startRun(pane, "Dọn lại bố cục giáo án");
  assert.deepEqual(calls.find((c) => c.method === "POST").body.options, { mode: "plan" });
});

test("dang chay: model o phu de, dong tien do, the suy luan co nut Hien/An", async () => {
  const { pane } = paneWith({ "POST /runs": [200, { ok: true, result: { runId: "r2" } }] });
  await startRun(pane, "Trình bày lại");
  pane.emit("run.started", { conversationId: "c1", model: "claude-sonnet-5-5", mode: "act" });
  await tick();
  assert.equal(pane.text("paneSubtitle"), "Writer · claude-sonnet-5-5");
  assert.equal(pane.text("statusText"), "Đang chạy · claude-sonnet-5-5");
  pane.emit("run.progress", { round: 2, text: "Đang thiết kế phần 1–15: 7 slide…" });
  await tick();
  assert.match(pane.text("chat"), /Đang thiết kế phần 1–15: 7 slide…/);
  pane.emit("run.reasoning", { round: 1, text: "x".repeat(400) });
  await tick();
  assert.match(pane.text("chat"), /Suy luận · vòng 1/);
  const toggle = pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "reason");
  assert.ok(toggle, "the suy luan phai co nut Hien/An");
  assert.equal(toggle.text, "Hiện", "suy luan dai thi thu lai truoc");
  assert.equal(pane.text("chat").includes("x".repeat(300)), false, "ban thu lai khong do ca khoi van dai");
  pane.click(toggle);
  assert.equal(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "reason").text, "Ẩn");
  assert.ok(pane.text("chat").includes("x".repeat(300)), "bam Hien thi hien day du");
});

test("ky nang + ghi nho: dong rieng, the 'Da ghi nho' co nut Xoa", async () => {
  const { pane, calls } = paneWith({
    "POST /runs": [200, { ok: true, result: { runId: "r3" } }],
    "DELETE /memory/m1": [200, { ok: true, result: { deleted: "m1" } }],
  });
  await startRun(pane, "Nhớ giúp tôi");
  pane.emit("skill.loaded", { name: "soan-thao", source: "user" });
  pane.emit("memory.written", { id: "m1", event: "ADD", text: "Người dùng thích văn phong ngắn" });
  await tick();
  assert.match(pane.text("chat"), /Dùng kỹ năng: soan-thao/);
  assert.match(pane.text("chat"), /Đã ghi nhớ/);
  assert.match(pane.text("chat"), /Người dùng thích văn phong ngắn/);
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "forget"));
  await tick();
  const call = calls.find((c) => c.method === "DELETE");
  assert.equal(call.url, BASE + "memory/m1");
  assert.match(pane.text("chat"), /Đã xoá/, "xoa xong thi gach ngang va noi da xoa");
});

test("footer: 'Hoan tac luot nay' chay writer.undo {count}, 'Chen tra loi' chen cau tra loi", async () => {
  const { pane, calls } = paneWith({
    "POST /runs": [200, { ok: true, result: { runId: "r4" } }],
    "POST /exec": [200, { ok: true, result: { undone: 2 } }],
  });
  await startRun(pane, "Trình bày lại");
  pane.emit("tool.finished", { callId: "k1", action: "writer.appendText", ok: true });
  pane.emit("tool.finished", { callId: "k2", action: "writer.insertTable", ok: true });
  pane.emit("run.completed", { reply: "Xong rồi", seconds: 7.8, rounds: 2 });
  await tick();
  assert.equal(pane.text("statusText"), "Xong trong 7s · 2 thao tác");
  assert.equal(pane.byId("undoBtn").hasAttribute("hidden"), false, "sua tai lieu roi thi hien nut Hoan tac");
  assert.equal(pane.byId("insertBtn").hasAttribute("hidden"), false, "co cau tra loi thi hien nut Chen");
  pane.click("undoBtn");
  await tick();
  assert.deepEqual(calls.find((c) => c.url === BASE + "exec").body,
    { action: "writer.undo", params: { count: 2 } });
  assert.match(pane.text("chat"), /Đã hoàn tác 2 thao tác của lượt vừa rồi\./);
  pane.click("insertBtn");
  await tick();
  const execs = calls.filter((c) => c.url === BASE + "exec");
  assert.deepEqual(execs[1].body, { action: "writer.insertStyledText", params: { text: "Xong rồi" } });
  assert.equal(pane.text("statusText"), "Đã chèn vào tài liệu");
});

test("the ke hoach: 'Thuc hien ke hoach' gui lai dung cau lenh cua Windows", async () => {
  const { pane, calls } = paneWith({ "POST /runs": [200, { ok: true, result: { runId: "r5" } }] });
  pane.click("planToggle");
  await startRun(pane, "Dọn lại bố cục");
  pane.emit("run.started", { model: "m", mode: "plan" });
  pane.emit("run.completed", { reply: "1. Đổi tiêu đề\n2. Kẻ bảng", seconds: 5 });
  await tick();
  assert.match(pane.text("chat"), /Kế hoạch đã sẵn sàng - tài liệu chưa bị sửa/);
  assert.equal(pane.text("statusText"), "Đã lập kế hoạch trong 5s · chưa sửa gì");
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "planExec"));
  await tick();
  const runs = calls.filter((c) => c.url === BASE + "runs");
  assert.equal(runs.length, 2);
  assert.equal(runs[1].body.options, undefined, "chay that thi khong con che do ke hoach");
  assert.match(runs[1].body.prompt, /^Thực hiện kế hoạch ở trên\./);
  assert.equal(pane.byId("planToggle").getAttribute("aria-pressed"), "false", "chay xong thi tat cong tac");
});

test("the loi: 'Thu lai' gui lai dung cau hoi vua roi, 'Mo Cai dat' mo man hinh Thiet lap", async () => {
  const { pane, calls } = paneWith({
    "POST /runs": [200, { ok: true, result: { runId: "r6" } }],
    "GET /setup": [200, { ok: true, result: { current: {}, problems: [] } }],
  });
  await startRun(pane, "Sửa giúp tôi");
  pane.emit("run.failed", { error: "hết hạn mức" });
  await tick();
  assert.match(pane.text("chat"), /Có lỗi xảy ra/);
  assert.match(pane.text("chat"), /hết hạn mức/);
  assert.equal(pane.text("statusText"), "Lỗi · nhấn Cài đặt để kiểm tra Agent Core");
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "retry"));
  await tick();
  const runs = calls.filter((c) => c.url === BASE + "runs");
  assert.equal(runs.length, 2, "Thu lai phai gui lai dung cau hoi vua roi");
  assert.equal(runs[1].body.prompt, "Sửa giúp tôi");
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "view" &&
    el.getAttribute("data-value") === "setup"));
  await tick();
  assert.equal(pane.byId("view").hasAttribute("hidden"), false, "'Mo Cai dat' mo man hinh Thiet lap");
});

test("man hinh Ghi nho: loc pham vi, xoa tam, khoi phuc, xoa vinh vien co xac nhan", async () => {
  const memories = [
    { id: "m1", scope: "user", text: "Thích văn phong ngắn", pinned: true },
    { id: "m2", scope: "document", scopeKey: "bao-cao.docx", text: "Báo cáo tuần", deletedAt: "2026-10-19T02:00:00Z" },
  ];
  const { pane, calls } = paneWith({
    "GET /memory": [200, { ok: true, result: { memories } }],
    "DELETE /memory/m1": [200, { ok: true, result: { deleted: "m1" } }],
    "POST /memory/m2/restore": [200, { ok: true, result: { restored: "m2" } }],
    "DELETE /memory": [200, { ok: true, result: { purged: 2 } }],
  }, { confirm: () => true });
  pane.click("memoryBtn");
  await tick();
  assert.equal(pane.byId("view").hasAttribute("hidden"), false);
  assert.match(pane.text("view"), /Bộ nhớ AI/);
  assert.match(pane.text("view"), /Thích văn phong ngắn/);
  assert.match(pane.text("view"), /Người dùng · đã ghim/);
  assert.match(pane.text("view"), /Tài liệu · bao-cao\.docx · đã xoá tạm/);
  assert.match(pane.text("view"), /Khôi phục/, "muc da xoa tam thi co nut Khoi phuc");
  // Loc pham vi: gui kem ?scope=document
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "memScope" &&
    el.getAttribute("data-value") === "document"));
  await tick();
  assert.match(calls.filter((c) => c.url.indexOf("/memory?") > 0).pop().url, /scope=document/);
  // Xoa mem + khoi phuc
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "memDelete"));
  await tick();
  assert.ok(calls.some((c) => c.method === "DELETE" && c.url === BASE + "memory/m1"));
  assert.match(pane.text("view"), /Đã xoá tạm — khôi phục được trong danh sách này/);
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "memRestore"));
  await tick();
  assert.ok(calls.some((c) => c.method === "POST" && c.url === BASE + "memory/m2/restore"));
  // Xoa vinh vien: phai co xac nhan, va xoa theo PHAM VI dang loc (Core khong co xoa cung tung muc)
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "memPurge"));
  await tick();
  assert.ok(calls.some((c) => c.method === "DELETE" && c.url === BASE + "memory?scope=document&confirm=true"));
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "closeView"));
  assert.ok(pane.byId("view").hasAttribute("hidden"), "nut Dong dong han man hinh phu");
});

test("man hinh Ghi nho: nguoi dung tu choi xac nhan thi khong xoa vinh vien", async () => {
  const { pane, calls } = paneWith({ "GET /memory": [200, { ok: true, result: { memories: [] } }] },
    { confirm: () => false });
  pane.click("memoryBtn");
  await tick();
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "memPurge"));
  await tick();
  assert.equal(calls.filter((c) => c.method === "DELETE").length, 0);
});

test("man hinh Nhat ky: nhan lenh bang tieng Viet + gio may, co Tai lai", async () => {
  const { pane, calls } = paneWith({
    "GET /audit": [200, { ok: true, result: { calls: [
      { id: 1, action: "writer.appendText", ok: true, ms: 12, createdAt: "2026-10-19T02:03:04Z" },
      { id: 2, action: "writer.insertTable", ok: false, ms: 30, error: "ArgumentException: bảng quá rộng",
        createdAt: "2026-10-19T02:03:05Z" },
    ] } }],
  });
  pane.click("auditBtn");
  await tick();
  const shown = pane.text("view");
  assert.match(shown, /Nhật ký hoạt động/);
  assert.match(shown, /✓ Viết thêm nội dung \(writer\.appendText, 12ms\)/, "nhan lenh lay tu AxiomPaneCore.labelFor");
  assert.match(shown, /✕ Chèn bảng/);
  assert.match(shown, /ArgumentException: bảng quá rộng/);
  assert.equal(/2026-10-19T02:03:04Z/.test(shown), false, "gio UTC cua Core phai doi sang gio may");
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "auditReload"));
  await tick();
  assert.equal(calls.filter((c) => c.url === BASE + "audit").length, 2);
});

test("man hinh Thiet lap: dien san trang thai THAT, Luu chi gui o da dien, Kiem tra Core", async () => {
  const { pane, calls } = paneWith({
    "GET /setup": [200, { ok: true, result: {
      current: { endpoint: "https://api.example/v1", model: "claude-sonnet-5-5", hasKey: true,
        visualQaEnabled: true, memoryEnabled: false, llmShowReasoning: true, tavilyEnabled: false,
        verifyWorkEnabled: true },
      problems: [{ id: "token", severity: "warning", label: "Chưa có khoá bảo vệ",
        hint: "Bấm \"Tạo khoá mới\" rồi khởi động lại Core." }],
    } }],
    "POST /config": [200, { ok: true, result: { saved: ["LlmModel"], ignored: [], current: {} } }],
    "POST /llm/test": [200, { ok: true, result: { seconds: 1.2, model: "claude-sonnet-5-5", reply: "OK" } }],
  });
  pane.click("setupBtn");
  await tick();
  assert.equal(pane.byId("set_LlmEndpoint").value, "https://api.example/v1", "o nhap dien san cau hinh dang chay");
  assert.equal(pane.byId("set_LlmModel").value, "claude-sonnet-5-5");
  assert.equal(pane.byId("set_VisualQaEnabled").checked, true);
  assert.equal(pane.byId("set_MemoryEnabled").checked, false);
  assert.equal(pane.byId("set_LlmApiKey").value, "", "khong bao gio dien san khoa API");
  assert.match(pane.text("view"), /Chưa có khoá bảo vệ/);
  assert.match(pane.text("view"), /WPS chưa chụp được trang — add-in thiếu lệnh app\.screenshot/,
    "cong tac Nhin tai lieu phai noi that la WPS chua lam duoc (test_addin.mjs: app.screenshot khong co handler)");
  pane.byId("set_LlmModel").value = "claude-opus-5-5";
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "saveSetup"));
  await tick();
  const saved = calls.find((c) => c.url === BASE + "config");
  assert.equal(saved.body.LlmModel, "claude-opus-5-5");
  assert.equal(saved.body.LlmEndpoint, "https://api.example/v1");
  assert.equal("LlmApiKey" in saved.body, false, "o khoa de trong = giu khoa da luu");
  assert.match(pane.text("view"), /Đã lưu 1 mục/);
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "testCore"));
  await tick();
  const probe = calls.find((c) => c.url === BASE + "llm/test");
  assert.ok(probe, "Kiem tra Core goi POST /wps/v1/llm/test");
  // Core doc khoa CHU THUONG (providerId/provider/endpoint/model/apiKey). Gui khoa cau hinh
  // (LlmEndpoint, LlmModel...) thi Core tra "Endpoint is not configured" va nut bao loi du cau hinh dung.
  assert.deepEqual(Object.keys(probe.body).sort(),
    ["apiKey", "endpoint", "model", "provider", "providerId"], "body phai dung khoa Core doc");
  assert.equal(probe.body.endpoint, "https://api.example/v1");
  assert.equal(probe.body.model, "claude-sonnet-5-5", "o nhap dien lai tu cau hinh dang chay sau khi Luu");
  assert.equal(probe.body.apiKey, "", "khoa API de trong = dung khoa da luu");
  assert.match(pane.text("view"), /Kết nối được với claude-sonnet-5-5 \(1\.2 giây\)\. Model trả lời: OK/);
});

test("?view=<ten> mo thang man hinh phu (ribbon bam thang vao day)", async () => {
  const { pane, calls } = paneWith({ "GET /audit": [200, { ok: true, result: { calls: [] } }] },
    { search: "?app=wps&view=audit" });
  assert.equal(pane.byId("view").hasAttribute("hidden"), false);
  assert.match(pane.text("view"), /Nhật ký hoạt động/);
  assert.ok(calls.some((c) => c.url === BASE + "audit"));
});

// ---- Man hinh "Ky nang" (ribbon: nut Ky nang -> ?view=skills) ----

test("man hinh Ky nang: loc theo app dang mo, noi ro nguon va thu muc nap loi", async () => {
  const listing = { ok: true, result: {
    skills: [
      { name: "soan-thao", description: "Viết văn bản hành chính", apps: ["wps"], source: "builtin", directory: "/a/soan-thao" },
      { name: "bang-bieu", description: "", apps: [], source: "user", directory: "/y/bang-bieu" },
    ],
    errors: [{ directory: "/z", source: "org", error: "YAML hỏng ở dòng 3" }],
    sources: [{ name: "builtin", directory: "/a", exists: true }, { name: "org", directory: "/z", exists: true },
      { name: "user", directory: "/y", exists: false }],
  } };
  const { pane, calls } = paneWith({
    "GET /skills": [200, listing],
    "POST /skills/reload": [200, listing],
  }, { search: "?app=wps&view=skills" });
  await tick();
  assert.match(calls.find((c) => c.url.indexOf("/skills?") > 0).url, /app=wps$/,
    "liet ke theo loai tai lieu dang mo (AdminForms.cs: SkillsForm)");
  const shown = pane.text("view");
  assert.match(shown, /2 kỹ năng cho Writer/);
  assert.match(shown, /1 thư mục nạp lỗi/);
  assert.match(shown, /nguồn: sẵn có, tổ chức\./, "chi liet ke thu muc CO THAT (user khong ton tai)");
  assert.match(shown, /Viết văn bản hành chính · dùng cho: wps · sẵn có/);
  assert.match(shown, /dùng cho mọi ứng dụng · cá nhân/, "ky nang khong ghi app nao thi dung cho moi app");
  assert.match(shown, /⚠ Không nạp được/);
  assert.match(shown, /\/z: YAML hỏng ở dòng 3/);
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "skillsReload"));
  await tick();
  assert.ok(calls.some((c) => c.method === "POST" && c.url === BASE + "skills/reload"), "Nap lai goi Core");
  assert.equal(calls.filter((c) => c.url.indexOf("/skills?") > 0).length, 2, "nap lai xong phai liet ke lai");
});

// ---- Man hinh "Trang thai" (ribbon: nut Trang thai -> ?view=status) ----

const STATUS = { ok: true, result: {
  addin: { connected: true, everPoll: true, app: "wps", document: { name: "bao-cao.docx", fullName: "/tmp/bao-cao.docx" },
    addinVersion: "0.1.0", running: false, queued: 0, waiting: 1, bridgePort: 47861, sessionPath: "/tmp/s.json",
    pollSeconds: 2, disconnectSeconds: 45, uptimeSeconds: 300, lastPollSeconds: 1.2 },
  core: { version: "1.0.0", pid: 4242, port: 47840, started: "2026-10-19T01:00:00.000Z", uptimeSeconds: 3725,
    dataDir: "/root/.local/share/axiomoffice", logFile: "/root/.local/share/axiomoffice/core.log",
    provider: "openai", endpoint: "https://api.example/v1", model: "claude-sonnet-5-5", configured: true,
    memory: "on", memoryError: null, skills: 12, skillErrors: 1 },
} };

test("man hinh Trang thai: mot loi goi tra ca tang add-in lan tang Core", async () => {
  const { pane, calls } = paneWith({ "GET /status": [200, STATUS] }, { search: "?app=wps&view=status" });
  await tick();
  assert.equal(calls.filter((c) => c.url === BASE + "status").length, 1, "pane khong tu hoi bridge (khac cong)");
  const shown = pane.text("view");
  assert.match(shown, /✓ Add-in: đang trả lời ở cổng 47861 \(phiên bản 0\.1\.0\)\./);
  assert.match(shown, /Tài liệu: bao-cao\.docx/);
  assert.match(shown, /Đang chờ add-in: 1 lệnh/);
  assert.match(shown, /✓ Agent Core: đang chạy \(phiên bản 1\.0\.0, pid 4242, đã chạy 1 giờ 2 phút\)\./,
    "uptime 3725s = 1 gio 2 phut (RibbonUi.Uptime)");
  assert.match(shown, /Model: claude-sonnet-5-5/);
  assert.match(shown, /Bộ nhớ AI: đang bật/);
  assert.match(shown, /Kỹ năng: 12 \(1 thư mục nạp lỗi\)/);
  assert.match(shown, /API URL: http:\/\/127\.0\.0\.1:47861\/health/);
  assert.match(shown, /Nhật ký: \/root\/\.local\/share\/axiomoffice\/core\.log/);
});

test("man hinh Trang thai: add-in tat va Core loi bo nho thi noi ro, khong bia", async () => {
  const { pane } = paneWith({ "GET /status": [200, { ok: true, result: {
    addin: { connected: false, everPoll: true, app: "", document: {}, addinVersion: "", running: false,
      queued: 0, waiting: 0, bridgePort: 47861, pollSeconds: 2, disconnectSeconds: 45, uptimeSeconds: 90,
      lastPollSeconds: 61 },
    core: { version: "1.0.0", pid: 9, port: 47840, uptimeSeconds: 30, dataDir: "/d",
      logFile: "/d/core.log", provider: "openai", endpoint: "", model: "", configured: false,
      memory: "off", memoryError: "sqlite: disk I/O error", skills: 0, skillErrors: 0 },
  } }] }, { search: "?app=wps&view=status" });
  await tick();
  const shown = pane.text("view");
  assert.match(shown, /✗ Add-in: không trả lời — lần trả lời cuối cách đây 1 phút\./);
  assert.match(shown, /AI sẽ không đọc\/sửa được tài liệu này/);
  assert.match(shown, /Model: \(chưa chọn - bấm Thiết lập…\)/);
  assert.match(shown, /✗ Bộ nhớ AI lỗi: sqlite: disk I\/O error/, "loi that phai hien nguyen van");
  assert.match(shown, /Kỹ năng: 0/, "khong co thu muc nap loi thi khong them ve sau");
});

test("man hinh Trang thai: ban Core cu (thieu truong) thi noi thang, khong bia bao cao trong", async () => {
  const { pane } = paneWith({ "GET /status": [200, { ok: true, result: { addin: {}, core: {} } }] },
    { search: "?app=wps&view=status" });
  await tick();
  const shown = pane.text("view");
  assert.match(shown, /✗ Add-in: chưa kết nối lần nào kể từ khi Agent Core khởi động\./);
  assert.match(shown, /✗ Agent Core: trả lời thiếu thông tin — bản Core này cũ hơn pane\./);
  assert.equal(/đang chạy \(phiên bản ,/.test(shown), false, "khong duoc bia ra mot dong trong nhu that");
});

test("?do=testModel: mo Trang thai la kiem tra mo hinh luon (menu Khac cua ribbon)", async () => {
  const { pane, calls } = paneWith({
    "GET /status": [200, STATUS],
    "POST /llm/test": [200, { ok: true, result: { seconds: 2.4, model: "claude-sonnet-5-5", provider: "openai", reply: "Xin chào" } }],
  }, { search: "?app=wps&view=status&do=testModel" });
  await tick();
  const probe = calls.find((c) => c.url === BASE + "llm/test");
  assert.ok(probe, "mo pane la kiem tra luon, khong doi nguoi dung bam nut");
  assert.deepEqual(probe.body, { providerId: "", provider: "openai",
    endpoint: "https://api.example/v1", model: "claude-sonnet-5-5", apiKey: "" });
  assert.match(pane.text("view"), /Kết nối được với claude-sonnet-5-5 \(2\.4 giây\)\. Model trả lời: Xin chào/);
});

test("?do=testModel khi chua cau hinh AI: nhac mo Thiet lap, khong goi Core", async () => {
  const { pane, calls } = paneWith({ "GET /status": [200, { ok: true, result: {
    addin: { connected: true, bridgePort: 47861, document: {}, queued: 0, waiting: 0, uptimeSeconds: 5 },
    core: { version: "1.0.0", pid: 1, uptimeSeconds: 5, logFile: "/d/core.log", endpoint: "", model: "",
      configured: false, memory: "on", skills: 0, skillErrors: 0 },
  } }] }, { search: "?app=wps&view=status&do=testModel" });
  await tick();
  assert.equal(calls.filter((c) => c.url === BASE + "llm/test").length, 0);
  assert.match(pane.text("view"), /Chưa cấu hình AI\. Bấm Thiết lập… để chọn máy chủ AI và model\./);
});

test("?do=copyUrl: chep dia chi bridge that (/health tra loi, khong phai '/' la 404)", async () => {
  const written = [];
  const { pane } = paneWith({ "GET /status": [200, STATUS] },
    { search: "?app=wps&view=status&do=copyUrl",
      navigator: { clipboard: { writeText: (text) => { written.push(text); return Promise.resolve(); } } } });
  await tick();
  assert.deepEqual(written, ["http://127.0.0.1:47861/health"]);
  assert.match(pane.text("view"), /Đã chép vào clipboard: http:\/\/127\.0\.0\.1:47861\/health/);
  // Bam lai nut trong man hinh cung chep lai duoc (khong chi mot lan luc mo pane)
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "statusCopyUrl"));
  assert.equal(written.length, 2);
});

test("?do=copyUrl khi CEF khong cho chep: van hien dia chi de nguoi dung chon tay", async () => {
  const { pane } = paneWith({ "GET /status": [200, STATUS] },
    { search: "?app=wps&view=status&do=copyUrl" });
  await tick();
  assert.match(pane.text("view"), /Không tự chép được — chọn và sao chép: http:\/\/127\.0\.0\.1:47861\/health/);
});

// ---- Man hinh "Soat tai lieu" (ribbon: ba nut Soat ... -> ?view=check&check=<lenh>) ----

test("man hinh Soat: chay dung lenh trong ?check= va doc bao cao bang tieng Viet", async () => {
  const { pane, calls } = paneWith({
    "POST /exec": [200, { ok: true, result: { tableCount: 3, issueCount: 3, issues: [
      { type: "empty-cells", detail: "Bảng 2: ô (3,2) trống" },
      { type: "long-header-cell" },
      { type: "khong-biet" },
    ] } }],
  }, { search: "?app=wps&view=check&check=writer.checkTables" });
  await tick();
  assert.equal(pane.byId("view").hasAttribute("hidden"), false);
  assert.deepEqual(calls.find((c) => c.url === BASE + "exec").body,
    { action: "writer.checkTables", params: {} }, "nut Soat bang chay dung lenh cua no");
  const shown = pane.text("view");
  assert.match(shown, /Soát bảng/);
  assert.match(shown, /3 bảng trong tài liệu/);
  assert.match(shown, /3 vấn đề:/);
  assert.match(shown, /• Có ô trống — Bảng 2: ô \(3,2\) trống/);
  assert.match(shown, /• Ô tiêu đề quá dài•/, "khong co detail thi khong them dau gach");
  assert.match(shown, /• khong-biet/, "loai la thi hien chinh ten do, khong nuot van de");
  assert.match(shown, /Đã soát bảng trong văn bản · chỉ đọc, tài liệu không bị sửa/);
});

test("man hinh Soat: dong boi canh doi theo tung lenh (giong checks.py)", async () => {
  const cases = [
    ["wpp.checkLayout", { slideWidth: 960, slideHeight: 540, slides: [{}, {}], issues: [], issueCount: 0 },
      /2 trang · khổ 960×540 pt/],
    ["et.checkRange", { sheet: "Sheet1", range: "A1:D9", rows: 9, cols: 4, issues: [], issueCount: 0 },
      /Trang tính Sheet1 · vùng A1:D9 · 9 dòng × 4 cột/],
    ["writer.checkTables", { tableCount: 0, issues: [], issueCount: 0 }, /0 bảng trong tài liệu/],
  ];
  for (const [command, result, context] of cases) {
    const { pane } = paneWith({ "POST /exec": [200, { ok: true, result }] },
      { search: "?app=wps&view=check&check=" + command });
    await tick();
    const shown = pane.text("view");
    assert.match(shown, context, command);
    assert.match(shown, /Không phát hiện vấn đề\./, command);
    assert.equal(/vấn đề:/.test(shown), false, command + ": khong co van de thi khong dem");
  }
});

test("man hinh Soat: 409 cua Core duoc dich, va nut 'Soat lai' chay lai dung lenh", async () => {
  let attempt = 0;
  const { pane, calls } = paneWith({
    "POST /exec": () => {
      attempt += 1;
      return attempt === 1
        ? [409, { ok: false, error: "a run is in progress: wait for it to finish", code: "run-in-progress" }]
        : [409, { ok: false, error: "WPS add-in is not connected: open a document in WPS to connect the add-in",
          code: "addin-disconnected" }];
    },
  }, { search: "?app=wps&view=check&check=wpp.checkLayout" });
  await tick();
  let shown = pane.text("view");
  assert.match(shown, /Đang có một lượt AI chạy — đợi lượt đó xong rồi soát lại\./);
  assert.equal(/run is in progress/.test(shown), false, "khong hien chuoi tieng Anh cua Core");
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "checkRun"));
  await tick();
  shown = pane.text("view");
  assert.match(shown, /Add-in chưa kết nối: mở một tài liệu trong WPS rồi thử lại\./);
  assert.deepEqual(calls.filter((c) => c.url === BASE + "exec").map((c) => c.body.action),
    ["wpp.checkLayout", "wpp.checkLayout"]);
});

test("man hinh Soat: mo ma thieu ?check= thi khong chay lenh nao", async () => {
  const { pane, calls } = paneWith({}, { search: "?app=wps&view=check" });
  await tick();
  assert.equal(calls.filter((c) => c.url === BASE + "exec").length, 0);
  assert.match(pane.text("view"), /Thiếu tên lệnh soát — mở màn hình này từ nút trên ribbon\./);
});

// ---- Man hinh "Nhat ky Core" (ribbon: nut Mo nhat ky -> ?view=log) ----

test("man hinh Nhat ky Core: doc duoi file log, 'Xem them' xin nhieu dong hon", async () => {
  const { pane, calls } = paneWith({
    "GET /log": ({ url }) => {
      const more = url.indexOf("lines=400") > 0;
      return [200, { ok: true, result: { path: "/root/.local/share/axiomoffice/core.log", exists: true, size: 900,
        modified: "2026-10-19T02:00:00.000Z", lines: more ? ["a", "b", "c"] : ["b", "c"],
        truncated: !more, count: more ? 3 : 2 } }];
    },
  }, { search: "?app=wps&view=log" });
  await tick();
  assert.match(calls.find((c) => c.url.indexOf("/log?") > 0).url, /lines=200$/, "mac dinh 200 dong cuoi");
  const shown = pane.text("view");
  assert.match(shown, /Nhật ký Core/);
  assert.match(shown, /2 dòng cuối \(còn dòng cũ hơn\)/);
  assert.match(shown, /\/root\/\.local\/share\/axiomoffice\/core\.log/);
  // Moi dong log la mot <p class="auditLine"> (text cua DOM gia noi lien nhau nen dem theo the, khong
  // do chuoi).
  const rows = () => pane.findAll((el) => el.className === "auditLine").map((el) => el.text);
  assert.deepEqual(rows(), ["b", "c"]);
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "logMore"));
  await tick();
  assert.match(calls.filter((c) => c.url.indexOf("/log?") > 0).pop().url, /lines=400$/);
  assert.deepEqual(rows(), ["a", "b", "c"], "Xem them lay them dong cu hon");
  assert.match(pane.text("view"), /3 dòng cuối ·/);
});

test("man hinh Nhat ky Core: Core chua ghi dong nao thi noi ro, khong hien loi do", async () => {
  const { pane } = paneWith({
    "GET /log": [200, { ok: true, result: { path: "/d/core.log", exists: false, lines: [], truncated: false } }],
  }, { search: "?app=wps&view=log" });
  await tick();
  const shown = pane.text("view");
  assert.match(shown, /Chưa có nhật ký — Core chưa ghi dòng nào kể từ khi cài\./);
  assert.equal(/Lỗi|error/i.test(shown), false);
});

// ---- Hop dong bao cao soat tai lieu: pane.html va checks.py phai noi cung mot thu tieng ----

test("bang nhan van de cua pane.html khop tung dong voi checks.py", () => {
  const python = fs.readFileSync(CHECKS_PY, "utf8");
  const html = fs.readFileSync(PANE, "utf8");
  const read = (source, open, close, line) => {
    const block = new RegExp(open + "([\\s\\S]*?)" + close).exec(source);
    assert.ok(block, "khong doc duoc bang nhan trong: " + open);
    const out = {};
    for (const raw of block[1].split("\n")) {
      const found = line.exec(raw);
      if (found) { out[found[1]] = found[2]; }
    }
    return out;
  };
  const want = read(python, "ISSUE_LABELS = \\{", "\\n\\}", /^\s*"([^"]+)":\s*"([^"]+)",\s*$/);
  const got = read(html, "var ISSUE_LABELS = \\{", "\\n  \\};", /^\s*"?([A-Za-z-]+)"?:\s*"([^"]+)",?\s*$/);
  assert.ok(Object.keys(want).length >= 14, "doc duoc bang nhan cua checks.py: " + Object.keys(want).length);
  assert.deepEqual(got, want, "ISSUE_LABELS cua pane.html phai khop checks.py (ca hai chieu)");
  // Cau bao cao cung la mot hop dong: moi manh chuoi tieng Viet phai co o CA HAI file.
  for (const fragment of ["Không phát hiện vấn đề.", " vấn đề:", "• ", " — ", " trang · khổ ", " pt",
    "Trang tính ", " · vùng ", " dòng × ", " cột", " bảng trong tài liệu"]) {
    assert.ok(python.includes(fragment), "checks.py khong con manh chuoi: " + JSON.stringify(fragment));
    assert.ok(html.includes(fragment), "pane.html thieu manh chuoi bao cao: " + JSON.stringify(fragment));
  }
});

test("pane.html: id moi co that trong markup va #view la anh em cua #app", () => {
  const html = fs.readFileSync(PANE, "utf8");
  for (const [id, why] of [["planToggle", "cong tac lap ke hoach"], ["undoBtn", "nut Hoan tac luot nay"],
    ["insertBtn", "nut Chen tra loi"], ["view", "man hinh phu (Ghi nho / Nhat ky / Thiet lap)"],
    ["memoryBtn", "loi vao Ghi nho"], ["auditBtn", "loi vao Nhat ky"], ["setupBtn", "loi vao Thiet lap"]]) {
    assert.match(html, new RegExp("id\\s*=\\s*[\"']" + id + "[\"']"), "pane.html thieu #" + id + " (" + why + ")");
  }
  // #view phai nam NGOAI #app: no phu ca pane, va listener bam duoc gan rieng (bindClicks).
  assert.match(html, /<\/div>\s*<div id="view"/, "#view phai la anh em dung sau #app");
  assert.match(html, /bindClicks\(\$\("view"\)\)/, "man hinh phu phai duoc gan listener bam");
});

// ---- Menu "..." tren header (Word Copilot: tieu de + mot nut ... o goc phai) ----

const menuItem = (pane, act) => pane.findAll((el) => el.getAttribute && el.getAttribute("data-act") === act)[0];

test("header: nut '...' mo/dong menu, bon loi vao cu nam trong menu", () => {
  const { pane } = paneWith({});
  const menu = pane.byId("moreMenu");
  assert.ok(menu, "pane.html thieu #moreMenu");
  assert.ok(menu.hasAttribute("hidden"), "menu phai dong luc dau");
  assert.equal(pane.byId("moreBtn").getAttribute("aria-expanded"), "false");
  // Ghi nho / Nhat ky / Thiet lap / Tro chuyen moi gio nam trong menu, header chi con tieu de + nut "...".
  for (const id of ["newBtn", "historyBtn", "memoryBtn", "auditBtn", "setupBtn"]) {
    assert.equal(pane.byId(id).parentNode, menu, "#" + id + " phai nam trong menu '...'");
  }
  pane.click("moreBtn");
  assert.ok(!menu.hasAttribute("hidden"), "bam '...' thi menu mo");
  assert.equal(pane.byId("moreBtn").getAttribute("aria-expanded"), "true");
  pane.click("moreBtn");
  assert.ok(menu.hasAttribute("hidden"), "bam '...' lan nua thi menu dong");
  assert.equal(pane.byId("moreBtn").getAttribute("aria-expanded"), "false");
});

test("header: Esc va bam ra ngoai deu dong menu", () => {
  const { pane } = paneWith({});
  const menu = pane.byId("moreMenu");
  pane.click("moreBtn");
  assert.ok(!menu.hasAttribute("hidden"));
  press(pane.byId("moreBtn"), 27);
  assert.ok(menu.hasAttribute("hidden"), "Esc phai dong menu");
  // Bam vao cho khong co lenh nao (khung chat) cung dong menu.
  pane.click("moreBtn");
  assert.ok(!menu.hasAttribute("hidden"));
  pane.click(pane.byId("chat"));
  assert.ok(menu.hasAttribute("hidden"), "bam ra ngoai menu thi menu dong");
});

test("header: chon mot muc trong menu thi menu dong va mo dung man hinh", async () => {
  const { pane } = paneWith({ "GET /setup": [200, { ok: true, result: { settings: [], values: {} } }] });
  pane.click("moreBtn");
  assert.ok(!pane.byId("moreMenu").hasAttribute("hidden"));
  pane.click("setupBtn");
  assert.ok(pane.byId("moreMenu").hasAttribute("hidden"), "chon muc xong thi menu phai dong");
  await tick();
  assert.match(pane.text("view"), /Thiết lập/, "mo dung man hinh cua muc vua chon");
});

// ---- Man hinh "Hoi thoai truoc" (muc dau tien trong menu "...") ----

const CONVOS = { conversations: [
  { id: "c1", title: "Giáo án tuần 3", documentName: "giaoan.docx", app: "wps",
    createdAt: "2026-10-18T09:00:00Z", updatedAt: "2026-10-19T08:31:07Z" },
  { id: "c2", title: "", summary: "Bảng điểm lớp 9A", documentName: "diem.xlsx", app: "et",
    createdAt: "2026-10-17T10:00:00Z", updatedAt: "2026-10-17T10:05:00Z" },
] };
const CONVO_C1 = { id: "c1", title: "Giáo án tuần 3", documentName: "giaoan.docx", app: "wps",
  createdAt: "2026-10-18T09:00:00Z", updatedAt: "2026-10-19T08:31:07Z",
  messages: [
    { seq: 1, role: "user", content: "Dọn lại bố cục", createdAt: "2026-10-18T09:00:01Z" },
    { seq: 2, role: "assistant", content: "Đã sửa 3 đoạn.", createdAt: "2026-10-18T09:00:09Z" },
  ] };

test("Hoi thoai truoc: danh sach roi mo mot hoi thoai (chi doc)", async () => {
  const { pane, url } = paneWith({
    "GET /conversations": [200, { ok: true, result: CONVOS }],
    "GET /conversations/c1": [200, { ok: true, result: CONVO_C1 }],
  }, { search: "?app=wps&view=conversations" });
  await tick();
  assert.equal(url("/conversations").length, 1, "mo man hinh thi xin danh sach mot lan");
  const shown = pane.text("view");
  assert.match(shown, /Hội thoại trước/);
  assert.match(shown, /Giáo án tuần 3/);
  assert.match(shown, /Bảng điểm lớp 9A/, "khong co title thi lay summary");
  assert.match(shown, /2026-10-19 08:31/, "hien ngay gio sua cuoi");
  assert.match(shown, /2 hội thoại gần đây/);
  // Bam mot dong -> xin noi dung hoi thoai do, van la man hinh chi doc.
  pane.click(menuItem(pane, "convoOpen"));
  await tick();
  assert.equal(url("/conversations/c1").length, 1, "bam dong thi xin chi tiet hoi thoai");
  const detail = pane.text("view");
  assert.match(detail, /Dọn lại bố cục/);
  assert.match(detail, /Đã sửa 3 đoạn/);
  assert.match(detail, /Writer/, "phu de doi chieu app cua hoi thoai -> ten ban");
  // Cau nguoi dung / cau AI dung lai dung hai kieu bong bong cua khung chat.
  const bubbles = pane.findAll((el) => typeof el.className === "string" && el.className.indexOf("bubble") === 0);
  assert.deepEqual(bubbles.map((el) => el.className), ["bubble bubbleUser", "bubble bubbleAi"]);
  // "Quay lai" tra ve danh sach.
  pane.click(menuItem(pane, "convoBack"));
  await tick();
  assert.match(pane.text("view"), /Hội thoại trước/);
  assert.equal(url("/conversations").length, 2, "quay lai thi xin lai danh sach");
});

test("Hoi thoai truoc: chua co hoi thoai nao thi noi ro, khong hien loi do", async () => {
  const { pane } = paneWith({ "GET /conversations": [200, { ok: true, result: { conversations: [] } }] },
    { search: "?app=wps&view=conversations" });
  await tick();
  assert.match(pane.text("view"), /Chưa có hội thoại nào được lưu/);
});

test("Hoi thoai truoc: xoa phai hoi truoc, dong y thi goi DELETE roi quay ve danh sach", async () => {
  const asked = [];
  const { pane, url } = paneWith({
    "GET /conversations": [200, { ok: true, result: CONVOS }],
    "GET /conversations/c1": [200, { ok: true, result: CONVO_C1 }],
    "DELETE /conversations/c1": [200, { ok: true, result: { deleted: "c1" } }],
  }, { search: "?app=wps&view=conversations", confirm: (text) => { asked.push(text); return true; } });
  await tick();
  pane.click(menuItem(pane, "convoOpen"));
  await tick();
  pane.click(menuItem(pane, "convoDelete"));
  await tick();
  assert.equal(asked.length, 1, "phai hoi nguoi dung truoc khi xoa");
  assert.match(asked[0], /Không khôi phục được/);
  const deletes = url("/conversations/c1").filter((c) => c.method === "DELETE");
  assert.equal(deletes.length, 1, "dong y thi goi DELETE dung hoi thoai dang mo");
  assert.match(pane.text("view"), /Đã xoá hội thoại/);
  assert.match(pane.text("view"), /Hội thoại trước/, "xoa xong quay ve danh sach");
});

test("Hoi thoai truoc: tu choi xac nhan thi khong xoa gi", async () => {
  const { pane, calls } = paneWith({
    "GET /conversations": [200, { ok: true, result: CONVOS }],
    "GET /conversations/c1": [200, { ok: true, result: CONVO_C1 }],
    "DELETE /conversations/c1": [200, { ok: true, result: { deleted: "c1" } }],
  }, { search: "?app=wps&view=conversations", confirm: () => false });
  await tick();
  pane.click(menuItem(pane, "convoOpen"));
  await tick();
  pane.click(menuItem(pane, "convoDelete"));
  await tick();
  assert.equal(calls.filter((c) => c.method === "DELETE").length, 0, "tu choi thi khong goi DELETE");
  assert.match(pane.text("view"), /Dọn lại bố cục/, "van o nguyen hoi thoai dang mo");
});

// ---- "Sua bang AI": danh sach muc lay tu Core. Loi vao duy nhat la menu chuot phai (khung chat khong ve menu rieng).
const QUICK_LIST = { ok: true, result: { actions: [
  { id: "polish", label: "Trau chuốt", edits: true },
  { id: "fix", label: "Sửa lỗi chính tả", edits: true },
  { id: "summarize", label: "Tóm tắt", edits: false },
] } };
test("Sua bang AI KHONG nam trong khung chat (20/10/2026: loi vao duy nhat la menu chuot phai)", async () => {
  const { pane } = paneWith({ "GET /quick-actions": [200, QUICK_LIST] });
  await tick();
  assert.equal(pane.byId("quickBtn"), null, "khong co nut Sua bang AI o chan o soan");
  assert.equal(pane.byId("quickMenu"), null);
  assert.equal(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "quick"), null);
  assert.equal(pane.text("chat").includes("Sửa bằng AI"), false);
});

// ---- Menu chuot phai -> khung chat nay: Core chuyen muc qua SSE pane/events (api/wpspane.go).
const paneSource = (pane) => pane.sources.find((s) => s.url.endsWith("/wps/v1/pane/events"));

test("chuot phai: khung chat Writer nghe pane/events; nhan pane.quick -> chay muc do TRONG hoi thoai dang mo", async () => {
  const { pane, calls } = paneWith({ "GET /quick-actions": [200, QUICK_LIST],
    "POST /runs": () => [200, { ok: true, result: { runId: "r" + calls.length, conversationId: "c1" } }] });
  await tick();
  const source = paneSource(pane);
  assert.ok(source, "khung chat Writer phai mo EventSource toi /wps/v1/pane/events");
  await startRun(pane, "Chào");
  pane.emit("run.completed", { reply: "Chào bạn" });
  await tick();
  source.emit("pane.quick", { quickAction: "fix" });
  await tick();
  const posts = calls.filter((c) => c.method === "POST" && c.url.endsWith("/runs"));
  assert.deepEqual(posts[1].body, { quickAction: "fix", conversationId: "c1" }, "giu hoi thoai dang mo");
  assert.match(pane.text("chat"), /Sửa lỗi chính tả/, "bong bong hien NHAN cua muc");
});

test("chuot phai: dang co luot chay -> bao ro bi bo qua, khong chay chen", async () => {
  const { pane, calls } = paneWith({ "GET /quick-actions": [200, QUICK_LIST],
    "POST /runs": [200, { ok: true, result: { runId: "r1" } }] });
  await tick();
  await startRun(pane, "Việc dài");
  paneSource(pane).emit("pane.quick", { quickAction: "polish" });
  await tick();
  assert.equal(calls.filter((c) => c.method === "POST" && c.url.endsWith("/runs")).length, 1, "khong gui luot thu hai");
  assert.match(pane.text("chat"), /Đang có một lượt chạy — chờ xong rồi chọn lại “Trau chuốt”/);
});

test("chuot phai: chua boi den -> the loi tieng Viet bao phai boi den truoc", async () => {
  const { pane } = paneWith({ "GET /quick-actions": [200, QUICK_LIST],
    "POST /runs": [200, { ok: true, result: { runId: "q2" } }] });
  await tick();
  paneSource(pane).emit("pane.quick", { quickAction: "summarize" });
  await tick();
  pane.emit("run.failed", { error: "quick action \"summarize\" works on the selected text: select it first", kind: "selection" });
  await tick();
  assert.match(pane.text("chat"), /Hãy bôi đen đoạn văn cần làm trước/);
  assert.equal(pane.text("chat").includes("select it first"), false);
});

test("chuot phai: man hinh phu (?view=) va Calc khong nghe pane/events", async () => {
  const view = paneWith({}, { search: "?app=wps&view=memory" });
  const calc = paneWith({}, { search: "?app=et" });
  await tick();
  assert.equal(paneSource(view.pane), undefined, "man hinh phu khong chay luot");
  assert.equal(paneSource(calc.pane), undefined, "Calc khong co menu nay");
});

// ---- Chan o soan: goi y phim tat KHONG xuong dong (anh chup 20/10/2026: cau day du gay hai dong, dinh vao
// chu "Sua bang AI"). Do cho trong that: day du -> ngan -> bo han; cau day du van o tooltip nut Gui.
function measureHint(pane, room) {
  const hint = pane.byId("composerHint");
  // Layout gia: moi ky tu ~6px, o rong `room` px.
  Object.defineProperty(hint, "clientWidth", { configurable: true, get: () => room });
  Object.defineProperty(hint, "scrollWidth", { configurable: true, get: () => hint.text.length * 6 });
  return hint;
}

test("chan o soan: goi y vua cho trong THAT; khong du cho thi rut ngan roi bo, tooltip nut Gui van con", async () => {
  for (const [room, want] of [[300, "Enter để gửi · Shift+Enter xuống dòng"], [100, "Enter để gửi"], [40, ""]]) {
    const { pane } = paneWith({ "GET /quick-actions": [200, QUICK_LIST] });
    measureHint(pane, room);
    await tick(); // nap xong danh sach muc -> render -> updateHint do lai voi layout gia
    assert.equal(pane.text("composerHint"), want, "o rong " + room + "px");
  }
  const { pane } = paneWith({});
  assert.equal(pane.byId("sendBtn").getAttribute("title"), "Enter để gửi · Shift+Enter xuống dòng",
    "bo goi y o chan thi phim tat van doc duoc o tooltip nut Gui");
});

test("chan o soan: chua do duoc layout -> doan theo be rong o soan", () => {
  // DOM gia khong co clientWidth/offsetWidth: pane rong 420px (pane_dom) - du cho cau day du.
  const { pane } = paneWith({});
  assert.equal(pane.text("composerHint"), "Enter để gửi · Shift+Enter xuống dòng");
  const narrow = paneWith({});
  narrow.pane.context.innerWidth = 300;
  narrow.pane.click("planToggle"); // bat ky thao tac nao cung ve lai
  assert.equal(narrow.pane.text("composerHint"), "Enter để gửi");
});
