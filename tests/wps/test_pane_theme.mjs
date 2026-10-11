// Spec T4c (docs/wps-bridge.md muc 7): pane Ask AI trong WPS phai dung CHUNG he thiet ke voi pane
// LibreOffice. theme.py la ban LibreOffice cua PaneTheme (src/AxiomOffice/Ai/PaneControls.cs) va tu khai
// "cung mau, co chu, khoang cach voi pane cua MS Office/WPS de hai ban trong nhu mot san pham" - nen pane
// WPS khong duoc tu nghi ra bang mau rieng (ban cu dung mau Google Material: #e8f0fe, #f1f3f4, #f9ab00...).
//
//   node --test tests/wps/test_pane_theme.mjs
//
// Hop dong:
// - src/AxiomOffice.WPS/pane.html khai bao khoi `:root` dau tien trong <style>, moi token cua theme.py mot
//   custom property. Ten: `--axiom-` + ten hang so theme.py viet thuong, `_` -> `-`
//   (PANE_BG -> --axiom-pane-bg, SIZE_TOOL_ID -> --axiom-size-tool-id, BUBBLE_MAX_USER -> --axiom-bubble-max-user).
//   Gia tri: mau la #RRGGBB; so do la px (ti le 0..1 la %); co chu la pt (theme.py tinh bang diem, CSS pt
//   dung 1pt = 96/72px nen hai ben ra cung so pixel). MOI token phai duoc dung that (var(--axiom-...) >= 1 lan)
//   va KHONG duoc co mau nao khac ngoai khoi :root (khong viet #hex trong rule, khong #fff/#abc).
// - pane.html giu chuoi tieng Viet cua pane LibreOffice: PROMPT_PLACEHOLDER/PROMPT_HINT/PROMPT_HINT_SHORT
//   (theme.py) va cac nhan cua panel.py/chatview.py ("GỢI Ý", "Sẵn sàng", "Đang chạy", "Cần bạn xác nhận: ",
//   "Có lỗi xảy ra", "Gửi", "Dừng", "Trò chuyện mới", "Đồng ý", "Từ chối").
// - pane-core.js mang phan du lieu cua he thiet ke (de test duoc khong can DOM): AxiomPaneCore.ACTION_LABELS,
//   SUFFIX_LABELS, TOOL_LABELS, NOUNS, APP_NAMES, SUGGESTIONS phai BANG theme.py tung dong; labelFor(action)
//   giong theme.label_for; emptyState(kind) giong theme.empty_state; plainReply(text) giong theme.plain_reply.
// - main.js: paneUrl() them "?app=<wps|et|wpp>" (currentAppKind) de pane biet dang o app nao; pane.html doc lai
//   tham so do (khong co thi coi la "wps").
import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
const ADDIN = path.join(ROOT, "src", "AxiomOffice.WPS");
const PANE = path.join(ADDIN, "pane.html");

// He thiet ke cua pane LibreOffice (nguon doi chieu duy nhat - khong chep tay so nao vao day).
function theme() {
  return JSON.parse(execFileSync("python3", [path.join(ROOT, "tests", "wps", "theme_dump.py")],
    { encoding: "utf8", maxBuffer: 8 * 1024 * 1024 }));
}
function paneHtml() {
  return fs.readFileSync(PANE, "utf8");
}
function slug(name) {
  return "--axiom-" + name.toLowerCase().replace(/_/g, "-");
}
// Noi dung khoi :root (bo chu thich) -> { "pane-bg": "#ffffff", ... } theo ten khong co tien to.
function rootVars(html) {
  const start = html.indexOf(":root");
  assert.ok(start >= 0, "pane.html thieu khoi :root (khai bao token cua theme.py)");
  const open = html.indexOf("{", start);
  const close = html.indexOf("}", open);
  assert.ok(open >= 0 && close > open, "khoi :root khong dong (phai la mot khoi phang, khong long nhau)");
  const body = html.slice(open + 1, close).replace(/\/\*[\s\S]*?\*\//g, "");
  const vars = {};
  for (const match of body.matchAll(/--axiom-([a-z0-9-]+)\s*:\s*([^;]+)/g)) {
    vars[match[1]] = match[2].trim();
  }
  return { vars, span: [start, close] };
}
function loadCore() {
  const context = { console, setTimeout, clearTimeout, setInterval, clearInterval, Promise, JSON };
  context.window = context;
  context.globalThis = context;
  vm.createContext(context);
  vm.runInContext(fs.readFileSync(path.join(ADDIN, "pane-core.js"), "utf8"), context, { filename: "pane-core.js" });
  return context;
}
const plain = (value) => JSON.parse(JSON.stringify(value));
const sorted = (object) => Object.fromEntries(Object.entries(object).sort(([a], [b]) => (a < b ? -1 : 1)));

// ---------------------------------------------------------------- mau

test("pane.html: MOI mau cua theme.py co mat, dung gia tri", () => {
  const t = theme();
  const { vars } = rootVars(paneHtml());
  const missing = [];
  for (const [name, hex] of Object.entries(t.colors)) {
    const key = slug(name).slice(2);
    if (!vars[key]) { missing.push(slug(name)); continue; }
    assert.equal(vars[key].toUpperCase(), hex,
      slug(name) + " sai mau: theme.py " + hex + ", pane.html " + vars[key]);
  }
  assert.deepEqual(missing, [], "pane.html thieu token mau cua theme.py");
  assert.ok(Object.keys(t.colors).length >= 20, "theme.py phai co it nhat 20 mau (doc qua theme_dump.py)");
});

test("pane.html: khong co mau nao ngoai theme.py, va khong token nao chet", () => {
  const html = paneHtml();
  const { vars, span } = rootVars(html);
  const allowed = new Set(Object.values(theme().colors).map((hex) => hex.toUpperCase()));
  const strays = [];
  for (const match of html.matchAll(/#[0-9A-Fa-f]{6}\b/g)) {
    if (match.index >= span[0] && match.index <= span[1]) { continue; }
    if (!allowed.has(match[0].toUpperCase())) { strays.push(match[0]); }
  }
  assert.deepEqual(strays, [],
    "pane.html con mau ngoai theme.py (moi mau phai qua var(--axiom-...)): " + strays.join(", "));
  // Mau viet ngan (#fff, #abc) cung la mau ngoai bang: chan luon.
  const shortHex = [...html.matchAll(/:\s*[^;{}]*#[0-9A-Fa-f]{3,4}\b/g)].map((m) => m[0].trim());
  assert.deepEqual(shortHex, [], "pane.html dung mau viet ngan: " + shortHex.join(", "));
  const unused = Object.keys(vars).filter((key) => !html.includes("var(--axiom-" + key + ")"));
  assert.deepEqual(unused, [], "token khai bao ma khong dung - bo ra hoac dung that: " + unused.join(", "));
});

// ---------------------------------------------------------------- so do + co chu

test("pane.html: so do va co chu khop theme.py", () => {
  const t = theme();
  const { vars } = rootVars(paneHtml());
  for (const [name, value] of Object.entries(t.metrics)) {
    const key = slug(name).slice(2);
    assert.ok(vars[key], "pane.html thieu " + slug(name) + " (theme.py " + name + " = " + value + ")");
    if (value > 0 && value < 1) {
      assert.equal(vars[key], Math.round(value * 100) + "%",
        slug(name) + " la ti le " + value + " -> phai la " + Math.round(value * 100) + "%");
    } else {
      assert.equal(vars[key], value + "px", slug(name) + " phai la " + value + "px (theme.py " + name + ")");
    }
  }
  const sizes = ["SIZE_TITLE", "SIZE_EMPTY_TITLE", "SIZE_BODY", "SIZE_SMALL", "SIZE_TOOL", "SIZE_TOOL_ID",
    "SIZE_CAPTION"];
  for (const name of sizes) {
    const key = slug(name).slice(2);
    assert.ok(vars[key], "pane.html thieu " + slug(name));
    assert.equal(parseFloat(vars[key]), t.sizes[name], slug(name) + ": co chu");
    assert.match(vars[key], /pt$/, slug(name) + " phai tinh bang pt nhu theme.py (diem), khong phai px");
  }
});

// ---------------------------------------------------------------- chuoi cua pane LibreOffice

test("pane.html: dung chuoi tieng Viet cua pane LibreOffice", () => {
  const t = theme();
  const html = paneHtml();
  for (const [name, text] of Object.entries({
    "theme.PROMPT_PLACEHOLDER": t.strings.placeholder,
    "theme.PROMPT_HINT": t.strings.hint,
    "theme.PROMPT_HINT_SHORT": t.strings.hint_short,
    "chatview.py:382 'GỢI Ý'": "GỢI Ý",
    "panel.py:166 'Sẵn sàng'": "Sẵn sàng",
    "panel.py:99 'Đang chạy'": "Đang chạy",
    "chatview.py:313 'Cần bạn xác nhận: '": "Cần bạn xác nhận: ",
    "chatview.py:349 'Có lỗi xảy ra'": "Có lỗi xảy ra",
    "widgets.py 'Gửi'": "Gửi",
    "panel.py:170 'Dừng'": "Dừng",
    "panel.py:137 'Trò chuyện mới'": "Trò chuyện mới",
    "chatview.py 'Đồng ý'/'Từ chối'": "Đồng ý",
  })) {
    assert.ok(html.includes(text), "pane.html thieu chuoi cua " + name + ": " + text);
  }
  assert.ok(html.includes("Từ chối"), "pane.html thieu nut 'Từ chối'");
  // Tieng Viet phai la UTF-8 that trong file (khong viet &amp;#7897;): pane.html co <meta charset="utf-8">.
  assert.match(html, /charset="?utf-8"?/i, "pane.html phai khai charset utf-8");
  assert.doesNotMatch(html, /&#\d{3,};/, "pane.html viet ky tu tieng Viet bang ma so - dung UTF-8 truc tiep");
});

// ---------------------------------------------------------------- du lieu + nhan lenh trong pane-core.js

test("pane-core.js: nhan lenh, goi y, empty state BANG theme.py", () => {
  const t = theme();
  const ctx = loadCore();
  assert.equal(typeof ctx.AxiomPaneCore, "object", "thieu global AxiomPaneCore");
  const core = ctx.AxiomPaneCore;
  for (const [key, expected] of [["ACTION_LABELS", t.labels.action], ["SUFFIX_LABELS", t.labels.suffix],
    ["TOOL_LABELS", t.labels.tool], ["NOUNS", t.nouns], ["APP_NAMES", t.apps], ["SUGGESTIONS", null]]) {
    assert.ok(core[key], "pane-core.js thieu AxiomPaneCore." + key + " (du lieu cua theme.py)");
    if (expected) {
      assert.deepEqual(sorted(plain(core[key])), sorted(expected),
        "AxiomPaneCore." + key + " khac theme.py");
    }
  }
  for (const kind of ["wps", "et", "wpp"]) {
    assert.deepEqual(plain(core.SUGGESTIONS[kind]), t.empty[kind].suggestions, "SUGGESTIONS." + kind);
    const state = core.emptyState(kind);
    assert.equal(state.title, t.empty[kind].title, "emptyState(" + kind + ").title");
    assert.equal(state.description, t.empty[kind].description, "emptyState(" + kind + ").description");
    assert.deepEqual(plain(state.suggestions), t.empty[kind].suggestions, "emptyState(" + kind + ").suggestions");
  }
  assert.equal(typeof core.labelFor, "function", "thieu AxiomPaneCore.labelFor");
  for (const [action, expected] of Object.entries(t.label_probes)) {
    assert.equal(core.labelFor(action), expected, "labelFor(" + JSON.stringify(action) + ")");
  }
  assert.equal(typeof core.plainReply, "function", "thieu AxiomPaneCore.plainReply (theme.plain_reply)");
  assert.equal(core.plainReply("**Đậm** và `mã`\n"), "Đậm và mã", "plainReply phai bo ** va ` nhu theme.plain_reply");
});

test("pane.html dung labelFor/emptyState cua pane-core (khong tu che nhan)", () => {
  const html = paneHtml();
  assert.match(html, /labelFor/, "pane.html phai hien nhan lenh bang AxiomPaneCore.labelFor");
  assert.match(html, /emptyState/, "pane.html phai dung AxiomPaneCore.emptyState cho man hinh trong");
});

// ---------------------------------------------------------------- cau truc + chat luong tuong tac

test("pane.html: du vung giao dien cua pane LibreOffice", () => {
  const html = paneHtml();
  for (const [id, why] of [["paneHeader", "header (cham accent + tieu de + lien ket)"],
    ["paneTitle", "tieu de = ten app"], ["paneSubtitle", "phu de"],
    ["newBtn", "lien ket 'Trò chuyện mới'"], ["chat", "vung hoi thoai cuon duoc"],
    ["composer", "o soan"], ["input", "o nhap"], ["sendBtn", "nut 'Gửi'"],
    ["stopBtn", "nut 'Dừng' (footer)"], ["statusText", "dong trang thai o footer"],
    ["emptyTitle", "tieu de man hinh trong"], ["confirmTitle", "the xac nhan"]]) {
    // Vung sinh dong (man hinh trong, the xac nhan) gan id bang JS: chap nhan ca id="x" lan .id = "x".
    assert.match(html, new RegExp("id\\s*=\\s*[\"']" + id + "[\"']"),
      "pane.html thieu #" + id + " (" + why + ")");
  }
  assert.match(html, /pane-core\.js/, "pane.html phai nap pane-core.js");
  assert.match(html, /<html[^>]*lang="vi"/i, "pane.html phai khai lang=\"vi\"");
  // Trang thai khong chi noi bang mau: co ca dau ✓/✕ cho dong thao tac (chatview.py _line).
  assert.ok(html.includes("✓") && html.includes("✕"), "dong thao tac phai co dau ✓ va ✕, khong chi doi mau");
});

test("pane.html: trang thai hover/focus/disabled theo chuan UX", () => {
  const html = paneHtml();
  const rules = html.slice(html.indexOf("<style"), html.indexOf("</style>"));
  assert.match(rules, /:focus[^{]*\{[^}]*var\(--axiom-focus\)/, "thieu vien focus nhin thay duoc (mau theme.FOCUS)");
  assert.match(rules, /:disabled[^{]*\{[^}]*var\(--axiom-disabled-bg\)/, "trang thai disabled thieu nen DISABLED_BG");
  assert.match(rules, /:disabled[^{]*\{[^}]*var\(--axiom-disabled-fg\)/, "trang thai disabled thieu chu DISABLED_FG");
  const durations = [...rules.matchAll(/transition[^;]*?(\d+)ms/g)].map((m) => Number(m[1]));
  assert.ok(durations.length > 0, "thieu transition cho hover (150-300ms)");
  for (const ms of durations) {
    assert.ok(ms >= 150 && ms <= 300, "transition " + ms + "ms ngoai khoang 150-300ms");
  }
  assert.match(rules, /cursor:\s*pointer/, "cac phan bam duoc phai co cursor: pointer");
  assert.match(rules, /:hover[^{]*\{/, "nut/lien ket/chip phai co trang thai hover");
  assert.match(rules, /:active[^{]*\{/, "nut phai co phan hoi khi bam (:active)");
});

// ---------------------------------------------------------------- pane biet app nao (goi y theo app)

test("main.js: paneUrl() mang ?app=<kind>; pane.html doc lai", () => {
  const html = paneHtml();
  assert.match(html, /location\s*\.\s*search/, "pane.html phai doc app tu query string (?app=)");
  assert.match(html, /"wps"/, "pane.html thieu mac dinh 'wps' khi khong co ?app=");
  const main = fs.readFileSync(path.join(ADDIN, "main.js"), "utf8");
  assert.match(main, /pane\.html\?app="/, "paneUrl() phai them ?app=<kind> (currentAppKind)");
  const created = [];
  const context = { console, setTimeout, clearTimeout, setInterval, clearInterval, Promise, JSON };
  context.window = context;
  context.globalThis = context;
  context.Application = {
    Name: "WPS Spreadsheets",
    CreateTaskPane(url) { const pane = { url, Visible: false }; created.push(pane); return pane; },
  };
  context.location = { href: "http://127.0.0.1:47840/wps/addin/index.html" };
  context.fetch = () => new Promise(() => {});
  vm.createContext(context);
  for (const file of ["probe.js", "commands.js", "main.js"]) {
    vm.runInContext(fs.readFileSync(path.join(ADDIN, file), "utf8"), context, { filename: file });
  }
  context.ribbon.OnAction({ Id: "axiomAskAI" });
  assert.equal(created.length, 1, "phai goi Application.CreateTaskPane");
  assert.equal(created[0].url, "http://127.0.0.1:47840/wps/addin/pane.html?app=et",
    "pane trong Spreadsheets phai duoc mo voi ?app=et (goi y dung loai tai lieu)");
  if (context.AxiomBridge) { context.AxiomBridge.stop(); }
});
