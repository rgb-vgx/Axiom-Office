// Fake WPS Writer cho nhom lenh SOAT PHAN TRANG cua add-in WPS (writer.checkLayout, writer.deleteParagraphs) -
// doc_from_brief. Fake KHONG dan trang: moi doan mang san trang cua no (page = trang cua cuoi doan, startPage = trang
// cua dau doan), giong mot ban chup bo cuc sau Repaginate(). Tach khoi addin_fake_compose.mjs vi o day can trang/vi tri
// tung doan va dong cuoi hang cua bang, con fake kia can style/danh sach/truong SEQ.
//
// File nay KHONG phai test. Hanh vi duoc mo phong da DO tren WPS 12.1.2.29182 that (01/11/2026, scripts/wps/probe):
//   - doc.Repaginate(), doc.ComputeStatistics(2) = so trang, (0) = so chu.
//   - doc.Paragraphs.Count / Item(n) gom CA doan trong o bang VA mot doan "dau cuoi hang" moi hang: Text cua o la
//     "A\r\u0007", cua dau cuoi hang la "\r\u0007". Vi tri: moi doan dai len(chu) + 1.
//   - Dau ngat trang (Selection.InsertBreak(7) / writeContent {pageBreak}) la MOT doan Text "\f\r" nam o trang TRUOC
//     dau ngat; hai dau ngat lien nhau -> doan thu hai nam mot minh tren trang trang.
//   - Range.Information(3) = trang (cuoi khoang), doc.Range(s, s).Information(3) = trang tai vi tri s,
//     Information(6) = khoang cach tu dinh trang (pt), Information(12) = true/false (trong bang - kieu boolean).
//   - Paragraph.OutlineLevel: Heading 1 = 1, Heading 2 = 2, doan thuong = 10. Range.InlineShapes.Count chay.
//   - Table.Range.Start/End, Table.Range.Characters.First/Last.Information(3), Table.Rows.Count, Table.Title chay.
//   - doc.TablesOfContents.Count (0 khi chua co muc luc), Item(i).Range.Start/End.
//   - Paragraphs.Item(n).Range.Delete() xoa ca doan (ke ca doan "\f\r" va doan trong cuoi bai): Count giam 1, cac
//     doan sau DON so len - xoa nhieu doan phai xoa tu so lon ve so nho. Range.MoveEnd(1, -1) roi Delete() chi xoa chu.
//   - Application.UndoRecord.StartCustomRecord / EndCustomRecord.
//
// Moi doi tuong "strict": doc thuoc tinh/ham khong mo phong thi NEM loi - handler goi API chua do tren WPS that thi test do.

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
    set(_t, key) {
      throw new Error("fake: gan " + path + "." + String(key) + " chua mo phong");
    },
  });
}

// Doan dau vao: {t: "chu" (khong co dau doan), page, startPage? (mac dinh = page), level? (OutlineLevel, mac dinh 10),
// top? (Information(6)), shapes? (InlineShapes.Count)}. Bang: {table: [["A","B"],["1","2"]], page, lastPage?,
// title?} -> moi o mot doan + mot doan dau cuoi hang, tat ca thuoc bang. Muc luc: {toc: ["Muc 1", ...], page}.
function expand(blocks) {
  const paras = [], tables = [], tocs = [];
  for (const b of blocks) {
    if (b.table) {
      const index = tables.length + 1;
      const first = b.page, last = b.lastPage || b.page;
      const cells = [];
      b.table.forEach((row, r) => {
        // Hang cuoi nam o trang cuoi cua bang, cac hang truoc o trang dau (du cho kiem bang bi cat).
        const page = r === b.table.length - 1 ? last : first;
        for (const cell of row) { cells.push({ t: String(cell), page, startPage: page, level: 10, top: 0, shapes: 0, table: index, cell: true }); }
        cells.push({ t: "", page, startPage: page, level: 10, top: 0, shapes: 0, table: index, rowEnd: true });
      });
      paras.push(...cells);
      tables.push({ index, rows: b.table.length, title: b.title || "", first, last });
      continue;
    }
    if (b.toc) {
      const from = paras.length + 1;
      for (const line of b.toc) { paras.push({ t: line, page: b.page, startPage: b.page, level: 10, top: 0, shapes: 0, table: 0 }); }
      tocs.push({ from, to: paras.length });
      continue;
    }
    paras.push({ t: b.t, page: b.page, startPage: b.startPage || b.page, level: b.level || 10, top: b.top || 0,
      shapes: b.shapes || 0, table: 0 });
  }
  return { paras, tables, tocs };
}

function positions(state) {
  let pos = 0;
  return state.paras.map((p) => {
    const span = { start: pos, end: pos + p.t.length + 1 };
    pos = span.end;
    return span;
  });
}

function rawText(p) {
  if (p.cell) { return p.t + "\r\u0007"; }
  if (p.rowEnd) { return "\r\u0007"; }
  return p.t + "\r";
}

function paragraphAt(state, position) {
  const spans = positions(state);
  for (let i = 0; i < spans.length; i++) {
    if (position >= spans[i].start && position < spans[i].end) { return state.paras[i]; }
  }
  return state.paras[state.paras.length - 1];
}

function makeRange(state, start, end, path, owner) {
  let curEnd = end;
  return strict(path, {
    Start: { get: () => start },
    End: { get: () => curEnd },
    Text: { get: () => {
      if (owner) { return rawText(owner); }
      throw new Error("fake: Text cua khoang tu do chua mo phong");
    } },
    InlineShapes: { get: () => strict(path + ".InlineShapes", { Count: { get: () => (owner ? owner.shapes : 0) } }) },
  }, {
    Information: (what) => {
      const w = Number(what);
      const p = owner || paragraphAt(state, start);
      if (w === 3) { return owner ? p.page : p.startPage; }
      if (w === 6) { return p.top; }
      if (w === 12) { return p.table > 0; }
      throw new Error("fake: Information(" + what + ") chua mo phong");
    },
    MoveEnd: (unit, count) => {
      if (Number(unit) !== 1 || Number(count) !== -1 || !owner) { throw new Error("fake: MoveEnd(" + unit + "," + count + ") chua mo phong"); }
      curEnd -= 1;
      return -1;
    },
    Delete: () => {
      if (!owner) { throw new Error("fake: Delete cua khoang tu do chua mo phong"); }
      if (owner.table) { throw new Error("fake: xoa doan trong bang - WPS lam hong bang"); }
      state.deletes.push(owner.t);
      if (curEnd < end) { owner.t = ""; return; } // chi xoa chu (MoveEnd(1, -1))
      const at = state.paras.indexOf(owner) + 1;
      state.paras.splice(at - 1, 1);
      for (const toc of state.tocs) {
        if (at < toc.from) { toc.from -= 1; toc.to -= 1; }
      }
    },
  });
}

function makeParagraph(state, n) {
  const p = state.paras[n - 1];
  const spans = positions(state);
  const path = "Paragraphs.Item(" + n + ")";
  return strict(path, {
    Range: { get: () => makeRange(state, spans[n - 1].start, spans[n - 1].end, path + ".Range", p) },
    OutlineLevel: { get: () => p.level },
  });
}

function tableRange(state, table, path) {
  const spans = positions(state);
  const idx = state.paras.map((p, i) => (p.table === table.index ? i : -1)).filter((i) => i >= 0);
  const start = spans[idx[0]].start, end = spans[idx[idx.length - 1]].end;
  const edge = (page, name) => strict(path + ".Characters." + name, {}, {
    Information: (what) => {
      if (Number(what) !== 3) { throw new Error("fake: Characters." + name + ".Information(" + what + ") chua mo phong"); }
      return page;
    },
  });
  return strict(path, {
    Start: { get: () => start },
    End: { get: () => end },
    Characters: { get: () => strict(path + ".Characters", {
      First: { get: () => edge(table.first, "First") },
      Last: { get: () => edge(table.last, "Last") },
    }) },
  });
}

// blocks: xem expand(); options.pages (mac dinh: trang lon nhat), options.words (mac dinh: dem tu cua moi doan).
export function makeLayoutApp(blocks, options = {}) {
  const { paras, tables, tocs } = expand(blocks);
  const state = { paras, tables, tocs, deletes: [], undo: [], repaginated: 0 };
  const pages = () => (options.pages !== undefined ? options.pages : Math.max(1, ...state.paras.map((p) => p.page)));
  const words = () => (options.words !== undefined ? options.words
    : state.paras.reduce((n, p) => n + p.t.replace(/\f/g, " ").split(/\s+/).filter(Boolean).length, 0));
  const doc = strict("ActiveDocument", {
    Name: { get: () => "bao-cao.docx" },
    FullName: { get: () => "/tmp/bao-cao.docx" },
    Paragraphs: { get: () => strict("Paragraphs", { Count: { get: () => state.paras.length } }, {
      Item: (n) => {
        const i = Number(n);
        if (!(i >= 1 && i <= state.paras.length)) { throw new Error("Paragraphs.Item(" + n + ") ngoai pham vi"); }
        return makeParagraph(state, i);
      },
    }) },
    Tables: { get: () => strict("Tables", { Count: { get: () => state.tables.length } }, {
      Item: (n) => {
        const table = state.tables[Number(n) - 1];
        if (!table) { throw new Error("Tables.Item(" + n + ") ngoai pham vi"); }
        const path = "Tables.Item(" + n + ")";
        return strict(path, {
          Range: { get: () => tableRange(state, table, path + ".Range") },
          Rows: { get: () => strict(path + ".Rows", { Count: { get: () => table.rows } }) },
          Title: { get: () => table.title },
        });
      },
    }) },
    TablesOfContents: { get: () => strict("TablesOfContents", { Count: { get: () => state.tocs.length } }, {
      Item: (n) => {
        const toc = state.tocs[Number(n) - 1];
        if (!toc) { throw new Error("TablesOfContents.Item(" + n + ") ngoai pham vi"); }
        const spans = positions(state);
        return strict("TablesOfContents.Item(" + n + ")", {
          Range: { get: () => strict("TablesOfContents.Item(" + n + ").Range", {
            Start: { get: () => spans[toc.from - 1].start }, End: { get: () => spans[toc.to - 1].end },
          }) },
        });
      },
    }) },
    Content: { get: () => strict("Content", { End: { get: () => { const s = positions(state); return s[s.length - 1].end; } } }) },
  }, {
    Repaginate: () => { state.repaginated += 1; },
    ComputeStatistics: (what) => {
      if (Number(what) === 2) { return pages(); }
      if (Number(what) === 0) { return words(); }
      throw new Error("fake: ComputeStatistics(" + what + ") chua mo phong");
    },
    Range: (s, e) => {
      if (Number(s) !== Number(e)) { throw new Error("fake: doc.Range(a, b) voi a != b chua mo phong"); }
      return makeRange(state, Number(s), Number(e), "ActiveDocument.Range(" + s + "," + e + ")", null);
    },
  });
  const undoRecord = strict("UndoRecord", {}, {
    StartCustomRecord: (title) => { state.undo.push("start:" + title); },
    EndCustomRecord: () => { state.undo.push("end"); },
  });
  const app = strict("Application", {
    Name: { get: () => "WPS Docs" },
    ActiveDocument: { get: () => doc },
    UndoRecord: { get: () => undoRecord },
  });
  APPS.set(app, state);
  return app;
}

// Ban chup de kiem: chu tung doan con lai (dau doan bo), cac lan xoa, nhat ky UndoRecord, so lan Repaginate.
export function layoutOf(app) {
  const state = APPS.get(app);
  return { texts: state.paras.map((p) => p.t), deletes: state.deletes.slice(), undo: state.undo.slice(), repaginated: state.repaginated };
}
