// Spec T2 (docs/wps-bridge.md muc 3): registry lenh + vong poll cua JS add-in WPS, khong can WPS.
// Spec T6 (docs/wps-bridge.md muc 5): add-in nhan biet app dang chay (Writer/Spreadsheets/Presentation).
// Spec registry ⊆ handler: moi lenh trong registry LibreOffice (nguon sinh tool cho agent) phai co
// handler trong add-in, hoac nam trong CHUA_CO_HANDLER kem ly do - de lenh registry khong am tham
// thanh "unknown action" khi may dang chay WPS.
//
//   node --test tests/wps/test_addin.mjs
//
// Hop dong:
// - src/AxiomOffice.WPS/commands.js dinh nghia global AxiomCommands:
//     catalog()                      -> [{name, kind, agent, summary, params:[{name, required, hint}]}]
//     catalogFor(kind)               -> lenh dung cho mot app: kind rong (null/"" theo registry LibreOffice)
//                                       hoac kind === tham so
//     appKind(app)                   -> "wps" | "et" | "wpp" theo app.Name dang chay
//     execute(app, action, params)   -> {ok:true, result} | {ok:false, error}; KHONG BAO GIO throw;
//                                       lenh sua tai lieu chay trong app.UndoRecord.StartCustomRecord("AI: <action>")
//                                       ... EndCustomRecord() (End luon duoc goi neu Start da goi);
//                                       lenh cua app khac bi tu choi (khong chay nham trong app sai).
// - src/AxiomOffice.WPS/main.js dinh nghia window.ribbon.OnAddinLoad (tra true, goi AxiomBridge.start())
//   va global AxiomBridge { start(), stop() }: vong long-poll bang fetch POST JSON toi URL ket thuc bang
//   "/wps/v1/poll" (body co app = app dang chay, commands = catalog da loc theo app do, document),
//   thuc thi tung job bang AxiomCommands.execute(window.Application, ...) roi POST {id, ok, result|error}
//   toi URL ket thuc bang "/wps/v1/result".
import { test } from "node:test";
import assert from "node:assert/strict";

import { addinCatalog, assertMetadata, loadAddin, loCatalog, mockApp, run, throwing } from "./addin_harness.mjs";


// Lenh dot 1 (docs/wps-bridge.md muc 5, T2): add-in phai co, metadata giong het registry LibreOffice.
const REQUIRED = [
  "app.info",
  "writer.getText", "writer.selection", "writer.typeText", "writer.appendText", "writer.insertStyledText",
  "writer.heading", "writer.formatSelection", "writer.setParagraphAlignment", "writer.insertTable",
  "writer.insertPageBreak", "writer.replaceAll", "writer.undo", "writer.save", "writer.saveAs", "writer.exportPdf",
];
const MUTATING = ["writer.typeText", "writer.appendText", "writer.insertStyledText", "writer.heading",
  "writer.formatSelection", "writer.setParagraphAlignment", "writer.insertTable", "writer.insertPageBreak",
  "writer.replaceAll"];
const READ_ONLY = ["app.info", "writer.getText", "writer.selection"];

function catalogMap(ctx) {
  return new Map(addinCatalog(ctx).map((c) => [c.name, c]));
}

// `kind` rong = lenh dung chung moi app. Registry LibreOffice dung `null` cho nam lenh nay
// (app.info, ai.ask, ui.askpane, ui.setup, app.screenshot), nen phai chap nhan ca null lan "".
function sharedKind(kind) {
  return kind === "" || kind === null || kind === undefined;
}

test("commands.js dinh nghia AxiomCommands voi catalog + execute", () => {
  const ctx = loadAddin();
  assert.equal(typeof ctx.AxiomCommands, "object", "thieu global AxiomCommands (src/AxiomOffice.WPS/commands.js)");
  assert.equal(typeof ctx.AxiomCommands.catalog, "function");
  assert.equal(typeof ctx.AxiomCommands.execute, "function");
});

test("registry add-in co du lenh dot 1 va metadata giong het registry LibreOffice", () => {
  const ctx = loadAddin();
  assert.deepEqual(addinCatalog(ctx), loCatalog());
});

test("lenh sua tai lieu: dung 1 buoc UndoRecord 'AI: <lenh>' (Start roi End)", () => {
  const ctx = loadAddin();
  const params = {
    "writer.typeText": { text: "abc" }, "writer.appendText": { text: "abc" },
    "writer.insertStyledText": { text: "abc", bold: true }, "writer.heading": { text: "T", level: 1 },
    "writer.formatSelection": { bold: true }, "writer.setParagraphAlignment": { alignment: "center" },
    "writer.insertTable": { rows: 2, cols: 2 }, "writer.insertPageBreak": {},
    "writer.replaceAll": { find: "a", replace: "b" },
  };
  for (const action of MUTATING) {
    const log = [];
    const reply = ctx.AxiomCommands.execute(mockApp(log), action, params[action]);
    assert.equal(typeof reply, "object", action + ": phai tra object");
    const undo = log.filter((entry) => entry.startsWith("start:") || entry === "end");
    assert.deepEqual(undo, ["start:AI: " + action, "end"], action + ": UndoRecord " + JSON.stringify(log.slice(0, 20)));
  }
});

test("lenh chi doc khong mo UndoRecord", () => {
  const ctx = loadAddin();
  for (const action of READ_ONLY) {
    const log = [];
    ctx.AxiomCommands.execute(mockApp(log), action, {});
    assert.ok(!log.some((entry) => entry.startsWith("start:")), action + " khong duoc mo UndoRecord");
  }
});

test("execute khong bao gio throw: loi object model -> ok:false + error, End van duoc goi", () => {
  const ctx = loadAddin();
  for (const action of ["writer.appendText", "writer.getText", "writer.insertTable"]) {
    const log = [];
    const app = mockApp(log, { "app.ActiveDocument": throwing("doc boom"), "app.Selection": throwing("sel boom") });
    let reply;
    assert.doesNotThrow(() => { reply = ctx.AxiomCommands.execute(app, action, { text: "x", rows: 1, cols: 1 }); });
    assert.equal(reply.ok, false, action + ": " + JSON.stringify(reply));
    assert.match(String(reply.error), /boom/, action + ": error phai giu thong diep goc");
    if (log.some((entry) => entry.startsWith("start:"))) {
      assert.equal(log.filter((entry) => entry === "end").length, 1, action + ": Start roi thi phai End");
    }
  }
});

test("lenh la / thieu app -> ok:false, khong throw", () => {
  const ctx = loadAddin();
  const unknown = ctx.AxiomCommands.execute(mockApp([]), "writer.khongCo", {});
  assert.equal(unknown.ok, false);
  assert.match(String(unknown.error), /writer\.khongCo/);
  const noApp = ctx.AxiomCommands.execute(null, "writer.getText", {});
  assert.equal(noApp.ok, false);
});

test("main.js: ribbon.OnAddinLoad tra true va bat vong poll; job -> result dung id", async () => {
  const polls = [];
  const results = [];
  let served = 0;
  let stopped = false;
  function respond(payload) {
    return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(payload),
      text: () => Promise.resolve(JSON.stringify(payload)) });
  }
  const fetch = (url, options = {}) => {
    const body = options.body ? JSON.parse(options.body) : {};
    if (String(url).endsWith("/wps/v1/poll")) {
      polls.push({ url: String(url), body, method: options.method, headers: options.headers || {} });
      served += 1;
      if (served === 1) {
        return respond({ ok: true, result: { jobs: [
          { id: "j1", action: "writer.getText", params: {} },
          { id: "j2", action: "writer.khongCo", params: {} },
        ] } });
      }
      return new Promise((resolve) => setTimeout(() => resolve(respond({ ok: true, result: { jobs: [] } })), 50));
    }
    if (String(url).endsWith("/wps/v1/result")) {
      results.push(body);
      return respond({ ok: true });
    }
    return Promise.reject(new Error("unexpected url " + url));
  };
  const ctx = loadAddin({ fetch, Application: mockApp([]), location: { href: "http://127.0.0.1:47840/wps/addin/index.html" } });
  assert.equal(typeof ctx.ribbon, "object", "main.js phai dinh nghia window.ribbon");
  assert.equal(typeof ctx.AxiomBridge, "object", "main.js phai dinh nghia AxiomBridge");
  try {
    assert.equal(ctx.ribbon.OnAddinLoad({}), true);
    const deadline = Date.now() + 4000;
    while (results.length < 2 && Date.now() < deadline) {
      await new Promise((resolve) => setTimeout(resolve, 20));
    }
  } finally {
    ctx.AxiomBridge.stop();
    stopped = true;
  }
  assert.ok(stopped);
  assert.ok(polls.length >= 1, "phai poll it nhat mot lan");
  const first = polls[0];
  assert.equal(String(first.method).toUpperCase(), "POST");
  assert.equal(first.body.app, "wps");
  assert.ok(Array.isArray(first.body.commands) && first.body.commands.some((c) => c.name === "writer.getText"),
    "poll phai gui catalog lenh");
  assert.ok("document" in first.body, "poll phai gui document (co the null)");
  const forbidden = first.body.commands.filter((c) => CHUA_CO_HANDLER[c.name]).map((c) => c.name);
  assert.deepEqual(forbidden, [], "poll khong duoc khai bao lenh chua co handler: " + JSON.stringify(forbidden));
  const contentType = Object.entries(first.headers).find(([key]) => key.toLowerCase() === "content-type");
  assert.ok(contentType && /application\/json/.test(contentType[1]), "poll gui Content-Type application/json");
  const byId = new Map(results.map((r) => [r.id, r]));
  assert.ok(byId.has("j1") && typeof byId.get("j1").ok === "boolean", "result cho j1: " + JSON.stringify(results));
  assert.equal(byId.get("j2").ok, false, "lenh la -> result ok:false");
  // Sau stop() khong poll them qua 1 vong dang treo.
  const after = polls.length;
  await new Promise((resolve) => setTimeout(resolve, 300));
  assert.ok(polls.length <= after + 1, "stop() phai dung vong poll");
});

// ---- T6: add-in nhan biet app dang chay (docs/wps-bridge.md muc 5) ----
//
// WPS nap cung mot add-in trong ca ba app (publish.xml: type="wps,et,wpp"). Ten app that do duoc:
// "WPS Writer" (Writer), "WPS Spreadsheets" (ET), "WPS Presentation" (WPP).

function etApp(overrides = {}) {
  return Object.assign({
    Name: "WPS Spreadsheets", Version: "12.0", Build: "probe",
    ActiveWorkbook: { Name: "ban-hang.xlsx", FullName: "/tmp/ban-hang.xlsx" },
    ActiveSheet: { Name: "Sheet1" },
  }, overrides);
}

function wppApp(overrides = {}) {
  return Object.assign({
    Name: "WPS Presentation", Version: "12.0", Build: "probe",
    ActivePresentation: { Name: "bao-cao.pptx", FullName: "/tmp/bao-cao.pptx" },
  }, overrides);
}

function writerApp(overrides = {}) {
  return Object.assign({
    Name: "WPS Writer", Version: "12.0", Build: "probe",
    ActiveDocument: { Name: "hop-dong.docx", FullName: "/tmp/hop-dong.docx" },
  }, overrides);
}

test("appKind: doc dung app dang chay tu Application.Name", () => {
  const ctx = loadAddin();
  assert.equal(typeof ctx.AxiomCommands.appKind, "function", "thieu AxiomCommands.appKind");
  const kindOf = ctx.AxiomCommands.appKind;
  assert.equal(kindOf(etApp()), "et");
  assert.equal(kindOf(wppApp()), "wpp");
  assert.equal(kindOf(writerApp()), "wps");
  // Ten la (WPS Office, ban cu) hoac khong doc duoc -> coi la Writer (app kind "wps" la mac dinh).
  assert.equal(kindOf({ Name: "WPS Office" }), "wps");
  assert.equal(kindOf({}), "wps");
  assert.equal(kindOf(null), "wps");
  assert.equal(kindOf(undefined), "wps");
});

test("catalogFor(kind): chi tra lenh dung app do; catalog() van la registry day du", () => {
  const ctx = loadAddin();
  assert.equal(typeof ctx.AxiomCommands.catalogFor, "function", "thieu AxiomCommands.catalogFor");
  const full = addinCatalog(ctx);
  assert.ok(full.some((c) => c.name === "writer.getText") && full.some((c) => c.name === "et.listSheets"),
    "catalog() phai giu day du moi lenh (metadata dung chung)");
  assert.deepEqual(addinCatalog(ctx).length, full.length, "catalog() khong duoc loc theo app");
  for (const kind of ["wps", "et", "wpp"]) {
    const names = ctx.AxiomCommands.catalogFor(kind).map((c) => c.name);
    assert.ok(names.length > 0, kind + ": catalogFor phai co lenh");
    for (const name of names) {
      const entry = full.find((c) => c.name === name);
      assert.ok(sharedKind(entry.kind) || entry.kind === kind,
        kind + ": catalogFor tra lenh " + name + " (kind=" + entry.kind + ")");
    }
    for (const entry of full) {
      if (sharedKind(entry.kind) || entry.kind === kind) {
        assert.ok(names.includes(entry.name), kind + ": catalogFor thieu " + entry.name);
      }
    }
    assert.ok(names.includes("app.info"), kind + ": app.info co kind rong nen phai co o moi app");
  }
  assert.ok(!ctx.AxiomCommands.catalogFor("et").some((c) => c.name === "writer.getText"),
    "app Spreadsheets khong duoc khai bao lenh writer.*");
});

// Lenh co trong registry LibreOffice (nguon sinh tool cho agent o Core: scripts/generate_mcp_commands.py)
// ma add-in CHUA co handler. `execute` tra "unknown action: <lenh>" cho nhung lenh nay, nen neu Core gui
// xuong WPS thi agent nhan loi that bai kho hieu thay vi lam duoc viec.
//
// Danh sach nay chi duoc PHEP ngan di: moi lenh o day ma sau nay co handler thi test ben duoi bat phai bo ra.
const CHUA_CO_HANDLER = {
  // Bon lenh kind rong (agent=false): khong phai lenh cua app van phong - Core/pane/MCP xu ly, add-in
  // khong bao gio nhan. Chung nam trong registry vi registry dung chung cho moi nen tang.
  "ai.ask": "Core xu ly (khong phai lenh cua app van phong)",
  "ui.askpane": "Core/pane xu ly",
  "ui.setup": "Core/pane xu ly",
  "app.screenshot": "Core chup cua so (phan visual), add-in khong nhan lenh nay",
  // Bay lenh writer.* (docs/wps-bridge.md muc 3): backlog - chua viet handler cho WPS.
  "writer.newDocument": "chua co handler trong add-in",
  "writer.open": "chua co handler trong add-in",
  "writer.closeAll": "chua co handler trong add-in",
  "writer.formatTable": "chua co handler trong add-in (agent=true: tool nay se loi khi may dang la WPS)",
  "writer.insertImage": "chua co handler trong add-in (agent=true)",
  "writer.insertHyperlink": "chua co handler trong add-in (agent=true)",
  "writer.checkTables": "chua co handler trong add-in (agent=true)",
};

test("registry LibreOffice: moi lenh deu co handler, hoac nam trong danh sach chua lam (co ly do)", () => {
  const ctx = loadAddin();
  const apps = { wps: writerApp(), et: etApp(), wpp: wppApp() };
  assert.ok(loCatalog().length >= addinCatalog(ctx).length,
    "registry LibreOffice la nguon: add-in khong duoc khai bao nhieu lenh hon");
  for (const entry of loCatalog()) {
    // Lenh kind rong chay duoc o moi app; lenh cua app nao thi phai goi bang app do, khong thi
    // execute() tra loi "cannot run in ..." chu khong phai "unknown action:".
    const kind = sharedKind(entry.kind) ? "wps" : entry.kind;
    assert.ok(apps[kind], entry.name + ": kind la " + entry.kind);
    const reply = ctx.AxiomCommands.execute(apps[kind], entry.name, {});
    const coHandler = reply.ok === true || String(reply.error) !== "unknown action: " + entry.name;
    if (coHandler) {
      assert.ok(!CHUA_CO_HANDLER[entry.name],
        entry.name + " da co handler (" + JSON.stringify(reply).slice(0, 120) + ") - bo khoi CHUA_CO_HANDLER");
    } else {
      assert.ok(CHUA_CO_HANDLER[entry.name],
        entry.name + ": registry LibreOffice co lenh nay nhung add-in khong co handler - "
        + "viet handler, hoac ghi vao CHUA_CO_HANDLER kem ly do");
    }
  }
});

test("hasHandler: true voi moi lenh co handler, false voi lenh trong CHUA_CO_HANDLER", () => {
  const ctx = loadAddin();
  assert.equal(typeof ctx.AxiomCommands.hasHandler, "function", "thieu AxiomCommands.hasHandler");
  for (const entry of loCatalog()) {
    assert.equal(!!ctx.AxiomCommands.hasHandler(entry.name), !CHUA_CO_HANDLER[entry.name], entry.name);
  }
});

test("execute: lenh cua app khac bi tu choi, khong chay nham", () => {
  const ctx = loadAddin();
  const reply = ctx.AxiomCommands.execute(etApp(), "writer.getText", {});
  assert.equal(reply.ok, false, "writer.getText khong duoc chay trong Spreadsheets");
  assert.match(String(reply.error), /writer\.getText/);
  assert.match(String(reply.error), /Spreadsheets/);
  const reply2 = ctx.AxiomCommands.execute(writerApp(), "et.listSheets", {});
  assert.equal(reply2.ok, false, "et.listSheets khong duoc chay trong Writer");
  assert.match(String(reply2.error), /Writer/);
  // Lenh kind rong (app.info) chay duoc o moi app.
  for (const app of [etApp(), wppApp(), writerApp()]) {
    assert.equal(ctx.AxiomCommands.execute(app, "app.info", {}).ok, true, "app.info phai chay duoc o moi app");
  }
});

test("app.info: tra ve app dang chay va tai lieu dang mo", () => {
  const ctx = loadAddin();
  const et = run(ctx, etApp(), "app.info");
  assert.equal(et.app, "et", "app.info trong Spreadsheets: " + JSON.stringify(et));
  assert.equal(et.document.name, "ban-hang.xlsx");
  assert.equal(et.document.fullName, "/tmp/ban-hang.xlsx");
  const wpp = run(ctx, wppApp(), "app.info");
  assert.equal(wpp.app, "wpp");
  assert.equal(wpp.document.name, "bao-cao.pptx");
  const wps = run(ctx, writerApp(), "app.info");
  assert.equal(wps.app, "wps");
  assert.equal(wps.document.name, "hop-dong.docx");
  // Chua mo tai lieu: van tra loi, document = null.
  const empty = run(ctx, { Name: "WPS Spreadsheets", Version: "12.0" }, "app.info");
  assert.equal(empty.app, "et");
  assert.equal(empty.document, null);
});

test("thieu Application: thong diep loi noi ve WPS noi chung, khong chi Writer", () => {
  const ctx = loadAddin();
  const noApp = ctx.AxiomCommands.execute(null, "et.listSheets", {});
  assert.equal(noApp.ok, false);
  assert.doesNotMatch(String(noApp.error), /Writer/, "thong diep khong duoc chi noi ve Writer: " + noApp.error);
  assert.match(String(noApp.error), /WPS/);
});

test("main.js: poll bao dung app dang chay + catalog da loc theo app do", async () => {
  for (const [app, kind, expected, absent] of [
    [etApp(), "et", "et.listSheets", "writer.getText"],
    [wppApp(), "wpp", "wpp.listSlides", "writer.getText"],
    [writerApp(), "wps", "writer.getText", "et.listSheets"],
  ]) {
    const polls = [];
    const fetch = (url, options = {}) => {
      if (String(url).endsWith("/wps/v1/poll")) {
        polls.push(JSON.parse(options.body));
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ ok: true, result: { jobs: [] } }) });
      }
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ ok: true }) });
    };
    const ctx = loadAddin({ fetch, Application: app, location: { href: "http://127.0.0.1:47840/wps/addin/index.html" } });
    try {
      ctx.ribbon.OnAddinLoad({});
      const deadline = Date.now() + 2000;
      while (polls.length < 1 && Date.now() < deadline) {
        await new Promise((resolve) => setTimeout(resolve, 10));
      }
    } finally {
      ctx.AxiomBridge.stop();
    }
    assert.ok(polls.length >= 1, kind + ": phai poll");
    const body = polls[0];
    assert.equal(body.app, kind, kind + ": poll phai bao app dang chay");
    const names = body.commands.map((c) => c.name);
    assert.ok(names.includes(expected), kind + ": poll thieu " + expected);
    assert.ok(!names.includes(absent), kind + ": poll khong duoc khai bao " + absent + " (cua app khac)");
    for (const name of names) {
      assert.ok(!CHUA_CO_HANDLER[name], kind + ": poll khong duoc khai bao " + name + " (chua co handler)");
    }
    assert.equal(body.document.name, app.ActiveDocument ? app.ActiveDocument.Name
      : (app.ActiveWorkbook ? app.ActiveWorkbook.Name : app.ActivePresentation.Name),
    kind + ": poll phai bao tai lieu dang mo");
  }
});

