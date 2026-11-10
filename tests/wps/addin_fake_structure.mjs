// Fake Writer cho nhom lenh CAU TRUC cua add-in WPS (writer.setStyle, writer.insertToc, writer.formatTable) - P1-2, P1-3
// cua docs/research/doi-chieu-wps-ai.md. Tach khoi addin_fake.mjs vi fake do mo phong VAN BAN (chu, vi tri, revision),
// con o day can STYLE cua tung doan, muc luc va thuoc tinh dinh dang bang.
//
// File nay KHONG phai test. Hanh vi duoc mo phong da DO tren WPS 12.1.2.29182 that (01/11/2026, tests/wps/REFERENCE.md
// "Style, muc luc, dinh dang bang"):
//   - doc.Styles.Item(ten) phan biet hoa thuong, tra NULL (khong nem loi) khi khong co; Item(-2) = "Heading 1" (hang
//     WdBuiltinStyle cua Word dung duoc).
//   - Paragraph.Style = "<ten khong co>" KHONG bao loi: doan am tham thanh "Normal". Doc lai Style tra doi tuong style
//     (NameLocal).
//   - Range.InsertParagraphBefore(): doan moi mang style cua doan dich.
//   - doc.TablesOfContents.Add(range, true, 1, n) tao muc luc tu cac Heading 1..n; toc.Update() doc lai Heading.
//   - Bang: AutoFitBehavior(1|2), Rows.Item(1).HeadingFormat, Range.Font.Name/Size/Bold, ParagraphFormat.Alignment,
//     Style = "Table Grid", Borders.Enable chay; Rows.Item(n).Shading.BackgroundPatternColor = mau thi WPS BO QUA (doc
//     lai van -16777216 = auto). Bang chen vao doan Heading thi chu trong o mang style Heading do.
// Don gian hoa: muc luc KHONG chen doan vao danh sach doan (writerStructureOf(app).tocs ghi cho chen).
//
// Moi doi tuong "strict": doc thuoc tinh/ham khong mo phong thi NEM loi - handler goi API chua do tren WPS that thi test do.

const BUILTIN = {
  "-1": "Normal", "-2": "Heading 1", "-3": "Heading 2", "-4": "Heading 3", "-5": "Heading 4", "-6": "Heading 5",
  "-7": "Heading 6", "-8": "Heading 7", "-9": "Heading 8", "-10": "Heading 9", "-35": "Caption", "-49": "List Bullet",
  "-50": "List Number", "-63": "Title", "-75": "Subtitle", "-181": "Quote",
};
const AUTO_COLOR = -16777216;
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

// options:
//   paragraphs: [{text, style}]            style mac dinh "Normal"
//   styles: ["Ten co quan", ...]            style rieng cua tai lieu (ngoai BUILTIN)
//   selection: [n, ...]                     so doan dang chon (mac dinh [1])
//   tables: [{cells: [[chu]], cellStyle?, title?}]   cellStyle: style cua chu trong moi o (mac dinh "Normal")
//   cursorTable: n                          con tro dang trong bang n (0 = khong)
export function makeStructureApp(options = {}) {
  const custom = options.styles || [];
  const state = {
    paragraphs: (options.paragraphs || []).map((p) => ({ text: p.text, style: p.style || "Normal" })),
    selection: options.selection || [1],
    cursorAt: (options.selection || [1])[0],
    tables: (options.tables || []).map((t) => ({
      cells: t.cells.map((row) => row.slice()),
      cellStyle: t.cells.map((row) => row.map(() => t.cellStyle || "Normal")),
      align: t.cells.map((row) => row.map(() => 0)),
      bold: t.cells.map(() => false),
      headingFormat: t.cells.map(() => false),
      fill: t.cells.map(() => AUTO_COLOR),
      font: { name: "Calibri", size: 11, color: AUTO_COLOR },
      style: "",
      borders: 0,
      borderColor: null,
      autoFit: null,
      title: t.title || "",
    })),
    cursorTable: options.cursorTable || 0,
    tocs: [],
    undoStack: [],
    group: null,
    calls: [],
  };
  const snapshot = () => JSON.parse(JSON.stringify({ paragraphs: state.paragraphs, tables: state.tables, tocs: state.tocs }));
  function beforeChange() { if (state.group === null) { state.undoStack.push(snapshot()); } }

  function styleName(value) {
    // Doi tuong style cua fake / so hang Word / ten. Ten khong co -> null.
    if (value && typeof value === "object" && STYLE_OBJECTS.has(value)) { return STYLE_OBJECTS.get(value); }
    if (typeof value === "number") { return BUILTIN[String(value)] || null; }
    const name = String(value);
    return Object.values(BUILTIN).includes(name) || custom.includes(name) ? name : null;
  }
  const STYLE_OBJECTS = new WeakMap();
  function styleObject(name) {
    const obj = strict("Style(" + name + ")", { NameLocal: { get: () => name }, Type: { get: () => 1 } });
    STYLE_OBJECTS.set(obj, name);
    return obj;
  }

  const RANGES = new WeakMap();   // Range -> { para: so doan ma Range tro toi (diem chen) }
  function paragraphRange(n) {
    const p = state.paragraphs[n - 1];
    const range = strict("doc.Paragraphs.Item(" + n + ").Range", {
      Text: { get: () => p.text + "\r" },
      Style: { set: (v) => setParagraphStyle(n, v) },
    }, {
      InsertParagraphBefore: () => {
        state.calls.push("InsertParagraphBefore(" + n + ")");
        beforeChange();
        state.paragraphs.splice(n - 1, 0, { text: "", style: p.style });
      },
      Collapse: () => undefined,
      Select: () => { state.cursorAt = n; },
    });
    RANGES.set(range, { para: n });
    return range;
  }
  function setParagraphStyle(n, value) {
    const name = styleName(value);
    if (name === null && typeof value !== "string") { throw new Error("Style khong ton tai: " + String(value)); }
    state.calls.push("Paragraph(" + n + ").Style=" + (name || "Normal"));
    beforeChange();
    state.paragraphs[n - 1].style = name || "Normal";   // WPS: ten la -> Normal, khong bao loi
  }
  function paragraph(n) {
    if (!Number.isInteger(n) || n < 1 || n > state.paragraphs.length) { throw new Error("Paragraphs.Item ngoai tai lieu: " + n); }
    return strict("doc.Paragraphs.Item(" + n + ")", {
      Range: { get: () => paragraphRange(n) },
      Style: { get: () => styleObject(state.paragraphs[n - 1].style), set: (v) => setParagraphStyle(n, v) },
      OutlineLevel: { get: () => (/^Heading (\d)$/.exec(state.paragraphs[n - 1].style) || [0, 10])[1] * 1 },
    });
  }

  function tocEntries(lower) {
    return state.paragraphs.filter((p) => {
      const m = /^Heading (\d)$/.exec(p.style);
      return m && Number(m[1]) <= lower;
    }).map((p) => p.text);
  }
  function tocObject(i) {
    const toc = state.tocs[i - 1];
    return strict("doc.TablesOfContents.Item(" + i + ")", {
      Range: { get: () => strict("toc.Range", {
        Text: { get: () => toc.entries.map((e) => e + "\t1\r").join("") },
        Paragraphs: { get: () => strict("toc.Range.Paragraphs", { Count: { get: () => toc.entries.length } }) },
      }) },
      UpperHeadingLevel: { get: () => 1 },
      LowerHeadingLevel: { get: () => toc.lower },
    }, {
      Update: () => { state.calls.push("toc.Update"); beforeChange(); toc.entries = tocEntries(toc.lower); toc.updates++; },
    });
  }
  const tablesOfContents = () => strict("doc.TablesOfContents", { Count: { get: () => state.tocs.length } }, {
    Item: (i) => {
      const idx = Number(i);
      if (!Number.isInteger(idx) || idx < 1 || idx > state.tocs.length) { throw new Error("TablesOfContents.Item ngoai pham vi"); }
      return tocObject(idx);
    },
    Add: (range, useHeadings, upper, lower) => {
      const where = RANGES.get(range);
      if (!where) { throw new Error("TablesOfContents.Add: tham so dau phai la Range cua tai lieu"); }
      if (useHeadings !== true || Number(upper) !== 1) { throw new Error("TablesOfContents.Add: fake chi mo phong (range, true, 1, n)"); }
      state.calls.push("TablesOfContents.Add(" + where.para + "," + lower + ")");
      beforeChange();
      state.tocs.push({ at: where.para, lower: Number(lower), entries: tocEntries(Number(lower)), updates: 0 });
      return tocObject(state.tocs.length);
    },
  });

  function cellObject(t, r, c) {
    const table = state.tables[t - 1];
    if (r < 1 || r > table.cells.length || c < 1 || c > table.cells[0].length) { throw new Error("Cell(" + r + "," + c + ") ngoai bang"); }
    return strict("Tables.Item(" + t + ").Cell(" + r + "," + c + ")", {
      Range: { get: () => strict("Cell.Range", {
        Text: { get: () => table.cells[r - 1][c - 1] + "\r\x07" },
        Style: { get: () => styleObject(table.cellStyle[r - 1][c - 1]) },
        ParagraphFormat: { get: () => strict("Cell.Range.ParagraphFormat", {
          Alignment: { get: () => table.align[r - 1][c - 1], set: (v) => { beforeChange(); table.align[r - 1][c - 1] = Number(v); } },
          // Do tren WPS that: o mang "Heading 1" doc ra 1; doan thuong 10.
          OutlineLevel: { get: () => (/^Heading (\d)$/.exec(table.cellStyle[r - 1][c - 1]) || [0, 10])[1] * 1 },
        }) },
      }) },
    });
  }
  function tableObject(t) {
    const table = state.tables[t - 1];
    const all = (fn) => table.cells.forEach((row, r) => row.forEach((_v, c) => fn(r, c)));
    const tableRange = strict("Tables.Item(" + t + ").Range", {
      Font: { get: () => strict("Table.Range.Font", {
        Name: { get: () => table.font.name, set: (v) => { beforeChange(); table.font.name = String(v); } },
        Size: { get: () => table.font.size, set: (v) => { beforeChange(); table.font.size = Number(v); } },
        Color: { get: () => table.font.color, set: (v) => { beforeChange(); table.font.color = Number(v); } },
      }) },
      ParagraphFormat: { get: () => strict("Table.Range.ParagraphFormat", {
        Alignment: { set: (v) => { beforeChange(); all((r, c) => { table.align[r][c] = Number(v); }); } },
      }) },
      Style: { set: (v) => {
        const name = styleName(v);
        state.calls.push("Table(" + t + ").Range.Style=" + (name || "Normal"));
        beforeChange();
        all((r, c) => { table.cellStyle[r][c] = name || "Normal"; });
        // Do tren WPS that: dat lai style cho ca bang thi chu dam va font cua bang cung ve mac dinh cua style.
        table.bold = table.bold.map(() => false);
        table.font = { name: "Calibri", size: 11, color: AUTO_COLOR };
      } },
      Start: { get: () => 1000 * t },
    });
    const row = (r) => {
      if (r < 1 || r > table.cells.length) { throw new Error("Rows.Item ngoai bang: " + r); }
      return strict("Tables.Item(" + t + ").Rows.Item(" + r + ")", {
        HeadingFormat: { get: () => (table.headingFormat[r - 1] ? -1 : 0), set: (v) => { beforeChange(); table.headingFormat[r - 1] = !!v; } },
        Range: { get: () => strict("Row.Range", { Font: { get: () => strict("Row.Range.Font", {
          Bold: { get: () => (table.bold[r - 1] ? -1 : 0), set: (v) => { beforeChange(); table.bold[r - 1] = !!v; } },
          Color: { set: () => { beforeChange(); } },
        }) } }) },
        // WPS 12.1.2 that: gan mau nen hang thi bi bo qua (doc lai van auto).
        Shading: { get: () => strict("Row.Shading", {
          BackgroundPatternColor: { get: () => table.fill[r - 1], set: () => { state.calls.push("Row(" + r + ").Shading ignored"); } },
        }) },
      });
    };
    return strict("doc.Tables.Item(" + t + ")", {
      Range: { get: () => tableRange },
      Rows: { get: () => strict("Tables.Item(" + t + ").Rows", { Count: { get: () => table.cells.length } }, { Item: (r) => row(Number(r)) }) },
      Columns: { get: () => strict("Tables.Item(" + t + ").Columns", { Count: { get: () => table.cells[0].length } }) },
      Style: { get: () => table.style, set: (v) => { beforeChange(); table.style = String(v); } },
      Title: { get: () => table.title, set: (v) => { beforeChange(); table.title = String(v); } },
      Borders: { get: () => strict("Table.Borders", {
        Enable: { get: () => (table.borders ? -1 : 0), set: (v) => { beforeChange(); table.borders = Number(v) ? 1 : 0; } },
        OutsideColor: { set: (v) => { beforeChange(); table.borderColor = Number(v); } },
        InsideColor: { set: (v) => { beforeChange(); table.borderColor = Number(v); } },
      }) },
    }, {
      Cell: (r, c) => cellObject(t, Number(r), Number(c)),
      AutoFitBehavior: (mode) => { beforeChange(); table.autoFit = Number(mode); },
    });
  }
  const tables = () => strict("doc.Tables", { Count: { get: () => state.tables.length } }, {
    Item: (i) => {
      const idx = Number(i);
      if (!Number.isInteger(idx) || idx < 1 || idx > state.tables.length) { throw new Error("Tables.Item ngoai pham vi"); }
      return tableObject(idx);
    },
  });

  const doc = strict("doc", {
    Name: { get: () => "cau-truc.docx" },
    FullName: { get: () => "/tmp/cau-truc.docx" },
    Paragraphs: { get: () => strict("doc.Paragraphs", { Count: { get: () => state.paragraphs.length } }, { Item: (i) => paragraph(Number(i)) }) },
    Styles: { get: () => strict("doc.Styles", { Count: { get: () => Object.keys(BUILTIN).length + custom.length } }, {
      Item: (key) => { const name = styleName(key); return name === null ? null : styleObject(name); },
    }) },
    TablesOfContents: { get: () => tablesOfContents() },
    Tables: { get: () => tables() },
  }, {
    Undo: (n) => {
      let done = 0;
      for (let i = 0; i < (Number(n) || 1); i++) {
        const prev = state.undoStack.pop();
        if (!prev) { break; }
        Object.assign(state, prev);
        done++;
      }
      return done > 0;
    },
  });

  const selectionRange = () => {
    const range = strict("Selection.Range", {}, { Collapse: () => undefined });
    RANGES.set(range, { para: state.cursorAt });
    return range;
  };
  const selection = () => strict("Selection", {
    Paragraphs: { get: () => strict("Selection.Paragraphs", { Count: { get: () => state.selection.length } }, {
      Item: (i) => paragraph(state.selection[Number(i) - 1]),
    }) },
    Range: { get: () => selectionRange() },
    Tables: { get: () => strict("Selection.Tables", { Count: { get: () => (state.cursorTable ? 1 : 0) } }, {
      Item: (i) => { if (Number(i) !== 1 || !state.cursorTable) { throw new Error("Selection.Tables.Item ngoai pham vi"); } return tableObject(state.cursorTable); },
    }) },
  }, {
    Information: (what) => { if (Number(what) !== 12) { throw new Error("Selection.Information(" + what + ") chua mo phong"); } return state.cursorTable > 0; },
    EndKey: () => undefined,
  });

  const undoRecord = strict("app.UndoRecord", {}, {
    StartCustomRecord: (title) => { state.calls.push("undo:start:" + title); state.undoStack.push(snapshot()); state.group = title; },
    EndCustomRecord: () => { state.calls.push("undo:end"); state.group = null; },
  });

  const app = strict("app", {
    Name: { get: () => "WPS Writer" },
    ActiveDocument: { get: () => doc },
    Selection: { get: () => selection() },
    UndoRecord: { get: () => undoRecord },
  });
  APPS.set(app, state);
  return app;
}

// Trang thai doc duoc de test so sanh (khong qua API WPS).
export function writerStructureOf(app) {
  const state = APPS.get(app);
  if (!state) { throw new Error("writerStructureOf: khong phai app cua addin_fake_structure"); }
  return JSON.parse(JSON.stringify({
    paragraphs: state.paragraphs.map((p) => [p.text, p.style]),
    tocs: state.tocs.map((t) => ({ at: t.at, lower: t.lower, entries: t.entries, updates: t.updates })),
    tables: state.tables.map((t) => ({
      style: t.style, title: t.title, borders: t.borders, borderColor: t.borderColor, autoFit: t.autoFit, font: t.font,
      bold: t.bold, headingFormat: t.headingFormat, fill: t.fill, align: t.align, cellStyle: t.cellStyle,
    })),
  }));
}
