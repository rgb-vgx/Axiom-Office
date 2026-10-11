// Ham dung chung cho spec add-in WPS (test_addin*.mjs): nap add-in vao vm, so metadata
// voi registry LibreOffice (lo_catalog.py), va vai tro giup kiem loi tham so.
//
// File nay KHONG phai test: CI chay tung file test_*.mjs tuong minh.
import { execFileSync } from "node:child_process";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

export const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
export const ADDIN = path.join(ROOT, "src", "AxiomOffice.WPS");

// Nap add-in (probe.js neu co, commands.js, main.js) vao mot context vm rieng.
export function loadAddin(extra = {}) {
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

// Object model WPS gia toi thieu: moi thuoc tinh/ham deu ton tai va tra mot mock (khong rang buoc
// cach goi API), tru UndoRecord (ghi nhan), cac override, Count/length = 0, Found = false va moi
// ham *.Execute() tra false.
export function mockApp(log, overrides = {}) {
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

export function throwing(message) {
  const getter = () => { throw new Error(message); };
  getter.getter = true;
  return getter;
}

let cachedCatalog = null;

// Registry lenh cua extension LibreOffice: nguon duy nhat cho metadata (ten, kind, agent, summary, params).
export function loCatalog() {
  if (cachedCatalog === null) {
    cachedCatalog = JSON.parse(execFileSync("python3", [path.join(ROOT, "tests", "wps", "lo_catalog.py")],
      { encoding: "utf8" }));
  }
  return cachedCatalog;
}

export function loByName() {
  return new Map(loCatalog().map((c) => [c.name, c]));
}

// catalog() cua add-in, da cat khoi vm (JSON round-trip).
export function addinCatalog(ctx) {
  const raw = ctx.AxiomCommands.catalog();
  return JSON.parse(JSON.stringify(raw));
}

// Metadata cua tung lenh trong `names` phai giong het registry LibreOffice, va moi lenh add-in khai
// bao phai co ten trong registry do (ten lenh dung chung giua cac nen tang).
export function assertMetadata(names, mine) {
  const byName = loByName();
  const have = new Map(mine.map((c) => [c.name, c]));
  for (const name of names) {
    assert.ok(have.has(name), "add-in thieu lenh " + name);
    const theirs = byName.get(name);
    const ours = have.get(name);
    assert.equal(ours.kind, theirs.kind, name + ": kind");
    assert.equal(ours.agent, theirs.agent, name + ": agent");
    assert.equal(ours.summary, theirs.summary, name + ": summary");
    assert.deepEqual(ours.params.map((p) => [p.name, p.required, p.hint === undefined ? null : p.hint]),
      theirs.params.map((p) => [p.name, p.required, p.hint]), name + ": params");
  }
  for (const [name, ours] of have) {
    assert.ok(byName.has(name), "lenh " + name + " khong co trong registry LibreOffice (ten lenh phai dung chung)");
    assert.equal(ours.kind, byName.get(name).kind, name + ": kind");
  }
}

// Chay mot lenh va doi ok:false voi thong diep khop `pattern` (giu nguyen van loi cua handler).
export function assertFails(ctx, app, action, params, pattern) {
  let reply;
  assert.doesNotThrow(() => { reply = ctx.AxiomCommands.execute(app, action, params); },
    action + ": execute khong duoc throw");
  assert.equal(reply.ok, false, action + " phai that bai: " + JSON.stringify(reply));
  assert.match(String(reply.error), pattern, action + ": thong diep loi");
  return reply;
}

// Chay mot lenh va doi ok:true; tra ve result.
// Ket qua di qua JSON: dung nhu duong HTTP toi Core (add-in gui result bang JSON) va cat khoi vm
// realm - object dung trong vm co prototype khac nen deepStrictEqual cua assert/strict bao sai du
// cung du lieu. Nho vay handler khong phai biet gi ve realm cua test.
export function run(ctx, app, action, params = {}) {
  const reply = ctx.AxiomCommands.execute(app, action, params);
  assert.equal(reply.ok, true, action + " phai thanh cong: " + JSON.stringify(reply));
  return reply.result === undefined ? undefined : JSON.parse(JSON.stringify(reply.result));
}

// Vi mot so lenh chi ap dung cho mot component: kiem add-in tu choi lenh cua component khac.
export function assertKindRefused(ctx, app, action, params = {}) {
  let reply;
  assert.doesNotThrow(() => { reply = ctx.AxiomCommands.execute(app, action, params); });
  assert.equal(reply.ok, false, action + " phai bi tu choi khi khong dung component: " + JSON.stringify(reply));
  return reply;
}
