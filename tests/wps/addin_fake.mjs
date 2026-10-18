// Mo hinh do hoa (fake) cua object model WPS cho test add-in offline.
//
//   node --test tests/wps/test_addin_et.mjs
//   node --test tests/wps/test_addin_wpp.mjs
//
// Spec goi AxiomCommands.execute(makeEtApp(opts), "et.xxx", params) roi kiem
//   (a) ket qua tra ve, (b) callsOf(app) - nhung loi goi API add-in da thuc hien.
//
// Nguyen tac:
// - Dung nhu WPS that 11.1.0.11723 da do: chi muc 1-based, Range("A1") la LOI GOI.
// - KHONG co UndoRecord tren bat ky object nao -> undefined (app/workbook/sheet/presentation).
// - Thuoc tinh KHONG duoc mo phong -> nem Error("fake chua mo phong: <duong dan>"),
//   khong bao gio tra undefined im lang (tru danh sach ngoai le o ALWAYS_UNDEFINED).
// - Count/length tra so that; collection .Item(n) ngoai pham vi -> Error("index out of range").
// - FileSystem giu BYTE nhu file that (ghi chu day du o makeFileSystem): chi `ReadFile` doc duoc duong dan,
//   `readFileString`/`writeFileString` bi sandbox, `readAsBinaryString` lam hong byte >= 0x80.
// - Moi thu deterministic: khong Date.now, khong random, khong I/O mang.
//
// ESM hien dai (chay node --test), khong phu thuoc thu vien ngoai.

// ---------------------------------------------------------------------------------------------
// So dang ky noi bo
// ---------------------------------------------------------------------------------------------

const LABELS = new WeakMap();       // proxy -> nhan dung khi in ra log
const APPS = new WeakMap();         // bat ky proxy nao -> state goc cua app (chua calls)
const SHEET_REF = new WeakMap();    // proxy worksheet -> { wb, sheet }

// Thuoc tinh "do su ton tai" add-in hay cham, duoc tra undefined thay vi nem.
const ALWAYS_UNDEFINED = ["UndoRecord"];
const APP_OPTIONAL_UNDEFINED = ["UndoRecord", "WindowState", "ScreenUpdating", "DisplayAlerts", "EnableEvents"];

// ---------------------------------------------------------------------------------------------
// Tien ich chung
// ---------------------------------------------------------------------------------------------

function push(state, line) {
  state.calls.push(line);
}

function member(prefix, name) {
  return prefix === "" ? name : prefix + "." + name;
}

// In gia tri truyen vao cho NGAN GON: chuoi tran, mang JSON, proxy -> nhan cua no.
function fmt(value) {
  if (value === undefined) return "undefined";
  if (value === null) return "null";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  if (Array.isArray(value)) return JSON.stringify(value);
  const label = LABELS.get(value);
  if (label !== undefined) return label;
  try { return JSON.stringify(value); } catch (e) { return String(value); }
}

function fmtArgs(args) {
  return Array.prototype.map.call(args, fmt).join(",");
}

// Proxy nghiem: props[key] la { get, set? }; methods[key] la ham; con lai -> nem.
function strict(path, spec) {
  const props = spec.props || {};
  const methods = spec.methods || {};
  const allowed = spec.allowed || [];
  const state = spec.state;
  const hasProp = (key) => Object.prototype.hasOwnProperty.call(props, key);

  const handler = {
    get(_t, key) {
      if (typeof key === "symbol") {
        if (key === Symbol.toPrimitive) return () => path;
        if (key === Symbol.toStringTag) return "Object";
        return undefined;
      }
      if (key === "then" || key === "toJSON") return undefined;
      if (key === "toString") return () => "[object Object]";
      if (key === "valueOf") return () => path;
      if (key === "constructor") return Object;
      if (key === "inspect" || key === "nodeType" || key === "asymmetricMatch") return undefined;
      if (typeof key === "string") {
        if (hasProp(key)) {
          const d = props[key];
          return d.get ? d.get() : undefined;
        }
        if (key === "length" && hasProp("Count")) return props.Count.get();
        if (Object.prototype.hasOwnProperty.call(methods, key)) {
          const fn = methods[key];
          return function (...args) { return fn(...args); };
        }
      }
      if (allowed.indexOf(key) >= 0) return undefined;
      throw new Error("fake chua mo phong: " + path + "." + key);
    },
    set(_t, key, value) {
      if (hasProp(key)) {
        const d = props[key];
        if (d.set) { d.set(value); return true; }
      }
      throw new Error("fake chua mo phong: " + path + "." + key);
    },
    has(_t, key) {
      if (typeof key === "symbol") return false;
      return hasProp(key) || Object.prototype.hasOwnProperty.call(methods, key);
    },
  };
  if (spec.call) {
    handler.apply = (_t, _thisArg, args) => spec.call.apply(null, args);
  }
  const proxy = new Proxy({}, handler);
  LABELS.set(proxy, path);
  if (state) APPS.set(proxy, state);
  return proxy;
}

// ---------------------------------------------------------------------------------------------
// Dia chi A1 <-> (row, col) 1-based
// ---------------------------------------------------------------------------------------------

function lettersToNum(letters) {
  let n = 0;
  for (const ch of String(letters).toUpperCase()) n = n * 26 + (ch.charCodeAt(0) - 64);
  return n;
}

function numToLetters(n) {
  let s = "";
  let x = n;
  while (x > 0) {
    const r = (x - 1) % 26;
    s = String.fromCharCode(65 + r) + s;
    x = Math.floor((x - 1) / 26);
  }
  return s;
}

function normalizeAddr(addr) {
  return String(addr).replace(/\$/g, "").trim().toUpperCase();
}

function parseRangeAddr(addr) {
  const s = normalizeAddr(addr);
  const m = s.match(/^([A-Z]+)(\d+)(?::([A-Z]+)(\d+))?$/);
  if (!m) return null;
  const c1 = lettersToNum(m[1]);
  const r1 = parseInt(m[2], 10);
  const c2 = m[3] ? lettersToNum(m[3]) : c1;
  const r2 = m[4] ? parseInt(m[4], 10) : r1;
  if (r1 < 1 || c1 < 1 || r2 < 1 || c2 < 1) return null;
  return {
    r1: Math.min(r1, r2), c1: Math.min(c1, c2),
    r2: Math.max(r1, r2), c2: Math.max(c1, c2),
  };
}

function absAddr(r1, c1, r2, c2) {
  const a = "$" + numToLetters(c1) + "$" + r1;
  if (r2 === r1 && c2 === c1) return a;
  return a + ":$" + numToLetters(c2) + "$" + r2;
}

// ---------------------------------------------------------------------------------------------
// O, cong thuc, dinh dang
// ---------------------------------------------------------------------------------------------

function cellKey(r, c) {
  return r + "," + c;
}

const SHEET_MAX_ROW = 1048576;
const SHEET_MAX_COL = 16384;
const COL_WIDTH = 63.35;
const ROW_HEIGHT = 14.4;

function readCell(sheet, r, c) {
  return sheet.cells.get(cellKey(r, c)) || null;
}

function cellValue(sheet, r, c) {
  const cell = readCell(sheet, r, c);
  if (!cell || cell.err) return null;
  return cell.v;
}

function cellFormula(sheet, r, c) {
  const cell = readCell(sheet, r, c);
  return cell && cell.f ? cell.f : null;
}

function cellText(sheet, r, c) {
  const cell = readCell(sheet, r, c);
  if (!cell) return "";
  if (cell.err) return cell.err;
  if (cell.v === null || cell.v === undefined) return "";
  if (typeof cell.v === "number") return String(cell.v);
  if (typeof cell.v === "boolean") return cell.v ? "TRUE" : "FALSE";
  return String(cell.v);
}

// Tinh cong thuc "don gian nhu WPS": =<dia chi>, =<so>*<so>, =A1*2, =1/0 -> #DIV/0!,
// con lai coi nhu 0 nhung VAN nho chuoi cong thuc.
function evalFormula(sheet, formula, depth) {
  const d = depth || 0;
  if (d > 32) return { v: 0, err: null };
  const body = String(formula).replace(/^=/, "").trim();
  const m = body.match(/^(.+?)([+\-*/])(.+)$/);
  if (m) {
    const left = evalOperand(sheet, m[1].trim(), d);
    const right = evalOperand(sheet, m[3].trim(), d);
    if (left.err || right.err) return { v: null, err: left.err || right.err };
    const a = left.v;
    const b = right.v;
    if (m[2] === "/") {
      if (b === 0) return { v: null, err: "#DIV/0!" };
      return { v: a / b, err: null };
    }
    if (m[2] === "*") return { v: a * b, err: null };
    if (m[2] === "+") return { v: a + b, err: null };
    return { v: a - b, err: null };
  }
  return evalOperand(sheet, body, d);
}

function evalOperand(sheet, token, depth) {
  let m = token.match(/^\$?([A-Za-z]+)\$?(\d+)$/);
  if (m) {
    const r = parseInt(m[2], 10);
    const c = lettersToNum(m[1]);
    const cell = readCell(sheet, r, c);
    if (!cell) return { v: 0, err: null };          // o trong -> 0 khi dung trong cong thuc
    if (cell.err) return { v: null, err: cell.err };
    if (cell.f) return evalFormula(sheet, cell.f, depth + 1);
    if (cell.v === null || cell.v === undefined) return { v: 0, err: null };
    return { v: cell.v, err: null };
  }
  m = token.match(/^(\d+(?:\.\d+)?)$/);
  if (m) return { v: Number(m[1]), err: null };
  return { v: 0, err: null };
}

function translateFormula(formula, dRow, dCol) {
  if (dRow === 0 && dCol === 0) return formula;
  return String(formula).replace(/(\$?)([A-Za-z]+)(\$?)(\d+)/g, (all, dcol, letters, drow, digits) => {
    let c = lettersToNum(letters);
    let r = parseInt(digits, 10);
    if (!dcol) c += dCol;
    if (!drow) r += dRow;
    if (c < 1 || r < 1) return all;
    return (dcol || "") + numToLetters(c) + (drow || "") + String(r);
  });
}

// LUU Y (lech da do, chua mo phong): WPS ET EP KIEU chuoi trong nhu so / nhu ngay khi gan vao `Value2`
// ("42" -> 42, "01/01" -> ngay kieu d-mmm; do tren WPS that 11/10/2026). Fake giu nguyen chuoi, nen mot lenh
// GHI chuoi trong nhu so se ra ket qua khac WPS that. Duong DOC thi dung: mot o da co chu trong file (seed o
// day, hoac file nguoi dung mo) van la chu that. Xem docs/wps-bridge.md muc 6.
function setCellScalar(sheet, r, c, value) {
  const key = cellKey(r, c);
  if (value === null || value === undefined || value === "") {
    sheet.cells.delete(key);
    return;
  }
  if (typeof value === "string" && value.charAt(0) === "=") {
    setCellFormula(sheet, r, c, value);
    return;
  }
  sheet.cells.set(key, { v: value, f: null, err: null });
}

function setCellFormula(sheet, r, c, formula) {
  const res = evalFormula(sheet, formula, 0);
  sheet.cells.set(cellKey(r, c), { v: res.err ? null : res.v, f: formula, err: res.err || null });
}

function styleAt(sheet, r, c) {
  return sheet.styles.get(cellKey(r, c)) || {};
}

function ensureStyle(sheet, r, c) {
  const key = cellKey(r, c);
  let s = sheet.styles.get(key);
  if (!s) { s = { font: {}, interior: {} }; sheet.styles.set(key, s); }
  return s;
}

// ---------------------------------------------------------------------------------------------
// Range (ET)
// ---------------------------------------------------------------------------------------------

function rangeRows(box) { return box.r2 - box.r1 + 1; }
function rangeCols(box) { return box.c2 - box.c1 + 1; }

function forEachCell(box, fn) {
  for (let r = box.r1; r <= box.r2; r++) {
    for (let c = box.c1; c <= box.c2; c++) fn(r, c);
  }
}

function makeCountCollection(state, count, makeItem, prefix) {
  return strict(prefix, {
    state,
    props: { Count: { get: () => count } },
    methods: {
      Item: (n) => {
        const i = Math.round(Number(n));
        if (!(i >= 1 && i <= count)) throw new Error("index out of range");
        if (!makeItem) throw new Error("fake chua mo phong: " + prefix + ".Item");
        return makeItem(i);
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeFont(state, sheet, box) {
  const prefix = "Font";
  const label = { bold: "Bold", italic: "Italic", size: "Size", color: "Color", name: "Name" };
  const setAll = (field, value) => {
    forEachCell(box, (r, c) => {
      const s = ensureStyle(sheet, r, c);
      if (!s.font) s.font = {};
      s.font[field] = value;
    });
    push(state, prefix + "." + label[field] + "=" + fmt(value));
  };
  const getAll = (field, dflt) => {
    const s = styleAt(sheet, box.r1, box.c1);
    return (s.font && s.font[field] !== undefined) ? s.font[field] : dflt;
  };
  return strict(prefix, {
    state,
    props: {
      Bold: { get: () => getAll("bold", false), set: (v) => setAll("bold", v) },
      Italic: { get: () => getAll("italic", false), set: (v) => setAll("italic", v) },
      Size: { get: () => getAll("size", 11), set: (v) => setAll("size", v) },
      Color: { get: () => getAll("color", 0), set: (v) => setAll("color", v) },
      Name: { get: () => getAll("name", "Arial"), set: (v) => setAll("name", v) },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeInterior(state, sheet, box) {
  const prefix = "Interior";
  const label = { color: "Color", pattern: "Pattern" };
  const setAll = (field, value) => {
    forEachCell(box, (r, c) => {
      const s = ensureStyle(sheet, r, c);
      if (!s.interior) s.interior = {};
      s.interior[field] = value;
    });
    push(state, prefix + "." + label[field] + "=" + fmt(value));
  };
  const getAll = (field, dflt) => {
    const s = styleAt(sheet, box.r1, box.c1);
    return (s.interior && s.interior[field] !== undefined) ? s.interior[field] : dflt;
  };
  return strict(prefix, {
    state,
    props: {
      Color: { get: () => getAll("color", 0), set: (v) => setAll("color", v) },
      Pattern: { get: () => getAll("pattern", 1), set: (v) => setAll("pattern", v) },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeSort(state, prefix) {
  return strict(prefix + ".Sort", {
    state,
    props: {
      Header: { get: () => 1, set: () => {} },
      Key1: { get: () => null, set: () => {} },
      Order1: { get: () => 1, set: () => {} },
      Orientation: { get: () => 1, set: () => {} },
    },
    methods: {
      Apply: () => push(state, prefix + ".Sort.Apply()"),
      SetRange: (...args) => push(state, prefix + ".Sort.SetRange(" + fmtArgs(args) + ")"),
    },
    call: (...args) => push(state, prefix + ".Sort(" + fmtArgs(args) + ")"),
    allowed: ALWAYS_UNDEFINED,
  });
}

// ---------------------------------------------------------------------------------------------
// FormatConditions (ET): dinh dang dieu kien kieu Excel/VBA
//   Type 1 = xlCellValue (Operator 1..8), Type 2 = xlExpression (khong Operator)
//   Range("A1:B2").FormatConditions.Add(...) -> doi tuong; .Count/.Item(n)/.Delete()
//   Doi tuong con: .Type/.Operator/.Formula1/.Formula2/.Interior.Color/.Font.{Bold,Italic,Color}
// Mot dinh dang gan vao mot VUNG: moi o trong vung do deu "thay" no (giong Excel), nen
// Range("A1").FormatConditions.Count = 1 khi dinh dang nam tren A1:B2.
// ---------------------------------------------------------------------------------------------

const FC_TYPE_CELL_VALUE = 1;

function boxContains(outer, inner) {
  return outer.r1 <= inner.r1 && outer.c1 <= inner.c1 && outer.r2 >= inner.r2 && outer.c2 >= inner.c2;
}

function fcInterior(state, fc, prefix) {
  const path = prefix + ".Interior";
  return strict(path, {
    state,
    props: {
      Color: { get: () => (fc.interiorColor === null ? 0 : fc.interiorColor),
        set: (v) => { fc.interiorColor = v; push(state, path + ".Color=" + fmt(v)); } },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function fcFont(state, fc, prefix) {
  const path = prefix + ".Font";
  return strict(path, {
    state,
    props: {
      Bold: { get: () => (fc.bold === null ? false : fc.bold),
        set: (v) => { fc.bold = v; push(state, path + ".Bold=" + fmt(v)); } },
      Italic: { get: () => (fc.italic === null ? false : fc.italic),
        set: (v) => { fc.italic = v; push(state, path + ".Italic=" + fmt(v)); } },
      Color: { get: () => (fc.fontColor === null ? 0 : fc.fontColor),
        set: (v) => { fc.fontColor = v; push(state, path + ".Color=" + fmt(v)); } },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeFormatCondition(state, sheet, fc, index) {
  const prefix = "FormatConditions.Item(" + index + ")";
  return strict(prefix, {
    state,
    props: {
      Type: { get: () => fc.type },
      Operator: { get: () => fc.operator },
      Formula1: { get: () => fc.formula1, set: (v) => { fc.formula1 = String(v); push(state, prefix + ".Formula1=" + fmt(v)); } },
      Formula2: { get: () => fc.formula2, set: (v) => { fc.formula2 = String(v); push(state, prefix + ".Formula2=" + fmt(v)); } },
      Interior: { get: () => fcInterior(state, fc, prefix) },
      Font: { get: () => fcFont(state, fc, prefix) },
    },
    methods: {
      Delete: () => {
        const i = sheet.formatConditions.indexOf(fc);
        if (i >= 0) sheet.formatConditions.splice(i, 1);
        push(state, prefix + ".Delete()");
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeFormatConditions(state, box, sheet, rangePrefix) {
  const prefix = rangePrefix + ".FormatConditions";
  const mine = () => sheet.formatConditions.filter((fc) => boxContains(fc.box, box));
  return strict(prefix, {
    state,
    props: { Count: { get: () => mine().length } },
    methods: {
      Add: (...args) => {
        const [type, operator, formula1, formula2] = args;
        const fc = {
          box: { r1: box.r1, c1: box.c1, r2: box.r2, c2: box.c2 },
          type: type === undefined || type === null ? FC_TYPE_CELL_VALUE : Math.round(Number(type)),
          operator: operator === undefined || operator === null ? null : Math.round(Number(operator)),
          formula1: formula1 === undefined || formula1 === null ? null : String(formula1),
          formula2: formula2 === undefined || formula2 === null ? null : String(formula2),
          interiorColor: null, fontColor: null, bold: null, italic: null,
        };
        push(state, prefix + ".Add(" + fmtArgs(args) + ")");
        const existing = sheet.formatConditions.filter((other) => boxContains(other.box, box));
        sheet.formatConditions.push(fc);
        return makeFormatCondition(state, sheet, fc, existing.length + 1);
      },
      Item: (n) => {
        const items = mine();
        const i = Math.round(Number(n));
        if (!(i >= 1 && i <= items.length)) throw new Error("index out of range");
        return makeFormatCondition(state, sheet, items[i - 1], i);
      },
      Delete: () => {
        const items = mine();
        for (const fc of items) {
          const i = sheet.formatConditions.indexOf(fc);
          if (i >= 0) sheet.formatConditions.splice(i, 1);
        }
        push(state, prefix + ".Delete()");
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeRange(state, sheet, box, labelPath) {
  const prefix = labelPath || ("Range(" + absAddr(box.r1, box.c1, box.r2, box.c2).replace(/\$/g, "") + ")");
  let self = null;
  const rows = rangeRows(box);
  const cols = rangeCols(box);
  const single = rows === 1 && cols === 1;

  function readMatrix(getter) {
    const out = [];
    for (let r = box.r1; r <= box.r2; r++) {
      const row = [];
      for (let c = box.c1; c <= box.c2; c++) row.push(getter(sheet, r, c));
      out.push(row);
    }
    return out;
  }

  function subRange(r1, c1, r2, c2, label) {
    return makeRange(state, sheet, { r1, c1, r2, c2 }, label);
  }

  const props = {
    Value2: {
      get: () => (single ? cellValue(sheet, box.r1, box.c1) : readMatrix(cellValue)),
      set: (value) => {
        push(state, prefix + ".Value2=" + fmt(value));
        if (Array.isArray(value)) {
          if (value.length === 0 || !Array.isArray(value[0])) {
            throw new Error("Value2: can mang 2 chieu (nhan duoc mang 1 chieu)");
          }
          for (let i = 0; i < value.length; i++) {
            const row = value[i];
            if (!Array.isArray(row)) throw new Error("Value2: can mang 2 chieu (hang " + (i + 1) + " khong phai mang)");
            for (let j = 0; j < row.length; j++) setCellScalar(sheet, box.r1 + i, box.c1 + j, row[j]);
          }
          return;
        }
        if (value === null || value === undefined) {
          forEachCell(box, (r, c) => sheet.cells.delete(cellKey(r, c)));
          return;
        }
        forEachCell(box, (r, c) => {
          if (typeof value === "string" && value.charAt(0) === "=") {
            setCellFormula(sheet, r, c, translateFormula(value, r - box.r1, c - box.c1));
          } else {
            setCellScalar(sheet, r, c, value);
          }
        });
      },
    },
    Formula: {
      get: () => (single ? cellFormula(sheet, box.r1, box.c1) : readMatrix(cellFormula)),
      set: (value) => {
        push(state, prefix + ".Formula=" + fmt(value));
        if (typeof value === "string" && value.charAt(0) === "=") {
          forEachCell(box, (r, c) => setCellFormula(sheet, r, c, translateFormula(value, r - box.r1, c - box.c1)));
          return;
        }
        if (typeof value === "string") {
          forEachCell(box, (r, c) => setCellScalar(sheet, r, c, value));
          return;
        }
        if (value === null || value === undefined) {
          forEachCell(box, (r, c) => sheet.cells.delete(cellKey(r, c)));
          return;
        }
        forEachCell(box, (r, c) => setCellScalar(sheet, r, c, value));
      },
    },
    Text: {
      get: () => (single ? cellText(sheet, box.r1, box.c1) : readMatrix(cellText)),
    },
    Row: { get: () => box.r1 },
    Column: { get: () => box.c1 },
    Count: { get: () => rows * cols },
    Left: { get: () => (box.c1 - 1) * COL_WIDTH },
    Top: { get: () => (box.r1 - 1) * ROW_HEIGHT },
    Width: { get: () => cols * COL_WIDTH },
    Height: { get: () => rows * ROW_HEIGHT },
    Rows: {
      get: () => makeCountCollection(state, rows,
        (n) => subRange(box.r1 + n - 1, box.c1, box.r1 + n - 1, box.c2, prefix + ".Rows.Item(" + n + ")"),
        prefix + ".Rows"),
    },
    Columns: {
      get: () => makeCountCollection(state, cols,
        (n) => subRange(box.r1, box.c1 + n - 1, box.r2, box.c1 + n - 1, prefix + ".Columns.Item(" + n + ")"),
        prefix + ".Columns"),
    },
    Cells: { get: () => self },
    Font: { get: () => makeFont(state, sheet, box) },
    Interior: { get: () => makeInterior(state, sheet, box) },
    NumberFormat: {
      get: () => styleAt(sheet, box.r1, box.c1).numFmt || "General",
      set: (v) => {
        forEachCell(box, (r, c) => { ensureStyle(sheet, r, c).numFmt = v; });
        push(state, prefix + ".NumberFormat=" + fmt(v));
      },
    },
    HorizontalAlignment: {
      get: () => styleAt(sheet, box.r1, box.c1).halign || null,
      set: (v) => {
        forEachCell(box, (r, c) => { ensureStyle(sheet, r, c).halign = v; });
        push(state, prefix + ".HorizontalAlignment=" + fmt(v));
      },
    },
    WrapText: {
      get: () => styleAt(sheet, box.r1, box.c1).wrap === true,
      set: (v) => {
        forEachCell(box, (r, c) => { ensureStyle(sheet, r, c).wrap = !!v; });
        push(state, prefix + ".WrapText=" + fmt(v));
      },
    },
    MergeCells: {
      get: () => styleAt(sheet, box.r1, box.c1).merged === true,
      set: (v) => {
        ensureStyle(sheet, box.r1, box.c1).merged = !!v;
        push(state, prefix + ".MergeCells=" + fmt(v));
      },
    },
    EntireColumn: { get: () => subRange(1, box.c1, SHEET_MAX_ROW, box.c2, "Range(" + numToLetters(box.c1) + ":" + numToLetters(box.c2) + ")") },
    EntireRow: { get: () => subRange(box.r1, 1, box.r2, SHEET_MAX_COL, "Range(" + box.r1 + ":" + box.r2 + ")") },
    ColumnWidth: {
      get: () => sheet.colWidths.get(box.c1) || 8.43,
      set: (v) => {
        for (let c = box.c1; c <= box.c2; c++) sheet.colWidths.set(c, v);
        push(state, prefix + ".ColumnWidth=" + fmt(v));
      },
    },
    Sort: { get: () => makeSort(state, prefix) },
    FormatConditions: { get: () => makeFormatConditions(state, box, sheet, prefix) },
  };

  const methods = {
    Address: () => absAddr(box.r1, box.c1, box.r2, box.c2),
    Item: (r, c) => {
      const rr = Math.round(Number(r));
      const cc = Math.round(Number(c));
      if (!(rr >= 1 && rr <= rows && cc >= 1 && cc <= cols)) throw new Error("index out of range");
      return subRange(box.r1 + rr - 1, box.c1 + cc - 1, box.r1 + rr - 1, box.c1 + cc - 1, prefix + ".Item(" + rr + "," + cc + ")");
    },
    Merge: () => {
      ensureStyle(sheet, box.r1, box.c1).merged = true;
      push(state, prefix + ".Merge()");
    },
    AutoFit: () => push(state, prefix + ".AutoFit()"),
    Select: () => push(state, prefix + ".Select()"),
    AutoFill: (...args) => push(state, prefix + ".AutoFill(" + fmtArgs(args) + ")"),
  };

  self = strict(prefix, { state, props, methods, allowed: ALWAYS_UNDEFINED });
  return self;
}

// ---------------------------------------------------------------------------------------------
// ChartObjects (ET)
// ---------------------------------------------------------------------------------------------

function makeChart(state, chartState, objPrefix) {
  const prefix = objPrefix + ".Chart";
  return strict(prefix, {
    state,
    props: {
      ChartType: {
        get: () => chartState.chartType,
        set: (v) => { chartState.chartType = v; push(state, prefix + ".ChartType=" + fmt(v)); },
      },
      HasTitle: {
        get: () => chartState.hasTitle,
        set: (v) => { chartState.hasTitle = !!v; push(state, prefix + ".HasTitle=" + fmt(v)); },
      },
      ChartTitle: {
        get: () => strict(prefix + ".ChartTitle", {
          state,
          props: {
            Text: {
              get: () => chartState.titleText,
              set: (v) => { chartState.titleText = String(v); push(state, prefix + ".ChartTitle.Text=" + fmt(v)); },
            },
          },
          allowed: ALWAYS_UNDEFINED,
        }),
      },
    },
    methods: {
      SetSourceData: (...args) => push(state, prefix + ".SetSourceData(" + fmtArgs(args) + ")"),
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeChartObject(state, sheet, chartState, index) {
  const prefix = "ChartObjects.Item(" + index + ")";
  return strict(prefix, {
    state,
    props: {
      Name: { get: () => chartState.name },
      Left: { get: () => chartState.left, set: (v) => { chartState.left = v; push(state, prefix + ".Left=" + fmt(v)); } },
      Top: { get: () => chartState.top, set: (v) => { chartState.top = v; push(state, prefix + ".Top=" + fmt(v)); } },
      Width: { get: () => chartState.width, set: (v) => { chartState.width = v; push(state, prefix + ".Width=" + fmt(v)); } },
      Height: { get: () => chartState.height, set: (v) => { chartState.height = v; push(state, prefix + ".Height=" + fmt(v)); } },
      Chart: { get: () => makeChart(state, chartState, prefix) },
    },
    methods: {
      Delete: () => {
        const i = sheet.charts.indexOf(chartState);
        if (i >= 0) sheet.charts.splice(i, 1);
        push(state, prefix + ".Delete()");
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeChartObjects(state, sheet) {
  const prefix = "ChartObjects";
  return strict(prefix, {
    state,
    props: { Count: { get: () => sheet.charts.length } },
    methods: {
      Item: (n) => {
        const i = Math.round(Number(n));
        if (!(i >= 1 && i <= sheet.charts.length)) throw new Error("index out of range");
        return makeChartObject(state, sheet, sheet.charts[i - 1], i);
      },
      Add: (...args) => {
        push(state, prefix + ".Add(" + fmtArgs(args) + ")");
        sheet.chartSeq += 1;
        const cs = {
          name: "Chart " + sheet.chartSeq,
          chartType: null,
          hasTitle: false,
          titleText: "",
          left: args.length > 0 ? args[0] : 0,
          top: args.length > 1 ? args[1] : 0,
          width: args.length > 2 ? args[2] : 0,
          height: args.length > 3 ? args[3] : 0,
        };
        sheet.charts.push(cs);
        return makeChartObject(state, sheet, cs, sheet.charts.length);
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

// ---------------------------------------------------------------------------------------------
// Worksheet (ET)
// ---------------------------------------------------------------------------------------------

function makeCellsAbsolute(state, sheet, prefix) {
  return strict(prefix, {
    state,
    props: { Count: { get: () => SHEET_MAX_ROW * SHEET_MAX_COL } },
    methods: {
      Item: (r, c) => {
        const rr = Math.round(Number(r));
        const cc = Math.round(Number(c));
        if (!(rr >= 1 && rr <= SHEET_MAX_ROW && cc >= 1 && cc <= SHEET_MAX_COL)) throw new Error("index out of range");
        return makeRange(state, sheet, { r1: rr, c1: cc, r2: rr, c2: cc }, prefix + ".Item(" + rr + "," + cc + ")");
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function usedRangeBox(sheet) {
  let minR = Infinity, minC = Infinity, maxR = 0, maxC = 0;
  for (const key of sheet.cells.keys()) {
    const parts = key.split(",");
    const r = Number(parts[0]);
    const c = Number(parts[1]);
    if (r < minR) minR = r;
    if (c < minC) minC = c;
    if (r > maxR) maxR = r;
    if (c > maxC) maxC = c;
  }
  if (maxR === 0) return { r1: 1, c1: 1, r2: 1, c2: 1 };
  return { r1: minR, c1: minC, r2: maxR, c2: maxC };
}

function makeWorksheet(state, wb, sheet, labelPath) {
  const prefix = labelPath;
  const sheetIndex = () => wb.sheets.indexOf(sheet) + 1;

  const props = {
    Name: {
      get: () => sheet.name,
      set: (v) => {
        const name = String(v);
        if (wb.sheets.some((other) => other !== sheet && other.name === name)) {
          throw new Error("ten sheet da ton tai");
        }
        sheet.name = name;
        push(state, prefix + ".Name=" + fmt(v));
      },
    },
    Index: { get: () => sheetIndex() },
    Visible: {
      get: () => sheet.visible,
      set: (v) => { sheet.visible = !!v; push(state, prefix + ".Visible=" + fmt(v)); },
    },
    UsedRange: { get: () => makeRange(state, sheet, usedRangeBox(sheet), "UsedRange") },
    Cells: { get: () => makeCellsAbsolute(state, sheet, "Cells") },
    Rows: { get: () => makeCountCollection(state, SHEET_MAX_ROW, (n) => makeRange(state, sheet, { r1: n, c1: 1, r2: n, c2: SHEET_MAX_COL }, prefix + ".Rows.Item(" + n + ")"), prefix + ".Rows") },
    Columns: { get: () => makeCountCollection(state, SHEET_MAX_COL, (n) => makeRange(state, sheet, { r1: 1, c1: n, r2: SHEET_MAX_ROW, c2: n }, prefix + ".Columns.Item(" + n + ")"), prefix + ".Columns") },
    Shapes: { get: () => makeShapesEt(state, sheet) },
  };

  const methods = {
    Range: (addr) => {
      const box = parseRangeAddr(addr);
      if (!box) throw new Error("dia chi khong hop le: " + addr);
      return makeRange(state, sheet, box, "Range(" + normalizeAddr(addr) + ")");
    },
    ChartObjects: () => makeChartObjects(state, sheet),
    Delete: () => {
      const i = wb.sheets.indexOf(sheet);
      if (i >= 0) wb.sheets.splice(i, 1);
      if (wb.activeSheetIndex >= wb.sheets.length) wb.activeSheetIndex = wb.sheets.length - 1;
      push(state, prefix + ".Delete()");
    },
    Activate: () => {
      const i = wb.sheets.indexOf(sheet);
      if (i >= 0) wb.activeSheetIndex = i;
      push(state, prefix + ".Activate()");
    },
    Select: () => push(state, prefix + ".Select()"),
    Copy: (...args) => push(state, prefix + ".Copy(" + fmtArgs(args) + ")"),
  };

  const proxy = strict(prefix, { state, props, methods, allowed: ALWAYS_UNDEFINED });
  SHEET_REF.set(proxy, { wb, sheet });
  return proxy;
}

function makeShapesEt(state, sheet) {
  const prefix = "Shapes";
  return strict(prefix, {
    state,
    props: { Count: { get: () => sheet.shapes.length } },
    methods: {
      Item: (n) => {
        const i = Math.round(Number(n));
        if (!(i >= 1 && i <= sheet.shapes.length)) throw new Error("index out of range");
        return sheet.shapes[i - 1];
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

// ---------------------------------------------------------------------------------------------
// Sheets / Workbook / Workbooks (ET)
// ---------------------------------------------------------------------------------------------

function uniqueSheetName(wb) {
  let n = wb.sheets.length + 1;
  let name = "Sheet" + n;
  while (wb.sheets.some((s) => s.name === name)) { n += 1; name = "Sheet" + n; }
  return name;
}

function makeSheetsCollection(state, wb, name) {
  const prefix = name;
  return strict(prefix, {
    state,
    props: { Count: { get: () => wb.sheets.length } },
    methods: {
      Item: (n) => {
        const i = Math.round(Number(n));
        if (!(i >= 1 && i <= wb.sheets.length)) throw new Error("index out of range");
        return makeWorksheet(state, wb, wb.sheets[i - 1], "Worksheet.Item(" + i + ")");
      },
      // Add([Before], [After], [Count], [Type]) kieu Excel/VBA: chen TRUOC `Before` hoac SAU `After`
      // (nhan doi tuong worksheet hoac chi so 1-based); khong co thi chen vao cuoi.
      Add: (...args) => {
        const sheet = newSheet({ name: uniqueSheetName(wb) });
        const at = (value) => {
          if (value === undefined || value === null) return -1;
          if (typeof value === "number") {
            const i = Math.round(value) - 1;
            return i >= 0 && i < wb.sheets.length ? i : -1;
          }
          const ref = SHEET_REF.get(value);
          if (ref && ref.wb === wb) return wb.sheets.indexOf(ref.sheet);
          return wb.sheets.findIndex((s) => s.name === String(value));
        };
        const before = at(args.length > 0 ? args[0] : undefined);
        const after = at(args.length > 1 ? args[1] : undefined);
        const position = before >= 0 ? before : (after >= 0 ? after + 1 : wb.sheets.length);
        wb.sheets.splice(position, 0, sheet);
        push(state, prefix + ".Add(" + fmtArgs(args) + ")");
        return makeWorksheet(state, wb, sheet, "Worksheet.Item(" + (position + 1) + ")");
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function splitPath(fullPath) {
  const s = String(fullPath);
  const i = s.lastIndexOf("/");
  if (i < 0) return { dir: "", base: s };
  return { dir: s.substring(0, i) || "/", base: s.substring(i + 1) };
}

function makeWorkbook(state, wb, labelPath) {
  const prefix = labelPath;
  const props = {
    Name: { get: () => wb.name },
    FullName: { get: () => wb.fullPath },
    Path: { get: () => splitPath(wb.fullPath).dir },
    Saved: { get: () => wb.saved, set: (v) => { wb.saved = !!v; push(state, prefix + ".Saved=" + fmt(v)); } },
    ReadOnly: { get: () => false },
    Sheets: { get: () => makeSheetsCollection(state, wb, "Sheets") },
    Worksheets: { get: () => makeSheetsCollection(state, wb, "Worksheets") },
    ActiveSheet: {
      get: () => {
        if (wb.sheets.length === 0 || wb.activeSheetIndex < 0) return null;
        return makeWorksheet(state, wb, wb.sheets[wb.activeSheetIndex], "ActiveSheet");
      },
      set: (value) => {
        const ref = value && typeof value === "object" ? SHEET_REF.get(value) : null;
        if (ref && ref.wb === wb) { wb.activeSheetIndex = wb.sheets.indexOf(ref.sheet); return; }
        const i = wb.sheets.findIndex((s) => s.name === String(value));
        if (i < 0) throw new Error("no sheet named '" + String(value) + "'");
        wb.activeSheetIndex = i;
      },
    },
  };

  const methods = {
    Save: (...args) => { wb.saved = true; push(state, prefix + ".Save(" + fmtArgs(args) + ")"); },
    SaveAs2: (...args) => methods.SaveAs(...args),
    SaveAs: (...args) => {
      if (args.length >= 1 && args[0] !== undefined && args[0] !== null) {
        const full = String(args[0]);
        wb.fullPath = full;
        wb.name = splitPath(full).base;
      }
      wb.saved = true;
      push(state, prefix + ".SaveAs(" + fmtArgs(args) + ")");
    },
    Close: (...args) => {
      const i = state.workbooks.indexOf(wb);
      if (i >= 0) state.workbooks.splice(i, 1);
      if (state.activeWorkbookIndex >= state.workbooks.length) state.activeWorkbookIndex = state.workbooks.length - 1;
      if (state.activeWorkbookIndex < 0) state.activeWorkbookIndex = 0;
      wb.closed = true;
      push(state, prefix + ".Close(" + fmtArgs(args) + ")");
    },
    Activate: () => {
      const i = state.workbooks.indexOf(wb);
      if (i >= 0) state.activeWorkbookIndex = i;
      push(state, prefix + ".Activate()");
    },
    // WPS Linux do duoc: goi duoc, tra ve binh thuong, nhung KHONG tao file nao ca. Chi ghi log
    // (giong Presentation.ExportAsFixedFormat) - add-in phai tu kiem file va tu dung PDF.
    ExportAsFixedFormat: (...args) => push(state, prefix + ".ExportAsFixedFormat(" + fmtArgs(args) + ")"),
  };

  return strict(prefix, { state, props, methods, allowed: ALWAYS_UNDEFINED });
}

function newWorkbook({ name, fullPath }) {
  return {
    name: name || "Book1",
    fullPath: fullPath || ("/tmp/" + (name || "Book1")),
    saved: true,
    closed: false,
    sheets: [],
    activeSheetIndex: 0,
  };
}

function makeWorkbooksCollection(state) {
  const prefix = "Workbooks";
  return strict(prefix, {
    state,
    props: { Count: { get: () => state.workbooks.length } },
    methods: {
      Item: (n) => {
        const i = Math.round(Number(n));
        if (!(i >= 1 && i <= state.workbooks.length)) throw new Error("index out of range");
        return makeWorkbook(state, state.workbooks[i - 1], "Workbook");
      },
      Add: (...args) => {
        state.workbookSeq += 1;
        const name = "Book" + state.workbookSeq;
        const wb = newWorkbook({ name, fullPath: "/tmp/" + name });
        wb.sheets.push(newSheet({ name: "Sheet1" }));
        wb.activeSheetIndex = 0;
        state.workbooks.push(wb);
        state.activeWorkbookIndex = state.workbooks.length - 1;
        push(state, prefix + ".Add(" + fmtArgs(args) + ")");
        return makeWorkbook(state, wb, "Workbook");
      },
      Open: (...args) => {
        const full = args.length > 0 && args[0] !== undefined ? String(args[0]) : "/tmp/blank.xlsx";
        const wb = newWorkbook({ name: splitPath(full).base, fullPath: full });
        wb.sheets.push(newSheet({ name: "Sheet" }));
        wb.activeSheetIndex = 0;
        state.workbooks.push(wb);
        state.activeWorkbookIndex = state.workbooks.length - 1;
        push(state, prefix + ".Open(" + fmtArgs(args) + ")");
        return makeWorkbook(state, wb, "Workbook");
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

// ---------------------------------------------------------------------------------------------
// FileSystem / CommandBars / WorksheetFunction
// ---------------------------------------------------------------------------------------------

// Do tren WPS that 11/10/2026 (tests/wps/REFERENCE.md, muc "FileSystem"):
// - `readFileString`/`writeFileString` BI NHOT trong sandbox: nem `path cannot contains "/" or "\"` voi MOI
//   duong dan co dau phan cach (ke ca duong dan tuyet doi), tra `null` voi ten file tran, va nem `参数太多.`
//   khi thua tham so. Chung KHONG doc duoc file cua nguoi dung.
// - `ReadFile(path)` moi la API doc that: giai ma UTF-8, thieu file -> nem `"Wrong path!"`.
// - `readAsBinaryString` lam hong moi byte >= 0x80 (thanh `0xFF00 | b`); `writeAsBinaryString` ghi byte nguyen ven.
// Nen file trong fake giu BYTE nhu file that tren dia:
//   files: { "/tmp/a.csv": "Ten,So\n" }         -> ma hoa UTF-8
//   files: { "/tmp/a.bin": [0x41, 0xC3, 0xAA] } -> dung byte do (file khong phai UTF-8)

const UTF8_ENCODER = new TextEncoder();
// ignoreBOM: ReadFile tra ve nguyen noi dung byte cua file, ke ca BOM (chua do duoc truong hop BOM tren WPS that;
// chon huong "giu BOM" vi do la huong kho hon cho add-in - no phai tu cat theo `utf-8-sig`).
const UTF8_DECODER = new TextDecoder("utf-8", { ignoreBOM: true });

function fileBytes(value) {
  if (Array.isArray(value)) return Array.prototype.map.call(value, (b) => Number(b) & 0xFF);
  return Array.from(UTF8_ENCODER.encode(String(value)));
}

// "binary string" cua WPS: moi ky tu la mot byte. Ghi thi lay byte thap (& 0xFF) - nho vay chuoi doc ra tu
// readAsBinaryString (da bi doi thanh 0xFF00|b) van ghi lai duoc dung byte goc.
function binaryStringBytes(data) {
  const s = String(data);
  const out = [];
  for (let i = 0; i < s.length; i++) out.push(s.charCodeAt(i) & 0xFF);
  return out;
}

// ReadFile giai ma UTF-8 va DUNG lai o byte NUL dau tien (do tren WPS: byte [.., 0x00, 0xFF] -> chuoi 4 ky tu).
function readFileText(bytes) {
  const nul = bytes.indexOf(0);
  return UTF8_DECODER.decode(Uint8Array.from(nul >= 0 ? bytes.slice(0, nul) : bytes));
}

function filesWithBytes(files) {
  const out = {};
  for (const [path, value] of Object.entries(files || {})) {
    out[path] = fileBytes(value);
  }
  return out;
}

const SANDBOXED_PATH = /[\\/]/;

function makeFileSystem(state) {
  const prefix = "FileSystem";
  const known = (p) => Object.prototype.hasOwnProperty.call(state.files, p);
  // Hai API sandbox: ten file tran -> null (khong nem), duong dan -> nem, va khong doc/ghi gi.
  // maxArgs: so tham so hop le cua API do (writeFileString nhan 2: duong dan + noi dung).
  const sandboxed = (name, args, maxArgs) => {
    push(state, prefix + "." + name + "(" + fmtArgs(args) + ")");
    if (args.length > maxArgs) throw new Error("参数太多.");
    if (SANDBOXED_PATH.test(String(args[0]))) throw new Error('path cannot contains "/" or "\\"');
    return null;
  };
  return strict(prefix, {
    state,
    props: {},
    methods: {
      Exists: (...args) => {
        push(state, prefix + ".Exists(" + fmtArgs(args) + ")");
        return known(String(args[0]));
      },
      existsSync: (...args) => {
        push(state, prefix + ".existsSync(" + fmtArgs(args) + ")");
        return known(String(args[0]));
      },
      ReadFile: (...args) => {
        push(state, prefix + ".ReadFile(" + fmtArgs(args) + ")");
        if (args.length > 1) throw new Error("参数太多.");
        const p = String(args[0]);
        if (!known(p)) throw new Error('"Wrong path!"');
        return readFileText(state.files[p]);
      },
      readFileString: (...args) => sandboxed("readFileString", args, 1),
      writeFileString: (...args) => sandboxed("writeFileString", args, 2),
      readAsBinaryString: (...args) => {
        push(state, prefix + ".readAsBinaryString(" + fmtArgs(args) + ")");
        if (args.length > 1) throw new Error("参数太多.");
        const p = String(args[0]);
        if (!known(p)) return null;
        return state.files[p].map((b) => String.fromCharCode(b < 0x80 ? b : 0xFF00 | b)).join("");
      },
      writeAsBinaryString: (...args) => {
        push(state, prefix + ".writeAsBinaryString(" + fmtArgs(args) + ")");
        state.files[String(args[0])] = binaryStringBytes(args[1]);
      },
      tmpdir: () => {
        push(state, prefix + ".tmpdir()");
        return "/tmp";
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeCommandBars(state) {
  const prefix = "CommandBars";
  return strict(prefix, {
    state,
    props: {},
    methods: {
      ExecuteMso: (...args) => {
        push(state, prefix + ".ExecuteMso(" + fmtArgs(args) + ")");
        return true;
      },
      GetPressedMso: (...args) => {
        push(state, prefix + ".GetPressedMso(" + fmtArgs(args) + ")");
        return false;
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function collectValues(arg, out) {
  if (Array.isArray(arg)) {
    for (const x of arg) collectValues(x, out);
    return out;
  }
  if (arg && typeof arg === "object" && LABELS.has(arg)) {
    collectValues(arg.Value2, out);
    return out;
  }
  out.push(arg === undefined ? null : arg);
  return out;
}

function makeWorksheetFunction(state) {
  const prefix = "WorksheetFunction";
  return strict(prefix, {
    state,
    props: {},
    methods: {
      CountA: (...args) => {
        push(state, prefix + ".CountA(" + fmtArgs(args) + ")");
        const values = collectAll(args);
        let n = 0;
        for (const v of values) if (v !== null && v !== undefined) n += 1;
        return n;
      },
      Count: (...args) => {
        push(state, prefix + ".Count(" + fmtArgs(args) + ")");
        const values = collectAll(args);
        let n = 0;
        for (const v of values) if (typeof v === "number" && Number.isFinite(v)) n += 1;
        return n;
      },
      Sum: (...args) => {
        push(state, prefix + ".Sum(" + fmtArgs(args) + ")");
        const values = collectAll(args);
        let total = 0;
        for (const v of values) if (typeof v === "number" && Number.isFinite(v)) total += v;
        return total;
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function collectAll(args) {
  const out = [];
  for (const arg of args) collectValues(arg, out);
  return out;
}

// ---------------------------------------------------------------------------------------------
// Sheet state + makeEtApp
// ---------------------------------------------------------------------------------------------

function newSheet({ name }) {
  return {
    name: name || "Sheet",
    visible: true,
    cells: new Map(),
    styles: new Map(),
    colWidths: new Map(),
    charts: [],
    chartSeq: 0,
    shapes: [],
    formatConditions: [],
  };
}

function applyInitialCell(sheet, addr, value) {
  const box = parseRangeAddr(addr);
  if (!box) throw new Error("dia chi o ban dau khong hop le: " + addr);
  if (value && typeof value === "object" && !Array.isArray(value) && typeof value.formula === "string") {
    setCellFormula(sheet, box.r1, box.c1, value.formula);
    return;
  }
  setCellScalar(sheet, box.r1, box.c1, value);
}

export function makeEtApp(options = {}) {
  const workbookName = options.workbookName || "blank.xlsx";
  const fullPath = options.path || ("/tmp/" + workbookName);
  const sheetDefs = Array.isArray(options.sheets) && options.sheets.length > 0
    ? options.sheets
    : [{ name: "Sheet" }];

  const state = {
    app: "et",
    calls: [],
    files: filesWithBytes(options.files),
    workbooks: [],
    activeWorkbookIndex: 0,
    workbookSeq: 0,
    screenUpdating: true,
    displayAlerts: true,
    enableEvents: true,
  };

  const wb = newWorkbook({ name: workbookName, fullPath });
  for (const def of sheetDefs) {
    const sheet = newSheet({ name: def.name });
    if (def.cells) {
      for (const [addr, value] of Object.entries(def.cells)) applyInitialCell(sheet, addr, value);
    }
    wb.sheets.push(sheet);
  }
  const requested = Math.round(Number(options.active));
  wb.activeSheetIndex = Number.isFinite(requested)
    ? Math.max(0, Math.min(wb.sheets.length - 1, requested))
    : 0;
  state.workbooks.push(wb);

  const props = {
    Name: { get: () => "WPS Spreadsheets" },
    Version: { get: () => "12.0" },
    Build: { get: () => "probe" },
    ActiveWorkbook: {
      get: () => {
        const w = state.workbooks[state.activeWorkbookIndex];
        return w ? makeWorkbook(state, w, "Workbook") : null;
      },
    },
    ActiveSheet: {
      get: () => {
        const w = state.workbooks[state.activeWorkbookIndex];
        if (!w || w.sheets.length === 0 || w.activeSheetIndex < 0) return null;
        return makeWorksheet(state, w, w.sheets[w.activeSheetIndex], "ActiveSheet");
      },
    },
    Sheets: {
      get: () => {
        const w = state.workbooks[state.activeWorkbookIndex];
        return w ? makeSheetsCollection(state, w, "Sheets") : null;
      },
    },
    Worksheets: {
      get: () => {
        const w = state.workbooks[state.activeWorkbookIndex];
        return w ? makeSheetsCollection(state, w, "Worksheets") : null;
      },
    },
    Workbooks: { get: () => makeWorkbooksCollection(state) },
    WorksheetFunction: { get: () => makeWorksheetFunction(state) },
    CommandBars: { get: () => makeCommandBars(state) },
    FileSystem: { get: () => makeFileSystem(state) },
    ScreenUpdating: { get: () => state.screenUpdating, set: (v) => { state.screenUpdating = !!v; } },
    DisplayAlerts: { get: () => state.displayAlerts, set: (v) => { state.displayAlerts = !!v; } },
    EnableEvents: { get: () => state.enableEvents, set: (v) => { state.enableEvents = !!v; } },
  };

  const app = strict("app", { state, props, methods: {}, allowed: APP_OPTIONAL_UNDEFINED });
  APPS.set(app, state);
  return app;
}

// ---------------------------------------------------------------------------------------------
// WPP: text holder dung chung cho shape / o bang / placeholder
// ---------------------------------------------------------------------------------------------

function makeTextHolder(state, prefix, store, getText, setText) {
  const tfPrefix = prefix + ".TextFrame";
  const trPrefix = tfPrefix + ".TextRange";

  const color = strict(trPrefix + ".Font.Color", {
    state,
    props: {
      RGB: {
        get: () => (store.color !== undefined ? store.color : 0),
        set: (v) => { store.color = v; push(state, trPrefix + ".Font.Color.RGB=" + fmt(v)); },
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });

  const font = strict(trPrefix + ".Font", {
    state,
    props: {
      Size: {
        get: () => (store.size !== undefined ? store.size : 18),
        set: (v) => { store.size = v; push(state, trPrefix + ".Font.Size=" + fmt(v)); },
      },
      Bold: {
        get: () => (store.bold !== undefined ? store.bold : 0),
        set: (v) => { store.bold = v; push(state, trPrefix + ".Font.Bold=" + fmt(v)); },
      },
      Italic: {
        get: () => (store.italic !== undefined ? store.italic : 0),
        set: (v) => { store.italic = v; push(state, trPrefix + ".Font.Italic=" + fmt(v)); },
      },
      Color: { get: () => color },
    },
    allowed: ALWAYS_UNDEFINED,
  });

  const paragraphFormat = strict(trPrefix + ".ParagraphFormat", {
    state,
    props: {
      Alignment: {
        get: () => (store.align !== undefined ? store.align : 1),
        set: (v) => { store.align = v; push(state, trPrefix + ".ParagraphFormat.Alignment=" + fmt(v)); },
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });

  const textRange = strict(trPrefix, {
    state,
    props: {
      Text: {
        get: () => getText(),
        set: (v) => {
          push(state, trPrefix + ".Text=" + fmt(v));
          setText(v === null || v === undefined ? "" : String(v));
        },
      },
      Font: { get: () => font },
      ParagraphFormat: { get: () => paragraphFormat },
    },
    allowed: ALWAYS_UNDEFINED,
  });

  const textFrame = strict(tfPrefix, {
    state,
    props: { TextRange: { get: () => textRange } },
    allowed: ALWAYS_UNDEFINED,
  });

  return { TextFrame: textFrame, TextRange: textRange };
}

// ---------------------------------------------------------------------------------------------
// WPP: shape / table
// ---------------------------------------------------------------------------------------------

function makeTable(state, shapeState, prefix) {
  const rows = shapeState.table.rows;
  const cols = shapeState.table.cols;
  const tPrefix = prefix + ".Table";
  const tableStore = {};

  return strict(tPrefix, {
    state,
    props: {
      Rows: { get: () => makeCountCollection(state, rows, null, tPrefix + ".Rows") },
      Columns: { get: () => makeCountCollection(state, cols, null, tPrefix + ".Columns") },
    },
    methods: {
      Cell: (r, c) => {
        const rr = Math.round(Number(r));
        const cc = Math.round(Number(c));
        if (!(rr >= 1 && rr <= rows && cc >= 1 && cc <= cols)) throw new Error("index out of range");
        const cellPrefix = tPrefix + ".Cell(" + rr + "," + cc + ")";
        const holder = makeTextHolder(
          state,
          cellPrefix + ".Shape",
          tableStore,
          () => shapeState.table.cells[rr - 1][cc - 1],
          (text) => { shapeState.table.cells[rr - 1][cc - 1] = text; },
        );
        return strict(cellPrefix, {
          state,
          props: { Shape: { get: () => makeTextHolderProxy(state, cellPrefix + ".Shape", holder) } },
          allowed: ALWAYS_UNDEFINED,
        });
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeShape(state, slide, shapeState, prefix) {
  const holder = makeTextHolder(state, prefix, shapeState, () => shapeState.text, (text) => { shapeState.text = text; });
  return strict(prefix, {
    state,
    props: {
      Name: { get: () => shapeState.name },
      Type: { get: () => shapeState.type },
      Left: { get: () => shapeState.left, set: (v) => { shapeState.left = v; push(state, prefix + ".Left=" + fmt(v)); } },
      Top: { get: () => shapeState.top, set: (v) => { shapeState.top = v; push(state, prefix + ".Top=" + fmt(v)); } },
      Width: { get: () => shapeState.width, set: (v) => { shapeState.width = v; push(state, prefix + ".Width=" + fmt(v)); } },
      Height: { get: () => shapeState.height, set: (v) => { shapeState.height = v; push(state, prefix + ".Height=" + fmt(v)); } },
      TextFrame: { get: () => holder.TextFrame },
      TextRange: { get: () => holder.TextRange },
      Table: {
        get: () => {
          if (shapeState.kind !== "Table") throw new Error("shape khong phai bang: " + prefix);
          return makeTable(state, shapeState, prefix);
        },
      },
    },
    methods: {
      Delete: () => {
        const i = slide.shapes.indexOf(shapeState);
        if (i >= 0) slide.shapes.splice(i, 1);
        push(state, prefix + ".Delete()");
      },
      Select: () => push(state, prefix + ".Select()"),
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeTextHolderProxy(state, prefix, holder) {
  // Proxy "Shape" toi gian chi de lo ra holder.TextFrame (dung cho o bang / placeholder).
  return strict(prefix, {
    state,
    props: { TextFrame: { get: () => holder.TextFrame } },
    allowed: ALWAYS_UNDEFINED,
  });
}

// ---------------------------------------------------------------------------------------------
// WPP: Shapes collection
// ---------------------------------------------------------------------------------------------

const WPP_TITLE_LAYOUTS = { 1: true, 2: true, 11: true };

function makeShapesWpp(state, slideState) {
  const prefix = "Shapes";

  function addShape(kind, name, type, left, top, width, height, logLine) {
    push(state, logLine);
    const shapeState = { kind, name, type, left, top, width, height, text: "", table: null };
    slideState.shapes.push(shapeState);
    return makeShape(state, slideState, shapeState, prefix + ".Item(" + slideState.shapes.length + ")");
  }

  function ensureTitle() {
    if (!slideState.titleShape) {
      slideState.titleShape = { kind: "Title", name: "Title 1", type: 13, left: 0, top: 0, width: 0, height: 0, text: "", table: null };
    }
    return slideState.titleShape;
  }

  return strict(prefix, {
    state,
    props: {
      Count: { get: () => slideState.shapes.length },
      Title: {
        get: () => {
          if (!WPP_TITLE_LAYOUTS[slideState.layout]) {
            throw new Error("layout " + slideState.layout + " khong co title: " + prefix + ".Title");
          }
          return makeShape(state, slideState, ensureTitle(), prefix + ".Title");
        },
      },
      Placeholders: {
        get: () => strict(prefix + ".Placeholders", {
          state,
          props: { Count: { get: () => (WPP_TITLE_LAYOUTS[slideState.layout] ? 1 : 0) } },
          methods: {
            Item: (n) => {
              if (!WPP_TITLE_LAYOUTS[slideState.layout] || Math.round(Number(n)) !== 1) throw new Error("index out of range");
              return makeShape(state, slideState, ensureTitle(), prefix + ".Placeholders.Item(1)");
            },
          },
          allowed: ALWAYS_UNDEFINED,
        }),
      },
    },
    methods: {
      Item: (n) => {
        const i = Math.round(Number(n));
        if (!(i >= 1 && i <= slideState.shapes.length)) throw new Error("index out of range");
        return makeShape(state, slideState, slideState.shapes[i - 1], prefix + ".Item(" + i + ")");
      },
      AddTextbox: (...args) => {
        slideState.shapeSeq += 1;
        return addShape("TextBox", "TextBox " + slideState.shapeSeq, 17, args[1], args[2], args[3], args[4],
          prefix + ".AddTextbox(" + fmtArgs(args) + ")");
      },
      AddTable: (...args) => {
        const rows = Math.round(Number(args[0]));
        const cols = Math.round(Number(args[1]));
        slideState.shapeSeq += 1;
        const shape = addShape("Table", "Table " + slideState.shapeSeq, 19, args[2], args[3], args[4], args[5],
          prefix + ".AddTable(" + fmtArgs(args) + ")");
        const shapeState = slideState.shapes[slideState.shapes.length - 1];
        const cells = [];
        for (let r = 0; r < rows; r++) {
          const row = [];
          for (let c = 0; c < cols; c++) row.push("");
          cells.push(row);
        }
        shapeState.table = { rows, cols, cells };
        return shape;
      },
      AddPicture: (...args) => {
        const path = args[0];
        const image = state.images[path];
        if (!image) throw new Error("image file not found");
        // Kieu Excel: Width/Height bo trong (hoac -1/0) = dung kich thuoc goc cua anh.
        const givenW = args[5];
        const givenH = args[6];
        const auto = (v) => v === undefined || v === null || v === 0 || v === -1;
        const width = auto(givenW) ? image.width : givenW;
        const height = auto(givenH) ? image.height : givenH;
        slideState.shapeSeq += 1;
        return addShape("Picture", "Picture " + slideState.shapeSeq, 13, args[3], args[4], width, height,
          prefix + ".AddPicture(" + fmtArgs(args) + ")");
      },
      AddShape: (...args) => {
        slideState.shapeSeq += 1;
        return addShape("AutoShape", "AutoShape " + slideState.shapeSeq, args[0], args[1], args[2], args[3], args[4],
          prefix + ".AddShape(" + fmtArgs(args) + ")");
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

// ---------------------------------------------------------------------------------------------
// WPP: NotesPage
// ---------------------------------------------------------------------------------------------

function makeNotesPage(state, pres, slideState, slidePrefix) {
  const prefix = slidePrefix + ".NotesPage";
  const shapesPrefix = prefix + ".Shapes";

  function holderProxy(index) {
    const holderPrefix = shapesPrefix + ".Placeholders.Item(" + index + ")";
    const store = {};
    const isSlideNumber = index === 1;
    const holder = makeTextHolder(
      state,
      holderPrefix,
      store,
      () => {
        if (isSlideNumber) {
          return slideState.notesNum !== null ? slideState.notesNum : String(pres.slides.indexOf(slideState) + 1);
        }
        return slideState.notes;
      },
      (text) => {
        if (isSlideNumber) slideState.notesNum = text;
        else slideState.notes = text;
      },
    );
    return makeTextHolderProxy(state, holderPrefix, holder);
  }

  const placeholders = strict(shapesPrefix + ".Placeholders", {
    state,
    props: { Count: { get: () => 2 } },
    methods: {
      Item: (n) => {
        const i = Math.round(Number(n));
        if (!(i >= 1 && i <= 2)) throw new Error("index out of range");
        return holderProxy(i);
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });

  const shapes = strict(shapesPrefix, {
    state,
    props: { Placeholders: { get: () => placeholders }, Count: { get: () => 2 } },
    methods: {},
    allowed: ALWAYS_UNDEFINED,
  });

  return strict(prefix, {
    state,
    props: { Shapes: { get: () => shapes } },
    allowed: ALWAYS_UNDEFINED,
  });
}

// ---------------------------------------------------------------------------------------------
// WPP: Slide / Slides
// ---------------------------------------------------------------------------------------------

function makeSlide(state, pres, slideState, labelPath) {
  const prefix = labelPath;
  return strict(prefix, {
    state,
    props: {
      Name: { get: () => slideState.name },
      SlideIndex: { get: () => pres.slides.indexOf(slideState) + 1 },
      Layout: { get: () => slideState.layout },
      Shapes: { get: () => makeShapesWpp(state, slideState) },
      NotesPage: { get: () => makeNotesPage(state, pres, slideState, prefix) },
    },
    methods: {
      Delete: () => {
        const i = pres.slides.indexOf(slideState);
        if (i >= 0) pres.slides.splice(i, 1);
        push(state, prefix + ".Delete()");
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeSlides(state, pres) {
  const prefix = "Slides";
  return strict(prefix, {
    state,
    props: { Count: { get: () => pres.slides.length } },
    methods: {
      Item: (n) => {
        const i = Math.round(Number(n));
        if (!(i >= 1 && i <= pres.slides.length)) throw new Error("index out of range");
        return makeSlide(state, pres, pres.slides[i - 1], "Slides.Item(" + i + ")");
      },
      Add: (...args) => {
        const rawIndex = args.length >= 1 && args[0] !== undefined && args[0] !== null
          ? Math.round(Number(args[0])) : pres.slides.length + 1;
        const layout = args.length >= 2 && args[1] !== undefined && args[1] !== null
          ? Number(args[1]) : 12;
        const at = Math.max(1, Math.min(pres.slides.length + 1, rawIndex));
        pres.slideSeq += 1;
        const slideState = { name: "Slide " + pres.slideSeq, layout, shapes: [], shapeSeq: 0, notes: "", notesNum: null, titleShape: null };
        pres.slides.splice(at - 1, 0, slideState);
        push(state, prefix + ".Add(" + fmtArgs(args) + ")");
        return makeSlide(state, pres, slideState, "Slides.Item(" + at + ")");
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

// ---------------------------------------------------------------------------------------------
// WPP: Presentation / Presentations
// ---------------------------------------------------------------------------------------------

function makePresentation(state, pres) {
  const prefix = "Presentation";
  const logPrefix = "";   // khop vi du "SaveAs(/tmp/x.pdf,32)" trong brief: method wpp ghi log tran

  const props = {
    Name: { get: () => pres.name },
    FullName: { get: () => pres.fullPath },
    Path: { get: () => splitPath(pres.fullPath).dir },
    Saved: { get: () => pres.saved, set: (v) => { pres.saved = !!v; push(state, "Presentation.Saved=" + fmt(v)); } },
    Slides: { get: () => makeSlides(state, pres) },
    PageSetup: {
      get: () => strict("PageSetup", {
        state,
        props: {
          SlideWidth: { get: () => pres.pageSetup.width, set: (v) => { pres.pageSetup.width = v; push(state, "PageSetup.SlideWidth=" + fmt(v)); } },
          SlideHeight: { get: () => pres.pageSetup.height, set: (v) => { pres.pageSetup.height = v; push(state, "PageSetup.SlideHeight=" + fmt(v)); } },
        },
        allowed: ALWAYS_UNDEFINED,
      }),
    },
    SlideMaster: {
      get: () => strict("SlideMaster", {
        state,
        props: {
          CustomLayouts: {
            get: () => strict("CustomLayouts", { state, props: { Count: { get: () => 12 } }, allowed: ALWAYS_UNDEFINED }),
          },
        },
        allowed: ALWAYS_UNDEFINED,
      }),
    },
  };

  const methods = {
    Save: (...args) => { pres.saved = true; push(state, member(logPrefix, "Save") + "(" + fmtArgs(args) + ")"); },
    SaveAs: (...args) => {
      if (args.length >= 1 && args[0] !== undefined && args[0] !== null) {
        const full = String(args[0]);
        pres.fullPath = full;
        pres.name = splitPath(full).base;
      }
      pres.saved = true;
      push(state, member(logPrefix, "SaveAs") + "(" + fmtArgs(args) + ")");
    },
    Close: (...args) => {
      const i = state.presentations.indexOf(pres);
      if (i >= 0) state.presentations.splice(i, 1);
      if (state.activePresentationIndex >= state.presentations.length) {
        state.activePresentationIndex = state.presentations.length - 1;
      }
      if (state.activePresentationIndex < 0) state.activePresentationIndex = 0;
      pres.closed = true;
      push(state, member(logPrefix, "Close") + "(" + fmtArgs(args) + ")");
    },
    // WPS Linux do duoc: KHONG tao file nao ca. Chi ghi log.
    ExportAsFixedFormat: (...args) => push(state, member(logPrefix, "ExportAsFixedFormat") + "(" + fmtArgs(args) + ")"),
  };

  return strict(prefix, { state, props, methods, allowed: ALWAYS_UNDEFINED });
}

function newPresentation({ name, fullPath }) {
  return {
    name: name || "Presentation1",
    fullPath: fullPath || ("/tmp/" + (name || "Presentation1")),
    saved: true,
    closed: false,
    slides: [],
    slideSeq: 0,
    pageSetup: { width: 720, height: 540 },
  };
}

function makePresentations(state) {
  const prefix = "Presentations";
  return strict(prefix, {
    state,
    props: { Count: { get: () => state.presentations.length } },
    methods: {
      Item: (n) => {
        const i = Math.round(Number(n));
        if (!(i >= 1 && i <= state.presentations.length)) throw new Error("index out of range");
        return makePresentation(state, state.presentations[i - 1]);
      },
      Add: (...args) => {
        state.presSeq += 1;
        const name = "Presentation" + state.presSeq;
        const pres = newPresentation({ name, fullPath: "/tmp/" + name });
        state.presentations.push(pres);
        state.activePresentationIndex = state.presentations.length - 1;
        push(state, prefix + ".Add(" + fmtArgs(args) + ")");
        return makePresentation(state, pres);
      },
      Open: (...args) => {
        const full = args.length > 0 && args[0] !== undefined ? String(args[0]) : "/tmp/blank.pptx";
        const pres = newPresentation({ name: splitPath(full).base, fullPath: full });
        state.presentations.push(pres);
        state.activePresentationIndex = state.presentations.length - 1;
        push(state, prefix + ".Open(" + fmtArgs(args) + ")");
        return makePresentation(state, pres);
      },
    },
    allowed: ALWAYS_UNDEFINED,
  });
}

function makeActiveWindow(state) {
  const prefix = "ActiveWindow";
  const view = strict(prefix + ".View", {
    state,
    props: { Type: { get: () => 9 } },
    methods: {
      GotoSlide: (...args) => push(state, prefix + ".View.GotoSlide(" + fmtArgs(args) + ")"),
    },
    allowed: ALWAYS_UNDEFINED,
  });
  return strict(prefix, {
    state,
    props: {
      ViewType: { get: () => 9 },
      View: { get: () => view },
      Presentation: {
        get: () => (state.presentations.length ? makePresentation(state, state.presentations[state.activePresentationIndex]) : null),
      },
    },
    methods: {},
    allowed: ALWAYS_UNDEFINED,
  });
}

// ---------------------------------------------------------------------------------------------
// makeWppApp
// ---------------------------------------------------------------------------------------------

export function makeWppApp(options = {}) {
  const presentationName = options.presentationName || "blank.pptx";
  const fullPath = options.path || ("/tmp/" + presentationName);

  const state = {
    app: "wpp",
    calls: [],
    files: filesWithBytes(options.files),
    images: options.images || {},
    presentations: [],
    activePresentationIndex: 0,
    presSeq: 0,
    screenUpdating: true,
    displayAlerts: true,
    enableEvents: true,
  };

  const pres = newPresentation({ name: presentationName, fullPath });
  pres.pageSetup.width = options.slideWidth !== undefined ? options.slideWidth : 720;
  pres.pageSetup.height = options.slideHeight !== undefined ? options.slideHeight : 540;
  const slideCount = options.slides !== undefined ? Math.max(0, Math.round(Number(options.slides))) : 0;
  for (let i = 0; i < slideCount; i++) {
    pres.slideSeq += 1;
    pres.slides.push({ name: "Slide " + pres.slideSeq, layout: 12, shapes: [], shapeSeq: 0, notes: "", notesNum: null, titleShape: null });
  }
  state.presentations.push(pres);

  const props = {
    Name: { get: () => "WPS Presentation" },
    Version: { get: () => "12.0" },
    Build: { get: () => "probe" },
    ActivePresentation: {
      get: () => {
        const p = state.presentations[state.activePresentationIndex];
        return p ? makePresentation(state, p) : null;
      },
    },
    Presentations: { get: () => makePresentations(state) },
    ActiveWindow: { get: () => makeActiveWindow(state) },
    CommandBars: { get: () => makeCommandBars(state) },
    FileSystem: { get: () => makeFileSystem(state) },
    ScreenUpdating: { get: () => state.screenUpdating, set: (v) => { state.screenUpdating = !!v; } },
    DisplayAlerts: { get: () => state.displayAlerts, set: (v) => { state.displayAlerts = !!v; } },
    EnableEvents: { get: () => state.enableEvents, set: (v) => { state.enableEvents = !!v; } },
  };

  const app = strict("app", { state, props, methods: {}, allowed: APP_OPTIONAL_UNDEFINED });
  APPS.set(app, state);
  return app;
}

// ---------------------------------------------------------------------------------------------
// Writer (van ban)
// ---------------------------------------------------------------------------------------------
//
// Mo hinh chu: tai lieu la mot chuoi `text`; ky tu chiem nua khoang [0, text.length) va
// doc.Content.End === text.length. Day dung la quy uoc `commands.js` dung de tinh vung khoa:
// prefix = Range(0, start).Text, suffix = Range(end, docEnd).Text, suffixLength = docEnd - end.
// (WPS that tra Range.Text la CHUOI, khong phai ham - do o cmdSelection/cmdGetText.)
//
// `tables` la danh sach { start, end } - Range cua tung bang, y nhu WPS tra ve qua
// doc.Tables.Item(i).Range. Bang NAM TRON trong vung bi thay bang chu thi mat (dung nhu Word/WPS
// xoa bang khi vung chon bi de bang chu thuong) - do la truong hop ma luoi an toan cua khoa vung
// phai bat. Bang nam MOT PHAN trong vung (con tro trong o) thi song, chi dich vi tri theo.
//
// Hoan tac: WPS Writer CO UndoRecord (khac ET/WPP - xem REFERENCE.md). `StartCustomRecord` mo mot
// nhom; ca nhom thanh MOT buoc; `doc.Undo(1)` tra ve nguyen trang thai truoc nhom. Nho vay luoi an
// toan cua khoa vung (goi doc.Undo(1) sau khi lenh chay) kiem duoc that, khong chi "khong nem loi".
//
// options:
//   text, documentName, path, tables: trang thai dau
//   selection: { start, end }        vi tri con tro/vung chon cua nguoi dung
//   selectionLockable: false         mo phong ban WPS khong keo duoc con tro: Selection khong co
//                                    SetRange va Start/End chi doc. Dung de kiem LUOI AN TOAN (con
//                                    tro ngoai vung khoa -> lenh chay o cho khac -> phai hoan tac).
//   selectionPositions: false        ban WPS khong tra Selection.Start/End (khong co vung de khoa)
//   range: false                     ban WPS khong co doc.Range (khong doc duoc phan truoc/sau ->
//                                    khong the khoa: thieu luoi an toan thi khong khoa)
export function makeWriterApp(options = {}) {
  const documentName = options.documentName || "blank.docx";
  const fullPath = options.path || ("/tmp/" + documentName);
  const lockable = options.selectionLockable !== false;
  const hasPositions = options.selectionPositions !== false;
  const hasRange = options.range !== false;

  const state = {
    app: "wps",
    calls: [],
    text: options.text !== undefined ? String(options.text) : "",
    tables: (options.tables || []).map((t) => ({ start: t.start, end: t.end })),
    selection: {
      start: options.selection ? options.selection.start : 0,
      end: options.selection ? options.selection.end : 0,
    },
    style: "",
    undoStack: [],
    group: null,
  };

  const snapshot = () => ({ text: state.text, tables: state.tables.map((t) => ({ start: t.start, end: t.end })) });
  const same = (a, b) => a.text === b.text && JSON.stringify(a.tables) === JSON.stringify(b.tables);

  // Thay doi ngoai mot nhom UndoRecord thi tu no la mot buoc hoan tac; trong nhom thi de nhom lo.
  function beforeChange() {
    if (state.group === null) { state.undoStack.push(snapshot()); }
  }

  // Thay [from, to) bang `replacement`, roi dich vi tri cac bang con lai.
  function applyEdit(from, to, replacement) {
    state.text = state.text.slice(0, from) + replacement + state.text.slice(to);
    const delta = replacement.length - (to - from);
    const next = [];
    for (const t of state.tables) {
      if (t.start >= from && t.end <= to) { continue; }   // bang nam tron trong vung bi thay -> mat
      next.push({
        start: t.start >= to ? t.start + delta : t.start,
        end: t.end >= to ? t.end + delta : t.end,
      });
    }
    state.tables = next;
  }

  function makeRange(from, to, label) {
    const f = Number(from);
    const t = Number(to);
    if (!Number.isFinite(f) || !Number.isFinite(t)) { throw new Error("Range: tham so khong phai so"); }
    if (f < 0 || t < f || t > state.text.length) {
      throw new Error("Range(" + f + "," + t + ") ngoai tai lieu (dai " + state.text.length + ")");
    }
    const path = label + "(" + f + "," + t + ")";
    return strict(path, {
      state,
      props: {
        Start: { get: () => f },
        End: { get: () => t },
        Text: { get: () => state.text.slice(f, t) },
        Find: { get: () => makeFind(f, t, path) },
      },
      methods: {
        // Chen ngay SAU ky tu cuoi cua Range. Voi doc.Content (Range(0, docEnd)) tuc la chen o
        // cuoi tai lieu - dung duong ma `writer.appendText` di.
        InsertAfter: (s) => {
          const value = String(s);
          push(state, path + ".InsertAfter(" + fmt(value) + ")");
          beforeChange();
          applyEdit(t, t, value);
        },
      },
    });
  }

  // Find.Execute(FindText, ..., Forward, Wrap, Format, ReplaceWith, Replace=2): thay trong [f, t).
  function makeFind(from, to, path) {
    return strict(path + ".Find", {
      state,
      props: { Found: { get: () => false } },
      methods: {
        Execute: function (findText) {
          const args = Array.prototype.slice.call(arguments);
          const replaceWith = args.length > 9 ? args[9] : "";
          const replaceMode = args.length > 10 ? Number(args[10]) : 0;
          push(state, "Find.Execute(" + fmtArgs(args) + ")");
          const needle = String(findText);
          if (needle === "" || replaceMode !== 2) { return false; }
          const hay = state.text.slice(from, to);
          const lower = hay.toLowerCase();
          const target = needle.toLowerCase();
          const parts = [];
          let pos = 0, idx = lower.indexOf(target, pos), hits = 0;
          while (idx >= 0) {
            parts.push(hay.slice(pos, idx));
            parts.push(String(replaceWith));
            pos = idx + needle.length;
            hits++;
            idx = lower.indexOf(target, pos);
          }
          if (hits === 0) { return false; }
          parts.push(hay.slice(pos));
          beforeChange();
          applyEdit(from, to, parts.join(""));
          return true;
        },
      },
    });
  }

  function makeTables() {
    return strict("doc.Tables", {
      state,
      props: { Count: { get: () => state.tables.length } },
      methods: {
        Item: (i) => {
          const idx = Number(i);
          if (!Number.isInteger(idx) || idx < 1 || idx > state.tables.length) { throw new Error("index out of range"); }
          const t = state.tables[idx - 1];
          const path = "doc.Tables.Item(" + idx + ")";
          return strict(path, {
            state,
            props: { Range: { get: () => makeRange(t.start, t.end, path + ".Range") } },
            methods: {},
          });
        },
      },
    });
  }

  function makeSelection() {
    const path = "app.Selection";
    const props = {
      Text: { get: () => state.text.slice(state.selection.start, state.selection.end) },
      Style: { get: () => state.style, set: (v) => { state.style = String(v); } },
    };
    if (hasPositions) {
      if (lockable) {
        props.Start = { get: () => state.selection.start, set: (v) => { state.selection.start = Number(v); } };
        props.End = { get: () => state.selection.end, set: (v) => { state.selection.end = Number(v); } };
      } else {
        props.Start = { get: () => state.selection.start };
        props.End = { get: () => state.selection.end };
      }
    }
    const methods = {
      TypeText: (t) => {
        const s = String(t);
        push(state, "Selection.TypeText(" + fmt(s) + ")");
        beforeChange();
        const from = state.selection.start;
        applyEdit(from, state.selection.end, s);
        state.selection = { start: from + s.length, end: from + s.length };
      },
      TypeParagraph: () => {
        push(state, "Selection.TypeParagraph()");
        beforeChange();
        const from = state.selection.start;
        applyEdit(from, state.selection.end, "\n");
        state.selection = { start: from + 1, end: from + 1 };
      },
      set_Style: (name) => { push(state, "Selection.set_Style(" + fmt(name) + ")"); state.style = String(name); },
    };
    if (lockable) {
      methods.SetRange = (a, b) => {
        push(state, "Selection.SetRange(" + fmt(a) + "," + fmt(b) + ")");
        const from = Number(a), to = Number(b);
        if (!Number.isFinite(from) || !Number.isFinite(to) || from < 0 || to < from || to > state.text.length) {
          throw new Error("SetRange(" + from + "," + to + ") ngoai tai lieu");
        }
        state.selection = { start: from, end: to };
      };
    }
    return strict(path, { state, props, methods });
  }

  function makeUndoRecord() {
    return strict("app.UndoRecord", {
      state,
      methods: {
        StartCustomRecord: (title) => {
          push(state, "UndoRecord.StartCustomRecord(" + fmt(title) + ")");
          state.group = snapshot();
        },
        EndCustomRecord: () => {
          push(state, "UndoRecord.EndCustomRecord()");
          // Nhom khong doi gi thi khong tao buoc hoan tac (dung nhu WPS: lenh khong doi gi thi
          // Ctrl+Z khong tra ve no).
          if (state.group !== null && !same(state.group, snapshot())) { state.undoStack.push(state.group); }
          state.group = null;
        },
      },
    });
  }

  function makeDocument() {
    const methods = {
      Undo: (n) => {
        const steps = Number(n) || 1;
        push(state, "doc.Undo(" + steps + ")");
        let done = 0;
        for (let i = 0; i < steps; i++) {
          const prev = state.undoStack.pop();
          if (!prev) { break; }
          state.text = prev.text;
          state.tables = prev.tables.map((t) => ({ start: t.start, end: t.end }));
          done++;
        }
        return done;
      },
    };
    if (hasRange) { methods.Range = (from, to) => makeRange(from, to, "doc.Range"); }
    return strict("app.ActiveDocument", {
      state,
      props: {
        Name: { get: () => documentName },
        FullName: { get: () => fullPath },
        Saved: { get: () => true },
        Content: { get: () => makeRange(0, state.text.length, "doc.Content") },
        Tables: { get: () => makeTables() },
      },
      methods,
    });
  }

  const props = {
    Name: { get: () => "WPS Writer" },
    Version: { get: () => "12.0" },
    Build: { get: () => "fake" },
    ActiveDocument: { get: () => makeDocument() },
    Selection: { get: () => makeSelection() },
    UndoRecord: { get: () => makeUndoRecord() },
  };

  const app = strict("app", { state, props, methods: {} });
  APPS.set(app, state);
  return app;
}

// Nguoi dung doi con tro/vung chon (bam sang cho khac, keo chuot...). Can cho cac phep kiem "khoa
// vung": luc khoa thi vung chon CHINH LA vung khoa, nen muon con tro ra ngoai thi phai doi SAU khi khoa.
export function writerSelect(app, start, end) {
  const state = APPS.get(app);
  if (!state || state.app !== "wps") throw new Error("writerSelect: doi tuong khong phai app Writer gia");
  const from = Number(start);
  const to = end === undefined ? from : Number(end);
  if (!Number.isFinite(from) || !Number.isFinite(to) || from < 0 || to < from || to > state.text.length) {
    throw new Error("writerSelect(" + start + "," + end + ") ngoai tai lieu (dai " + state.text.length + ")");
  }
  state.selection = { start: from, end: to };
}

// Trang thai hien tai cua tai lieu gia (de kiem SAU khi lenh chay, khong phai doan tu ket qua tra ve).
export function writerStateOf(app) {
  const state = APPS.get(app);
  if (!state || state.app !== "wps") throw new Error("writerStateOf: doi tuong khong phai app Writer gia");
  return {
    text: state.text,
    tables: state.tables.map((t) => ({ start: t.start, end: t.end })),
    selection: { start: state.selection.start, end: state.selection.end },
    style: state.style,
  };
}

// ---------------------------------------------------------------------------------------------
// callsOf
// ---------------------------------------------------------------------------------------------

export function callsOf(app) {
  const state = APPS.get(app);
  if (!state) throw new Error("callsOf: doi tuong khong phai app gia cua addin_fake");
  return state.calls.slice();
}
