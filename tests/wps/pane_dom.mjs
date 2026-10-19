// DOM gia toi thieu de chay script trong pane.html ma KHONG can trinh duyet (xem tests/wps/REFERENCE.md:
// pane la trang cua Core, chi song trong CEF cua WPS - khong mo bang trinh duyet de kiem).
//
// File nay KHONG phai test: CI chay tung file test_*.mjs tuong minh.
//
// Pham vi: du de chay pane.html - createElement/createTextNode, thuoc tinh, cay cha-con, su kien click
// co noi bot (bubbling), getElementById, body. KHONG phai mot trinh duyet: khong layout, khong CSS,
// khong bang phan giai that (offsetWidth = 0).
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

export const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
export const ADDIN = path.join(ROOT, "src", "AxiomOffice.WPS");

const VOID_TAGS = new Set(["meta", "link", "br", "img", "input", "hr", "source", "area", "base", "col",
  "embed", "param", "track", "wbr"]);

class TextNode {
  constructor(text) {
    this.nodeType = 3;
    this.text = String(text);
    this.parentNode = null;
  }
}

class Element {
  constructor(tag) {
    this.nodeType = 1;
    this.tagName = String(tag).toUpperCase();
    this.childNodes = [];
    this.attributes = {};
    this.style = {};
    this.parentNode = null;
    this.listeners = {};
    this.className = "";
    this.id = "";
    this.value = "";
    this.checked = false;
    this.type = "";
    this.offsetWidth = 0;
    this.scrollTop = 0;
    this.scrollHeight = 0;
  }
  get firstChild() { return this.childNodes.length > 0 ? this.childNodes[0] : null; }
  get children() { return this.childNodes.filter((node) => node.nodeType === 1); }
  appendChild(child) {
    if (child.nodeType === 11) { // DocumentFragment: chuyen con vao thang phan tu nhan.
      for (const inner of child.childNodes.slice()) { this.appendChild(inner); }
      child.childNodes.length = 0;
      return child;
    }
    if (child.parentNode) { child.parentNode.removeChild(child); }
    child.parentNode = this;
    this.childNodes.push(child);
    return child;
  }
  insertBefore(child, ref) {
    const at = ref ? this.childNodes.indexOf(ref) : -1;
    if (child.parentNode) { child.parentNode.removeChild(child); }
    child.parentNode = this;
    if (at < 0) { this.childNodes.push(child); } else { this.childNodes.splice(at, 0, child); }
    return child;
  }
  removeChild(child) {
    const at = this.childNodes.indexOf(child);
    if (at >= 0) { this.childNodes.splice(at, 1); child.parentNode = null; }
    return child;
  }
  setAttribute(name, value) {
    this.attributes[name] = String(value);
    if (name === "id") { this.id = String(value); }
    if (name === "class") { this.className = String(value); }
  }
  getAttribute(name) {
    return Object.prototype.hasOwnProperty.call(this.attributes, name) ? this.attributes[name] : null;
  }
  hasAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attributes, name); }
  removeAttribute(name) { delete this.attributes[name]; }
  addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
  removeEventListener(type, fn) {
    const list = this.listeners[type] || [];
    const at = list.indexOf(fn);
    if (at >= 0) { list.splice(at, 1); }
  }
  focus() {}
  blur() {}
  setSelectionRange() {}
  querySelectorAll() { return []; }
  get text() { return textOf(this); }
  get textContent() { return textOf(this); }
}

function decodeEntities(value) {
  return String(value).replace(/&(amp|lt|gt|quot|#39);/g, (whole, name) => ({
    amp: "&", lt: "<", gt: ">", quot: "\"", "#39": "'",
  }[name] || whole));
}

// Parser HTML toi thieu: du cho pane.html (the dung quy tac, khong long nhau sai).
export function parseHtml(html) {
  const root = new Element("#document");
  const stack = [root];
  let at = 0;
  const addText = (text) => {
    if (!text) { return; }
    stack[stack.length - 1].appendChild(new TextNode(decodeEntities(text)));
  };
  while (at < html.length) {
    const lt = html.indexOf("<", at);
    if (lt < 0) { addText(html.slice(at)); break; }
    if (lt > at) { addText(html.slice(at, lt)); }
    if (html.startsWith("<!--", lt)) {
      const end = html.indexOf("-->", lt);
      at = end < 0 ? html.length : end + 3;
      continue;
    }
    if (html.startsWith("<!", lt)) {
      const end = html.indexOf(">", lt);
      at = end < 0 ? html.length : end + 1;
      continue;
    }
    const gt = html.indexOf(">", lt);
    if (gt < 0) { break; }
    const raw = html.slice(lt + 1, gt).trim();
    at = gt + 1;
    if (raw.startsWith("/")) {
      const name = raw.slice(1).trim().toLowerCase();
      for (let k = stack.length - 1; k > 0; k--) {
        if (stack[k].tagName.toLowerCase() === name) { stack.length = k; break; }
      }
      continue;
    }
    const selfClosing = raw.endsWith("/");
    const body = selfClosing ? raw.slice(0, -1) : raw;
    const name = (/^[a-zA-Z0-9-]+/.exec(body) || [""])[0].toLowerCase();
    const node = new Element(name);
    for (const found of body.slice(name.length)
      .matchAll(/([a-zA-Z_:][-a-zA-Z0-9_:.]*)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+)))?/g)) {
      const value = found[2] !== undefined ? found[2] : (found[3] !== undefined ? found[3] : (found[4] || ""));
      node.setAttribute(found[1], decodeEntities(value));
    }
    stack[stack.length - 1].appendChild(node);
    if (name === "script" || name === "style") {
      // Bo qua noi dung: trong <script> co the co "<" khong phai the.
      const end = html.toLowerCase().indexOf("</" + name, at);
      at = end < 0 ? html.length : end;
      continue;
    }
    if (!selfClosing && !VOID_TAGS.has(name)) { stack.push(node); }
  }
  return root;
}

export function textOf(node) {
  if (!node) { return ""; }
  if (node.nodeType === 3) { return node.text; }
  return node.childNodes.map(textOf).join("");
}

export function byId(root, id) {
  if (root.nodeType === 1 && root.id === id) { return root; }
  for (const child of root.childNodes) {
    if (child.nodeType !== 1) { continue; }
    const found = byId(child, id);
    if (found) { return found; }
  }
  return null;
}

// Bam that: chay listener cua chinh phan tu roi noi bot len cac to tien (nhu trinh duyet).
export function click(node) {
  let at = node;
  while (at) {
    for (const fn of (at.listeners.click || []).slice()) {
      fn({ type: "click", target: node, srcElement: node, preventDefault() {}, stopPropagation() {} });
    }
    at = at.parentNode;
  }
}

export function press(node, keyCode, shiftKey) {
  let at = node;
  while (at) {
    for (const fn of (at.listeners.keydown || []).slice()) {
      fn({ type: "keydown", target: node, keyCode, which: keyCode, shiftKey: !!shiftKey,
        preventDefault() {}, stopPropagation() {} });
    }
    at = at.parentNode;
  }
}

export function type(node, value) {
  node.value = value;
  for (const fn of (node.listeners.input || []).slice()) {
    fn({ type: "input", target: node });
  }
}

export function find(root, test) {
  if (root.nodeType === 1 && test(root)) { return root; }
  for (const child of root.childNodes) {
    if (child.nodeType !== 1) { continue; }
    const found = find(child, test);
    if (found) { return found; }
  }
  return null;
}

export function findAll(root, test, out = []) {
  if (root.nodeType === 1 && test(root)) { out.push(root); }
  for (const child of root.childNodes) {
    if (child.nodeType === 1) { findAll(child, test, out); }
  }
  return out;
}

// Nap pane.html: dung cay DOM that tu chinh file do (khong chep tay markup) roi chay script trong pane.
// navigator: chi can cho nut "Sao chep API URL" (clipboard) - CEF co navigator.clipboard, harness thi
// khong, nen mac dinh la khong co (pane phai chiu duoc dieu do va roi ve loi nhac chon tay).
// page: "pane.html" (khung chat lon) hoac "mini.html" (khung chat mini canh doan boi den) - ca hai dung pane-core.js.
export function loadPane({ search = "", fetch, confirm, navigator, page = "pane.html" } = {}) {
  const html = fs.readFileSync(path.join(ADDIN, page), "utf8");
  const document = parseHtml(html);
  const body = findTag(document, "body");
  const inline = /<script>([\s\S]*?)<\/script>/.exec(html);
  if (!inline) { throw new Error(page + " khong co <script> trong trang"); }
  const context = {
    console, setTimeout, clearTimeout, setInterval, clearInterval, Promise, JSON, Date, URL, Math, String,
    Number, Boolean, Array, Object, RegExp, isNaN, encodeURIComponent, decodeURIComponent,
  };
  context.window = context;
  context.globalThis = context;
  context.document = document;
  document.body = body || new Element("body");
  document.createElement = (tag) => new Element(tag);
  document.createTextNode = (text) => new TextNode(text);
  document.createDocumentFragment = () => new Element("#fragment");
  document.getElementById = (id) => byId(document, id);
  context.location = { search, href: "http://127.0.0.1:47840/wps/addin/" + page + search };
  // window.close() cua khung chat mini (ShowDialog): ghi lai de test kiem nut Dong.
  context.closed = 0;
  context.close = () => { context.closed += 1; };
  context.innerWidth = 420;
  context.addEventListener = () => {};
  context.removeEventListener = () => {};
  context.fetch = fetch || (() => Promise.resolve({
    ok: true, status: 200, json: () => Promise.resolve({ ok: true, result: {} }),
  }));
  context.confirm = confirm || (() => false);
  context.navigator = navigator;
  // EventSource gia: ghi lai tung lan mo, day su kien SSE bang tay (giong tests/wps/test_pane.mjs).
  const sources = [];
  context.EventSource = function EventSource(url) {
    this.url = String(url);
    this.handlers = {};
    this.closed = false;
    sources.push(this);
  };
  context.EventSource.prototype.addEventListener = function (type, fn) {
    (this.handlers[type] = this.handlers[type] || []).push(fn);
  };
  context.EventSource.prototype.close = function () { this.closed = true; };
  context.EventSource.prototype.emit = function (type, data) {
    for (const fn of this.handlers[type] || []) { fn({ type, data: JSON.stringify(data) }); }
  };
  vm.createContext(context);
  vm.runInContext(fs.readFileSync(path.join(ADDIN, "pane-core.js"), "utf8"), context, { filename: "pane-core.js" });
  vm.runInContext(inline[1], context, { filename: "pane.html:inline" });
  return {
    document,
    context,
    sources,
    byId: (id) => byId(document, id),
    text: (id) => textOf(byId(document, id)),
    click: (id) => { const node = typeof id === "string" ? byId(document, id) : id; click(node); return node; },
    find: (test) => find(document, test),
    findAll: (test) => findAll(document, test),
    // Su kien SSE den tu lan mo EventSource moi nhat (luot chay dang chay).
    emit: (type, data) => {
      const source = sources[sources.length - 1];
      if (!source) { throw new Error("pane chua mo EventSource nao"); }
      source.emit(type, data);
    },
  };
}

function findTag(root, tag) {
  if (root.nodeType === 1 && root.tagName.toLowerCase() === tag) { return root; }
  for (const child of root.childNodes) {
    if (child.nodeType !== 1) { continue; }
    const found = findTag(child, tag);
    if (found) { return found; }
  }
  return null;
}

export { Element, TextNode };
