// Fake WPS Writer cho nhom lenh SOAN VAN BAN cua add-in WPS (writer.writeContent, writer.insertLayoutTable,
// writer.insertCaption) - Smart Create va doc_from_brief. Tach khoi addin_fake.mjs (chu tho + vi tri) va
// addin_fake_structure.mjs (style tung doan) vi o day can ca hai CUNG LUC: doan voi style/can le/phong chu/danh sach,
// bang voi tung o nhieu dong, truong SEQ, va VI TRI ky tu dung nhu WPS de doc.Range(a, b) chi dung doan.
//
// File nay KHONG phai test. Hanh vi duoc mo phong da DO tren WPS 12.1.2.29182 that (01/11/2026, REPL Xvfb):
//   - Vi tri: moi doan = chu + 1 (dau doan); o bang = chu cac doan trong o + 1 (dau o, Text doc ra "...\r\u0007");
//     moi hang them 1 (dau cuoi hang). Bai luon ket thuc bang mot doan thuong.
//   - doc.Content.InsertAfter(s) chen TRUOC dau doan cuoi ("\r" trong s tach doan); Content.End tang dung len(s).
//   - Tables.Add(Selection.Range, r, c) voi con tro o cuoi bai: doan cuoi CO chu thi bang nam SAU doan do; sau bang
//     luon co mot doan trong. Chen bang ngay sau mot bang (con tro o doan trong sau bang) thi WPS GOP vao bang truoc
//     (Tables.Count khong tang, doi tuong tra ve la bang cu) - muon hai bang rieng phai co doan ngan giua.
//   - Table.Borders.Enable = 0, Table.Title (ghi/doc), PreferredWidthType = 2 + PreferredWidth, Columns.Item(c)
//     .PreferredWidthType/PreferredWidth, Cell(r, c).Range.Text = "a\rb" (2 doan), Cell.Range.Paragraphs.Item(i)
//     .Alignment/.Range.Font.*, doc.Range(0, end).Tables.Count deu chay.
//   - app.ListGalleries.Item(1|2).ListTemplates.Item(1) + Range.ListFormat.ApplyListTemplate(t, false): ListType 2
//     (gach dau dong) / 3 (so). Doan moi sau danh sach THUA KE danh sach; ListFormat.RemoveNumbers() go ra.
//   - Selection.Style = "Heading 1" roi TypeParagraph(): doan sau la "Normal". Selection.InsertBreak(7): sang trang moi.
//   - Paragraph.Style = -35 (Caption), KeepWithNext = -1, Range.MoveEnd(1, -1), doc.Fields.Add(range, -1,
//     "SEQ Bang \\* ARABIC", false) + Update() -> Result.Text la so thu tu; Paragraphs.Last dung duoc.
//   - doc.Styles.Item(-1 | -2..-10 | -35 | ten) tra style (NameLocal); ten la -> null.
//
// Moi doi tuong "strict": doc thuoc tinh/ham khong mo phong thi NEM loi - handler goi API chua do tren WPS that thi test do.

const STYLE_IDS = {
  "-1": "Normal", "-2": "Heading 1", "-3": "Heading 2", "-4": "Heading 3", "-5": "Heading 4", "-6": "Heading 5",
  "-7": "Heading 6", "-8": "Heading 7", "-9": "Heading 8", "-10": "Heading 9", "-35": "Caption", "-49": "List Bullet",
  "-50": "List Number", "-63": "Title", "-75": "Subtitle", "-181": "Quote",
};
const STYLE_NAMES = new Set(Object.values(STYLE_IDS));
const APPS = new WeakMap();

function strict(path, props, methods = {}) {
  return new Proxy({}, {
    get(_t, key) {
      if (typeof key === "symbol") { return key === Symbol.toPrimitive ? () => path : undefined; }
      if (key === "then" || key === "toJSON" || key === "inspect" || key === "constructor") { return undefined; }
      if (key === "toString") { return () => "[object Object]"; }
      if (key === "valueOf") { return () => path; }
      if (Object.prototype.hasOwnProperty.call(props, key)) { return props[key].get ? props[key].get() : undefined; }
      if (Object.prototype.hasOwnProperty.call(methods, key)) { return methods[key]; }
      throw new Error("fake: " + path + "." + String(key) + " chua mo phong (chua do tren WPS that)");
    },
    set(_t, key, value) {
      if (Object.prototype.hasOwnProperty.call(props, key) && props[key].set) { props[key].set(value); return true; }
      throw new Error("fake: gan " + path + "." + String(key) + " chua mo phong");
    },
  });
}

function styleName(value) {
  if (value && typeof value === "object") { return String(value.NameLocal); }
  if (typeof value === "number" || /^-\d+$/.test(String(value))) {
    const name = STYLE_IDS[String(value)];
    if (!name) { throw new Error("fake: style " + value + " chua mo phong"); }
    return name;
  }
  // WPS: ten la -> doan am tham thanh Normal
  return STYLE_NAMES.has(String(value)) ? String(value) : "Normal";
}

function para(text, from) {
  const base = from || {};
  return {
    type: "p", text, style: base.style || "Normal", align: base.align === undefined ? 0 : base.align, list: base.list || 0,
    keepWithNext: 0, spaceBefore: base.spaceBefore, spaceAfter: base.spaceAfter, lineRule: base.lineRule, font: Object.assign({ bold: 0, italic: 0, underline: 0, size: 12, name: "Calibri", color: -16777216 },
      base.font || {}),
  };
}

// ---- vi tri ----
// spans(): danh sach phan tu {kind: "p", p, start, end, owner} cho moi doan (ke ca doan trong o bang) va {kind: "row"}.
function layout(state) {
  const spans = [];
  let pos = 0;
  state.items.forEach((item, index) => {
    if (item.type === "p") {
      spans.push({ kind: "p", p: item, start: pos, end: pos + item.text.length + 1, item: index });
      pos += item.text.length + 1;
      return;
    }
    item.start = pos;
    item.cells.forEach((row, r) => {
      row.forEach((cell, c) => {
        cell.forEach((p) => {
          spans.push({ kind: "p", p, start: pos, end: pos + p.text.length + 1, item: index, table: item, cell: [r + 1, c + 1] });
          pos += p.text.length + 1;
        });
      });
      pos += 1; // dau cuoi hang
    });
    item.end = pos;
  });
  return { spans, end: pos };
}

function paragraphsIn(state, start, end) {
  const { spans } = layout(state);
  return spans.filter((s) => s.kind === "p" && (start === end ? s.start <= start && start < s.end : s.start < end && s.end > start));
}

function lastParagraph(state) { return state.items[state.items.length - 1]; }

function makeFont(targets, path) {
  const set = (key) => (v) => { for (const p of targets()) { p.font[key] = v; } };
  const get = (key) => () => { const list = targets(); return list.length ? list[0].font[key] : 9999999; };
  return strict(path, {
    Bold: { get: get("bold"), set: set("bold") }, Italic: { get: get("italic"), set: set("italic") },
    Underline: { get: get("underline"), set: set("underline") }, Size: { get: get("size"), set: set("size") },
    Name: { get: get("name"), set: set("name") }, Color: { get: get("color"), set: set("color") },
  });
}

function makeParagraphFormat(targets, path, state) {
  const set = (key) => (v) => { for (const p of targets()) { p[key] = v; } };
  const get = (key) => () => { const list = targets(); return list.length ? list[0][key] : 9999999; };
  return strict(path, {
    Alignment: { get: get("align"), set: set("align") },
    SpaceBefore: { get: get("spaceBefore"), set: set("spaceBefore") },
    SpaceAfter: { get: get("spaceAfter"), set: set("spaceAfter") },
    LineSpacingRule: { get: get("lineRule"), set: set("lineRule") },
    KeepWithNext: { get: get("keepWithNext"), set: set("keepWithNext") },
  });
}

function makeRange(state, startIn, endIn, path) {
  const r = { start: startIn, end: endIn };
  const targets = () => paragraphsIn(state, r.start, r.end).map((s) => s.p);
  return strict(path, {
    Start: { get: () => r.start },
    End: { get: () => r.end },
    Text: {
      get: () => {
        const { spans } = layout(state);
        let out = "";
        for (const s of spans) {
          if (s.kind !== "p" || s.end <= r.start || s.start >= r.end) { continue; }
          const full = s.p.text + "\r";
          out += full.slice(Math.max(0, r.start - s.start), Math.min(full.length, r.end - s.start));
        }
        return out;
      },
      set: (v) => {
        // Chi mo phong: ghi lai phan chu cua MOT doan (sau MoveEnd(1,-1)) - dung cho chu thich.
        const hits = paragraphsIn(state, r.start, r.end);
        if (hits.length !== 1 || r.start !== hits[0].start || r.end !== hits[0].end - 1) {
          throw new Error("fake: Range.Text = ... chi mo phong tren dung phan chu cua mot doan");
        }
        if (String(v).indexOf("\r") >= 0) { throw new Error("fake: Range.Text co \\r chua mo phong"); }
        hits[0].p.text = String(v);
        r.end = r.start + String(v).length;
      },
    },
    Font: { get: () => makeFont(targets, path + ".Font") },
    ParagraphFormat: { get: () => makeParagraphFormat(targets, path + ".ParagraphFormat", state) },
    ListFormat: { get: () => strict(path + ".ListFormat", {
      ListType: { get: () => (targets()[0] || {}).list || 0 },
    }, {
      ApplyListTemplate: (template, continuePrevious) => {
        if (!template || !template.__gallery) { throw new Error("fake: ApplyListTemplate can ListTemplate cua ListGalleries"); }
        if (continuePrevious !== false) { throw new Error("fake: chi mo phong ApplyListTemplate(t, false)"); }
        for (const p of targets()) { p.list = template.__gallery === 1 ? 2 : 3; }
      },
      RemoveNumbers: () => { for (const p of targets()) { p.list = 0; } },
    }) },
    Tables: { get: () => {
      const tables = state.items.filter((it) => it.type === "t" && (layout(state), it.start < r.end && it.end > r.start));
      return strict(path + ".Tables", { Count: { get: () => tables.length } }, {
        Item: (n) => makeTable(state, tables[Number(n) - 1], path + ".Tables.Item(" + n + ")"),
      });
    } },
    Paragraphs: { get: () => makeParagraphs(state, () => targets(), path + ".Paragraphs") },
  }, {
    Collapse: (dir) => { if (Number(dir) === 0) { r.start = r.end; } else { r.end = r.start; } },
    MoveEnd: (unit, count) => {
      if (Number(unit) !== 1) { throw new Error("fake: MoveEnd chi mo phong don vi ky tu (1)"); }
      r.end += Number(count);
    },
    Select: () => { state.cursor = r.start; },
    Information: (what) => {
      const n = Number(what);
      if (n === 12) { const hit = paragraphsIn(state, r.start, r.start)[0]; return !!(hit && hit.table); }
      if (n === 3) { return 1; }
      throw new Error("fake: Information(" + what + ") chua mo phong");
    },
    InsertParagraphAfter: () => { insertParagraphAfter(state); },
  });
}

function insertParagraphAfter(state) {
  const last = lastParagraph(state);
  state.items.push(para("", last));
}

function makeParagraph(state, p, path) {
  const range = () => {
    const hit = layout(state).spans.find((s) => s.p === p);
    return makeRange(state, hit.start, hit.end, path + ".Range");
  };
  return strict(path, {
    Range: { get: range },
    Style: { get: () => strict(path + ".Style", { NameLocal: { get: () => p.style } }), set: (v) => {
      p.style = styleName(v);
      if (/^Heading/.test(p.style)) { p.font.bold = -1; }
    } },
    Alignment: { get: () => p.align, set: (v) => { p.align = v; } },
    KeepWithNext: { get: () => p.keepWithNext, set: (v) => { p.keepWithNext = v; } },
    OutlineLevel: { get: () => { const m = /^Heading (\d)$/.exec(p.style); return m ? Number(m[1]) : 10; } },
  });
}

function makeParagraphs(state, list, path) {
  return strict(path, {
    Count: { get: () => list().length },
    Last: { get: () => { const all = list(); return makeParagraph(state, all[all.length - 1], path + ".Last"); } },
    First: { get: () => makeParagraph(state, list()[0], path + ".First") },
  }, {
    Item: (n) => {
      const p = list()[Number(n) - 1];
      if (!p) { throw new Error("Paragraphs.Item(" + n + ") ngoai pham vi"); }
      return makeParagraph(state, p, path + ".Item(" + n + ")");
    },
  });
}

function makeCell(state, table, r, c, path) {
  const cell = () => {
    const row = table.cells[r - 1];
    if (!row || !row[c - 1]) { throw new Error("Cell(" + r + "," + c + ") ngoai bang"); }
    return row[c - 1];
  };
  const rangeOf = () => {
    const spans = layout(state).spans.filter((s) => s.table === table && s.cell[0] === r && s.cell[1] === c);
    const range = makeRange(state, spans[0].start, spans[spans.length - 1].end, path + ".Range");
    return new Proxy(range, {
      get: (t, key) => (key === "Text" ? cell().map((p) => p.text).join("\r") + "\r\u0007" : t[key]),
      set: (t, key, v) => {
        if (key !== "Text") { t[key] = v; return true; }
        const old = cell()[0];
        table.cells[r - 1][c - 1] = String(v).split("\r").map((line) => para(line, old));
        return true;
      },
    });
  };
  return strict(path, { Range: { get: rangeOf } });
}

function makeTable(state, table, path) {
  if (!table) { throw new Error("fake: bang khong co: " + path); }
  const tableParagraphs = () => [].concat(...table.cells.map((row) => [].concat(...row)));
  return strict(path, {
    Range: { get: () => { layout(state); return makeRange(state, table.start, table.end, path + ".Range"); } },
    Borders: { get: () => strict(path + ".Borders", { Enable: { get: () => table.borders, set: (v) => { table.borders = v; } } }) },
    Title: { get: () => table.title, set: (v) => { table.title = String(v); } },
    Style: { get: () => table.style, set: (v) => { table.style = String(v); } },
    PreferredWidthType: { get: () => table.widthType, set: (v) => { table.widthType = v; } },
    PreferredWidth: { get: () => table.width, set: (v) => { table.width = v; } },
    Rows: { get: () => strict(path + ".Rows", { Count: { get: () => table.cells.length } }) },
    Columns: { get: () => strict(path + ".Columns", { Count: { get: () => table.cols } }, {
      Item: (n) => {
        const col = table.columns[Number(n) - 1];
        if (!col) { throw new Error("Columns.Item(" + n + ") ngoai bang"); }
        return strict(path + ".Columns.Item(" + n + ")", {
          PreferredWidthType: { get: () => col.widthType, set: (v) => { col.widthType = v; } },
          PreferredWidth: { get: () => col.width, set: (v) => { col.width = v; } },
        });
      },
    }) },
    Paragraphs: { get: () => makeParagraphs(state, tableParagraphs, path + ".Paragraphs") },
  }, {
    Cell: (r, c) => makeCell(state, table, Number(r), Number(c), path + ".Cell(" + r + "," + c + ")"),
  });
}

function addTable(state, range, rows, cols) {
  const { spans } = layout(state);
  const at = Number(range.Start);
  const hit = spans.find((s) => s.kind === "p" && s.start <= at && at < s.end);
  const index = state.items.indexOf(hit && hit.p);
  if (index !== state.items.length - 1 || at !== hit.end - 1 || at !== Number(range.End)) {
    throw new Error("fake: Tables.Add chi mo phong voi Range thu gon o CUOI bai (Selection.EndKey(6))");
  }
  const blank = () => [para("")];
  const newRows = Array.from({ length: rows }, () => Array.from({ length: cols }, blank));
  const before = state.items[index - 1];
  if (hit.p.text === "" && before && before.type === "t") {
    // WPS: bang moi ngay sau mot bang thi GOP vao bang do (do that) - tra ve bang cu
    before.cells.push(...newRows.map((row) => row.slice(0, before.cols)));
    return before;
  }
  const table = {
    type: "t", cols, cells: newRows, borders: 1, title: "", style: "", widthType: 0, width: 0,
    columns: Array.from({ length: cols }, () => ({ widthType: 0, width: 0 })),
  };
  if (hit.p.text === "") {
    state.items.splice(index, 0, table);
  } else {
    state.items.splice(index + 1, 0, table, para("", hit.p));
  }
  return table;
}

// options: paragraphs: [{text, style}] (mac dinh mot doan trong); cursor: vi tri con tro ban dau (mac dinh 0)
export function makeComposeApp(options = {}) {
  const state = {
    items: (options.paragraphs && options.paragraphs.length ? options.paragraphs : [{ text: "" }])
      .map((p) => Object.assign(para(p.text), { style: p.style || "Normal" })),
    cursor: options.cursor || 0,
    fields: [],
    undo: [],
    breaks: 0,
  };
  const docEnd = () => layout(state).end;
  const atEnd = () => state.cursor === docEnd() - 1;
  const needEnd = (what) => { if (!atEnd()) { throw new Error("fake: Selection." + what + " chi mo phong khi con tro o CUOI bai (EndKey(6))"); } };

  const styles = strict("ActiveDocument.Styles", {}, {
    Item: (key) => {
      const name = typeof key === "number" || /^-\d+$/.test(String(key)) ? STYLE_IDS[String(key)] : (STYLE_NAMES.has(String(key)) ? String(key) : null);
      return name ? { NameLocal: name } : null;
    },
  });
  const galleries = strict("ListGalleries", {}, {
    Item: (n) => {
      const g = Number(n);
      if (g !== 1 && g !== 2) { throw new Error("fake: ListGalleries.Item(" + n + ") chua mo phong"); }
      return strict("ListGalleries.Item(" + g + ")", {
        ListTemplates: { get: () => strict("ListTemplates", {}, { Item: () => ({ __gallery: g }) }) },
      });
    },
  });
  const doc = strict("ActiveDocument", {
    Name: { get: () => "moi.docx" },
    FullName: { get: () => "/tmp/moi.docx" },
    Content: { get: () => {
      const range = makeRange(state, 0, docEnd(), "ActiveDocument.Content");
      return new Proxy(range, {
        get: (t, key) => {
          if (key === "InsertAfter") {
            return (s) => {
              const text = String(s);
              if (text.indexOf("\u0007") >= 0) { throw new Error("fake: InsertAfter co dau o bang"); }
              const pieces = text.split("\r");
              const last = lastParagraph(state);
              last.text += pieces[0];
              for (let i = 1; i < pieces.length; i++) { state.items.push(para(pieces[i], last)); }
              if (state.cursor >= docEnd() - text.length - 1) { state.cursor = docEnd() - 1; }
            };
          }
          if (key === "InsertParagraphAfter") { return () => insertParagraphAfter(state); }
          // Content luon phu ca bai: End doc lai sau moi lan chen (WPS/Word mo rong range theo chu chen)
          if (key === "End") { return docEnd(); }
          return t[key];
        },
      });
    } },
    Paragraphs: { get: () => makeParagraphs(state, () => layout(state).spans.filter((s) => s.kind === "p").map((s) => s.p), "ActiveDocument.Paragraphs") },
    Tables: { get: () => {
      const tables = state.items.filter((it) => it.type === "t");
      return strict("ActiveDocument.Tables", { Count: { get: () => tables.length } }, {
        Item: (n) => makeTable(state, tables[Number(n) - 1], "ActiveDocument.Tables.Item(" + n + ")"),
        Add: (range, rows, cols) => {
          const table = addTable(state, range, Number(rows), Number(cols));
          const index = state.items.filter((it) => it.type === "t").indexOf(table) + 1;
          return makeTable(state, table, "ActiveDocument.Tables.Item(" + index + ")");
        },
      });
    } },
    Styles: { get: () => styles },
    Fields: { get: () => strict("ActiveDocument.Fields", { Count: { get: () => state.fields.length } }, {
      Add: (range, type, code, preserve) => {
        const m = /^SEQ (\S+) \\\* ARABIC$/.exec(String(code));
        if (Number(type) !== -1 || !m || preserve !== false) { throw new Error("fake: Fields.Add chi mo phong SEQ <nhan> \\* ARABIC"); }
        const hits = paragraphsIn(state, Number(range.Start), Number(range.End));
        const p = hits[0].p;
        const offset = Number(range.Start) - hits[0].start;
        const number = String(state.fields.filter((f) => f.label === m[1]).length + 1);
        p.text = p.text.slice(0, offset) + number + p.text.slice(offset + (Number(range.End) - Number(range.Start)));
        state.fields.push({ label: m[1], paragraph: p });
        return strict("Field", { Result: { get: () => ({ Text: number }) } }, { Update: () => undefined });
      },
    }) },
  }, {
    Range: (a, b) => makeRange(state, Number(a), Number(b), "ActiveDocument.Range(" + a + "," + b + ")"),
  });
  const selection = strict("Selection", {
    Start: { get: () => state.cursor },
    End: { get: () => state.cursor },
    Range: { get: () => makeRange(state, state.cursor, state.cursor, "Selection.Range") },
    Style: {
      get: () => strict("Selection.Style", { NameLocal: { get: () => paragraphsIn(state, state.cursor, state.cursor)[0].p.style } }),
      set: (v) => { needEnd("Style"); const p = lastParagraph(state); p.style = styleName(v); if (/^Heading/.test(p.style)) { p.font.bold = -1; } },
    },
  }, {
    EndKey: (unit) => { if (Number(unit) !== 6) { throw new Error("fake: EndKey(" + unit + ") chua mo phong"); } state.cursor = docEnd() - 1; },
    TypeText: (text) => {
      needEnd("TypeText");
      const pieces = String(text).split(/\r|\n/);
      lastParagraph(state).text += pieces[0];
      for (let i = 1; i < pieces.length; i++) { state.items.push(para(pieces[i], lastParagraph(state))); }
      state.cursor = docEnd() - 1;
    },
    TypeParagraph: () => {
      needEnd("TypeParagraph");
      const last = lastParagraph(state);
      const next = para("", last);
      if (/^Heading/.test(last.style)) { next.style = "Normal"; next.font.bold = 0; }
      state.items.push(next);
      state.cursor = docEnd() - 1;
    },
    InsertBreak: (type) => {
      if (Number(type) !== 7) { throw new Error("fake: InsertBreak(" + type + ") chua mo phong"); }
      needEnd("InsertBreak");
      const last = lastParagraph(state);
      last.text += "\f";
      state.items.push(para("", last));
      state.cursor = docEnd() - 1;
    },
    Information: (what) => {
      if (Number(what) !== 12) { throw new Error("fake: Selection.Information(" + what + ") chua mo phong"); }
      const hit = paragraphsIn(state, state.cursor, state.cursor)[0];
      return !!(hit && hit.table);
    },
  });
  const undoRecord = strict("UndoRecord", {}, {
    StartCustomRecord: (title) => { state.undo.push("start:" + title); },
    EndCustomRecord: () => { state.undo.push("end"); },
  });
  const app = strict("Application", {
    Name: { get: () => "WPS Docs" },
    ActiveDocument: { get: () => doc },
    Selection: { get: () => selection },
    ListGalleries: { get: () => galleries },
    UndoRecord: { get: () => undoRecord },
  });
  APPS.set(app, state);
  return app;
}

// Ban chup de kiem: items = [{type: "p", text, style, align, list, keepWithNext, font} | {type: "t", cols, borders,
// title, style, widthType, width, columns, cells: [[[{text, align, font, ...}]]]}], fields, undo.
export function composeOf(app) {
  const state = APPS.get(app);
  return { items: JSON.parse(JSON.stringify(state.items)), fields: state.fields.map((f) => f.label), undo: state.undo.slice() };
}
