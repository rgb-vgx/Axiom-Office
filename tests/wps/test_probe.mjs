// Unit test cho src/AxiomOffice.WPS/probe.js (khong can WPS).
//     node --test tests/wps/test_probe.mjs
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import vm from "node:vm";

const here = path.dirname(fileURLToPath(import.meta.url));
const PROBE = path.join(here, "..", "..", "src", "AxiomOffice.WPS", "probe.js");
const require = createRequire(import.meta.url);
const { collectReport } = require(PROBE);

function fakeUndo(opts = {}) {
  const calls = { start: [], end: 0 };
  return {
    calls,
    StartCustomRecord(label) {
      calls.start.push(label);
      if (opts.startThrows) throw new Error("start boom");
    },
    EndCustomRecord() {
      calls.end += 1;
      if (opts.endThrows) throw new Error("end boom");
    },
  };
}

function fakeApp(overrides = {}) {
  const app = {
    Version: "11.1.0.11723",
    Name: "WPS Writer",
    ActiveDocument: { Name: "a.docx", FullName: "/tmp/a.docx" },
    Documents: { Count: 1 },
    UndoRecord: fakeUndo(),
  };
  return Object.assign(app, overrides);
}

function fakeWin(extra = {}) {
  return Object.assign(
    {
      location: { href: "http://127.0.0.1:3889/index.html" },
      navigator: { userAgent: "Mozilla/5.0 CEF" },
    },
    extra,
  );
}

test("full fake app with working UndoRecord", () => {
  const app = fakeApp();
  const r = collectReport(app, fakeWin());
  assert.equal(r.loaded, true);
  assert.equal(r.wpsVersion, "11.1.0.11723");
  assert.equal(r.appName, "WPS Writer");
  assert.equal(r.undoRecord, true);
  assert.equal(r.undoRecordTried, true);
  assert.equal(r.undoRecordMethods, true);
  assert.deepEqual(app.UndoRecord.calls.start, ["Axiom probe"]);
  assert.equal(app.UndoRecord.calls.end, 1);
  assert.deepEqual(r.activeDocument, { name: "a.docx", fullName: "/tmp/a.docx" });
  assert.equal(r.location, "http://127.0.0.1:3889/index.html");
  assert.equal(r.userAgent, "Mozilla/5.0 CEF");
  assert.deepEqual(r.errors, []);
  // must be JSON-serialisable as-is
  assert.deepEqual(JSON.parse(JSON.stringify(r)), r);
});

test("app without UndoRecord -> false", () => {
  const app = fakeApp();
  delete app.UndoRecord;
  const r = collectReport(app, fakeWin());
  assert.equal(r.undoRecord, false);
  assert.equal(r.wpsVersion, "11.1.0.11723");
  assert.ok(r.errors.some((e) => e.startsWith("undoRecord:")));
});

test("StartCustomRecord throws -> false, error recorded, no End call", () => {
  const app = fakeApp({ UndoRecord: fakeUndo({ startThrows: true }) });
  let r;
  assert.doesNotThrow(() => { r = collectReport(app, fakeWin()); });
  assert.equal(r.undoRecord, false);
  assert.equal(r.undoRecordTried, true);
  assert.equal(app.UndoRecord.calls.end, 0);
  assert.ok(r.errors.some((e) => e === "undoRecord.StartCustomRecord: start boom"), r.errors.join("|"));
});

test("EndCustomRecord throws after Start -> false + error", () => {
  const app = fakeApp({ UndoRecord: fakeUndo({ endThrows: true }) });
  const r = collectReport(app, fakeWin());
  assert.equal(r.undoRecord, false);
  assert.equal(app.UndoRecord.calls.start.length, 1);
  assert.equal(app.UndoRecord.calls.end, 1);
  assert.ok(r.errors.some((e) => e === "undoRecord.EndCustomRecord: end boom"), r.errors.join("|"));
});

test("Version getter throws -> wpsVersion '' + error", () => {
  const app = fakeApp();
  Object.defineProperty(app, "Version", { get() { throw new Error("no version"); } });
  const r = collectReport(app, fakeWin());
  assert.equal(r.wpsVersion, "");
  assert.ok(r.errors.includes("version: no version"), r.errors.join("|"));
  assert.equal(r.undoRecord, true);
});

test("Version missing falls back to Build; method-style properties are called", () => {
  const app = fakeApp({ Version: undefined, Build: () => "11.1.0.9999", Name: () => "WPS" });
  const r = collectReport(app, fakeWin());
  assert.equal(r.wpsVersion, "11.1.0.9999");
  assert.equal(r.appName, "WPS");
});

test("app null / undefined", () => {
  for (const app of [null, undefined]) {
    let r;
    assert.doesNotThrow(() => { r = collectReport(app, fakeWin()); });
    assert.equal(r.loaded, true);
    assert.equal(r.wpsVersion, "");
    assert.equal(r.undoRecord, false);
    assert.equal(r.activeDocument, null);
    assert.ok(r.errors.some((e) => e.startsWith("app:")));
  }
});

test("no active document -> undoRecordTried false, methods only checked", () => {
  const app = fakeApp({ ActiveDocument: null, Documents: { Count: 0 } });
  const r = collectReport(app, fakeWin());
  assert.equal(r.undoRecordTried, false);
  assert.equal(r.undoRecordMethods, true);
  assert.equal(r.undoRecord, false);
  assert.equal(r.activeDocument, null);
  assert.equal(app.UndoRecord.calls.start.length, 0);
  assert.equal(app.UndoRecord.calls.end, 0);
});

test("no ActiveDocument but Documents.Count > 0 -> still tried", () => {
  const app = fakeApp({ ActiveDocument: undefined, Documents: { Count: 2 } });
  const r = collectReport(app, fakeWin());
  assert.equal(r.undoRecordTried, true);
  assert.equal(r.undoRecord, true);
});

test("ActiveDocument getter throws -> error, does not throw", () => {
  const app = fakeApp({ Documents: { Count: 0 } });
  Object.defineProperty(app, "ActiveDocument", { get() { throw new Error("no doc"); } });
  const r = collectReport(app, fakeWin());
  assert.equal(r.activeDocument, null);
  assert.equal(r.undoRecordTried, false);
  assert.ok(r.errors.includes("activeDocument: no doc"));
});

test("win transport and globals detection", () => {
  const win = fakeWin({
    fetch() {},
    XMLHttpRequest: function XMLHttpRequest() {},
    WebSocket: function WebSocket() {},
    Application: {},
    wps: {},
  });
  const r = collectReport(fakeApp(), win);
  assert.deepEqual(r.transport, { fetch: true, xhr: true, webSocket: true });
  assert.deepEqual(r.globals, { Application: true, wps: true, WpsInvoke: false, cef: false, external: false });

  const bare = collectReport(fakeApp(), {});
  assert.deepEqual(bare.transport, { fetch: false, xhr: false, webSocket: false });
  assert.equal(bare.location, "");
  assert.equal(bare.userAgent, "");

  const noWin = collectReport(fakeApp(), undefined);
  assert.equal(noWin.globals.Application, false);
  assert.ok(noWin.errors.some((e) => e.startsWith("win:")));
});

test("throwing window properties are caught", () => {
  const win = {};
  Object.defineProperty(win, "location", { get() { throw new Error("denied"); } });
  Object.defineProperty(win, "cef", { get() { throw new Error("cef denied"); } });
  const r = collectReport(fakeApp(), win);
  assert.ok(r.errors.includes("location: denied"));
  assert.ok(r.errors.includes("globals.cef: cef denied"));
  assert.equal(r.globals.cef, false);
});

test("loads as a browser script and defines global AxiomProbe", () => {
  const source = readFileSync(PROBE, "utf8");
  const win = {
    navigator: { userAgent: "UA" },
    location: { href: "file:///addin/index.html" },
    XMLHttpRequest: function () {},
  };
  win.window = win;
  const context = vm.createContext(win);
  vm.runInContext(source, context, { filename: "probe.js" });
  assert.equal(typeof context.AxiomProbe, "object");
  assert.equal(typeof context.AxiomProbe.collectReport, "function");
  const r = vm.runInContext("AxiomProbe.collectReport(null, window)", context);
  assert.equal(r.loaded, true);
  assert.equal(r.transport.xhr, true);
  assert.equal(r.userAgent, "UA");
});

test("vm.runInNewContext without window falls back to globalThis", () => {
  const source = readFileSync(PROBE, "utf8");
  const sandbox = {};
  vm.runInNewContext(source, sandbox);
  assert.equal(typeof sandbox.AxiomProbe.collectReport, "function");
});

test("source stays ES5-ish (no optional chaining, arrows, let/const, async)", () => {
  const code = readFileSync(PROBE, "utf8").replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/.*$/gm, "");
  for (const [re, what] of [[/\?\./, "optional chaining"], [/=>/, "arrow function"],
    [/\b(let|const|async|await|class)\b/, "ES6 keyword"], [/`/, "template string"]]) {
    assert.ok(!re.test(code), "probe.js uses " + what);
  }
});
