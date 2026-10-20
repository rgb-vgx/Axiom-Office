// Spec: khung chat mini canh doan boi den (src/AxiomOffice.WPS/mini.html) - mo tu menu chuot phai bang
// Application.ShowDialog (main.js: openMini). Dung chung pane-core.js voi khung chat lon; khac duy nhat la moi luot
// deu gui options.requireSelection (Core dung ngay neu khong co doan boi den).
//
//   node --test tests/wps/test_mini_view.mjs
//
// DOM gia o tests/wps/pane_dom.mjs - KHONG mo trinh duyet (trang chi song trong CEF cua WPS).
import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { loadPane, press, ADDIN } from "./pane_dom.mjs";

const BASE = "http://127.0.0.1:47840/wps/v1/";
const tick = () => new Promise((resolve) => setTimeout(resolve, 10));
const QUICK_LIST = { ok: true, result: { actions: [
  { id: "polish", label: "Trau chuốt", edits: true },
  { id: "fix", label: "Sửa lỗi chính tả", edits: true },
  { id: "summarize", label: "Tóm tắt", edits: false },
] } };

function mini(routes = {}, search = "?app=wps") {
  const calls = [];
  const fetch = (url, options = {}) => {
    const method = (options.method || "GET").toUpperCase();
    const body = options.body ? JSON.parse(options.body) : null;
    calls.push({ url: String(url), method, body });
    for (const key of Object.keys(routes)) {
      const [wantMethod, wantPath] = key.split(" ");
      if (method !== wantMethod || !String(url).split("?")[0].endsWith(wantPath)) { continue; }
      const reply = typeof routes[key] === "function" ? routes[key]({ url, method, body }) : routes[key];
      return Promise.resolve({ ok: reply[0] < 400, status: reply[0], json: () => Promise.resolve(reply[1]) });
    }
    return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ ok: true, result: {} }) });
  };
  const pane = loadPane({ search, fetch, page: "mini.html" });
  const runs = () => calls.filter((c) => c.method === "POST" && c.url === BASE + "runs");
  return { pane, calls, runs };
}

const RUN = (id) => [200, { ok: true, result: { runId: id, conversationId: "c-mini" } }];

test("mini.html: cung he thiet ke voi pane.html - :root CHEP NGUYEN, khong mau ngoai :root, chuyen dong 150-300ms", () => {
  const cssOf = (file) => fs.readFileSync(path.join(ADDIN, file), "utf8");
  const rootOf = (css) => /:root\{[\s\S]*?\n\}/.exec(css)[0];
  assert.equal(rootOf(cssOf("mini.css")), rootOf(cssOf("pane.css")), "doi token o pane.css thi phai chep sang mini.css");
  const css = cssOf("mini.css").replace(/:root\{[\s\S]*?\n\}/, "");
  assert.deepEqual(css.match(/#[0-9a-fA-F]{3,8}\b/g) || [], [], "mau phai di qua token --axiom-*");
  for (const ms of css.match(/\b(\d+)ms\b/g) || []) {
    const n = Number.parseInt(ms, 10);
    assert.ok(n >= 150 && n <= 300, "chuyen dong " + ms + " ngoai 150-300ms");
  }
  assert.match(css, /prefers-reduced-motion/, "ton trong tuy chon giam chuyen dong");
});

test("mo tu menu chuot phai (?quick=polish): chay NGAY muc do, bat buoc doan boi den, bong bong hien nhan", async () => {
  const { pane, runs } = mini({ "GET /quick-actions": [200, QUICK_LIST], "POST /runs": RUN("m1") },
    "?app=wps&quick=polish");
  await tick();
  assert.equal(runs().length, 1, "mo ra la chay, khong bat nguoi dung bam them");
  assert.deepEqual(runs()[0].body, { quickAction: "polish", options: { requireSelection: true } });
  assert.match(pane.text("miniBody"), /Trau chuốt/);
  assert.equal(pane.byId("miniStop").hasAttribute("hidden"), false, "dang chay thi co nut Dung");
  assert.equal(pane.byId("miniSend").disabled, true);
});

test("mo tu 'Hoi AI ve doan nay': hien chip tu danh sach cua Core; bam chip hoac go yeu cau deu bat buoc boi den", async () => {
  const { pane, runs } = mini({ "GET /quick-actions": [200, QUICK_LIST], "POST /runs": RUN("m2") });
  await tick();
  assert.equal(runs().length, 0, "khong co ?quick= thi cho nguoi dung chon");
  assert.match(pane.text("miniBody"), /Chọn việc cần làm với đoạn đang bôi đen/);
  const chips = pane.findAll((el) => el.getAttribute && el.getAttribute("data-act") === "quick");
  assert.deepEqual(chips.map((c) => c.text), ["Trau chuốt", "Sửa lỗi chính tả", "Tóm tắt"]);
  pane.click(chips[1]);
  await tick();
  assert.deepEqual(runs()[0].body, { quickAction: "fix", options: { requireSelection: true } });
  pane.emit("run.completed", { reply: "Đã sửa 2 lỗi" });
  await tick();
  // Go them cho CUNG doan: Enter gui, giu hoi thoai, van bat buoc doan boi den (khong sua ca tai lieu).
  pane.byId("miniInput").value = "Ngắn hơn nữa";
  press(pane.byId("miniInput"), 13);
  await tick();
  assert.deepEqual(runs()[1].body,
    { prompt: "Ngắn hơn nữa", conversationId: "c-mini", options: { requireSelection: true } });
  assert.equal(pane.byId("miniInput").value, "", "gui xong thi xoa o nhap");
});

test("ket qua: buoc thao tac co nhan tieng Viet, tra loi khong lo **, Hoan tac chay writer.undo {count}", async () => {
  const { pane, calls } = mini({ "GET /quick-actions": [200, QUICK_LIST], "POST /runs": RUN("m3"),
    "POST /exec": [200, { ok: true, result: { undone: 1 } }] }, "?app=wps&quick=fix");
  await tick();
  pane.emit("tool.started", { callId: "k1", action: "writer.replaceAll" });
  pane.emit("tool.finished", { callId: "k1", action: "writer.replaceAll", ok: true });
  pane.emit("run.completed", { reply: "Đã sửa **TP. Hà Nội**", seconds: 6, rounds: 2 });
  await tick();
  const body = pane.text("miniBody");
  assert.match(body, /✓ Thay thế văn bản/);
  assert.match(body, /Đã sửa TP\. Hà Nội/);
  assert.equal(body.includes("**"), false);
  assert.equal(pane.byId("miniUndo").hasAttribute("hidden"), false, "co sua tai lieu thi co Hoan tac");
  assert.equal(pane.byId("miniStop").hasAttribute("hidden"), true);
  assert.match(pane.text("miniStatus"), /1 thao tác/);
  pane.click("miniUndo");
  await tick();
  assert.deepEqual(calls.find((c) => c.url === BASE + "exec").body, { action: "writer.undo", params: { count: 1 } });
});

test("chua boi den: Core dung (kind 'selection') -> cau tieng Viet, khong lo chuoi tieng Anh", async () => {
  const { pane } = mini({ "GET /quick-actions": [200, QUICK_LIST], "POST /runs": RUN("m4") }, "?app=wps&quick=polish");
  await tick();
  pane.emit("run.failed", { error: "quick action \"polish\" works on the selected text: select it first", kind: "selection" });
  await tick();
  assert.match(pane.text("miniBody"), /Hãy bôi đen đoạn văn cần làm trước, rồi thử lại\./);
  assert.equal(pane.text("miniBody").includes("select it first"), false);
});

test("Dong (nut hoac Esc): dang chay thi DUNG luot truoc roi moi dong cua so", async () => {
  const { pane, calls } = mini({ "GET /quick-actions": [200, QUICK_LIST], "POST /runs": RUN("m5") },
    "?app=wps&quick=polish");
  await tick();
  pane.click("miniClose");
  await tick();
  assert.ok(calls.some((c) => c.url === BASE + "runs/m5/cancel"), "khong de luot chay ngam sau khi dong");
  assert.equal(pane.context.closed, 1);
  const idle = mini({ "GET /quick-actions": [200, QUICK_LIST] });
  await tick();
  press(idle.pane.byId("miniInput"), 27);
  assert.equal(idle.pane.context.closed, 1, "Esc dong khung");
  assert.equal(idle.calls.some((c) => c.url.endsWith("/cancel")), false, "khong co luot thi khong goi huy");
});

test("xac nhan giua luot: the co Dong y / Khong", async () => {
  const { pane, calls } = mini({ "GET /quick-actions": [200, QUICK_LIST], "POST /runs": RUN("m6") },
    "?app=wps&quick=polish");
  await tick();
  pane.emit("confirm.required", { confirmationId: "x1", action: "writer.save", reason: "Lưu tài liệu?" });
  await tick();
  assert.match(pane.text("miniBody"), /Lưu tài liệu\?/);
  pane.click(pane.find((el) => el.getAttribute && el.getAttribute("data-act") === "deny"));
  await tick();
  assert.deepEqual(calls.find((c) => c.url === BASE + "runs/m6/confirm").body, { confirmationId: "x1", approved: false });
});
