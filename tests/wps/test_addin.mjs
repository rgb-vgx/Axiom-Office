// Spec T2 (docs/wps-bridge.md muc 3): registry lenh + vong poll cua JS add-in WPS, khong can WPS.
//
//   node --test tests/wps/test_addin.mjs
//
// Hop dong:
// - src/AxiomOffice.WPS/commands.js dinh nghia global AxiomCommands:
//     catalog()                      -> [{name, kind, agent, summary, params:[{name, required, hint}]}]
//     execute(app, action, params)   -> {ok:true, result} | {ok:false, error}; KHONG BAO GIO throw;
//                                       lenh sua tai lieu chay trong app.UndoRecord.StartCustomRecord("AI: <action>")
//                                       ... EndCustomRecord() (End luon duoc goi neu Start da goi).
// - src/AxiomOffice.WPS/main.js dinh nghia window.ribbon.OnAddinLoad (tra true, goi AxiomBridge.start())
//   va global AxiomBridge { start(), stop() }: vong long-poll bang fetch POST JSON toi URL ket thuc bang
//   "/wps/v1/poll" (body co app:"wps", commands = AxiomCommands.catalog(), document), thuc thi tung job bang
//   AxiomCommands.execute(window.Application, ...) roi POST {id, ok, result|error} toi URL ket thuc bang
//   "/wps/v1/result".
import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
const ADDIN = path.join(ROOT, "src", "AxiomOffice.WPS");

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

function loadAddin(extra = {}) {
  const context = { console, setTimeout, clearTimeout, setInterval, clearInterval, Promise, JSON, ...extra };
  context.window = context;
  context.globalThis = context;
  vm.createContext(context);
  for (const file of ["probe.js", "commands.js", "main.js"]) {
    const full = path.join(ADDIN, file);
    if (fs.existsSync(full)) {
      vm.runInContext(fs.readFileSync(full, "utf8"), context, { filename: file });
    }
  }
  return context;
}

// Object model WPS gia: moi thuoc tinh/ham deu ton tai va tra lai mot mock (khong rang buoc cach goi API),
// tru UndoRecord (ghi nhan), cac override, Count/length = 0, Found = false va moi ham *.Execute() tra false.
function mockApp(log, overrides = {}) {
  function deep(name) {
    const target = function () {};
    return new Proxy(target, {
      get(_t, prop) {
        if (prop === Symbol.toPrimitive) return () => "";
        if (prop === "toString" || prop === "valueOf") return () => "";
        if (prop === "then") return undefined;
        if (prop === "length" || prop === "Count") return 0;
        if (prop === "Found") return false;   // vong `while (Find.Found)` phai dung
        if (typeof prop === "symbol") return undefined;
        const key = name + "." + prop;
        if (Object.prototype.hasOwnProperty.call(overrides, key)) {
          const value = overrides[key];
          return typeof value === "function" && value.getter ? value() : value;
        }
        return deep(key);
      },
      apply() {
        log.push("call:" + name);
        if (name.endsWith(".Execute")) return false;   // vong `while (Find.Execute(...))` phai dung
        return deep(name + "()");
      },
      set() { return true; },
    });
  }
  const undo = {
    StartCustomRecord(title) { log.push("start:" + title); },
    EndCustomRecord() { log.push("end"); },
  };
  return new Proxy({}, {
    get(_t, prop) {
      if (prop === "UndoRecord") return undo;
      if (typeof prop === "symbol") return undefined;
      const key = "app." + prop;
      if (Object.prototype.hasOwnProperty.call(overrides, key)) {
        const value = overrides[key];
        if (typeof value === "function" && value.getter) return value();
        return value;
      }
      return deep(key);
    },
    set() { return true; },
  });
}

function throwing(message) {
  const getter = () => { throw new Error(message); };
  getter.getter = true;
  return getter;
}

test("commands.js dinh nghia AxiomCommands voi catalog + execute", () => {
  const ctx = loadAddin();
  assert.equal(typeof ctx.AxiomCommands, "object", "thieu global AxiomCommands (src/AxiomOffice.WPS/commands.js)");
  assert.equal(typeof ctx.AxiomCommands.catalog, "function");
  assert.equal(typeof ctx.AxiomCommands.execute, "function");
});

test("registry add-in co du lenh dot 1 va metadata giong het registry LibreOffice", () => {
  const lo = JSON.parse(execFileSync("python3", [path.join(ROOT, "tests", "wps", "lo_catalog.py")], { encoding: "utf8" }));
  const byName = new Map(lo.map((c) => [c.name, c]));
  const ctx = loadAddin();
  const mine = new Map(JSON.parse(JSON.stringify(ctx.AxiomCommands.catalog())).map((c) => [c.name, c]));
  for (const name of REQUIRED) {
    assert.ok(mine.has(name), "add-in thieu lenh " + name);
    const theirs = byName.get(name);
    const ours = mine.get(name);
    assert.equal(ours.kind, theirs.kind, name + ": kind");
    assert.equal(ours.agent, theirs.agent, name + ": agent");
    assert.equal(ours.summary, theirs.summary, name + ": summary");
    assert.deepEqual(ours.params.map((p) => [p.name, p.required, p.hint === undefined ? null : p.hint]),
      theirs.params.map((p) => [p.name, p.required, p.hint]), name + ": params");
  }
  for (const [name, ours] of mine) {
    assert.ok(byName.has(name), "lenh " + name + " khong co trong registry LibreOffice (ten lenh phai dung chung)");
    assert.equal(ours.kind, byName.get(name).kind, name + ": kind");
  }
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
