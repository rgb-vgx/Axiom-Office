// Fake WPS Presentation cho nhom lenh VE cua add-in WPS (wpp.addShape, wpp.addParagraphs) - dung slide tu spec
// (slides_from_brief). Tach khoi addin_fake.mjs vi fake do chi giu chu tho cua shape, con o day can nen, vien, bong,
// tung doan (cap, gach dau dong, can le, thut le), tung doan chu nhan manh va CHIEU CAO CHU.
//
// File nay KHONG phai test. Hanh vi duoc mo phong da DO tren WPS 12.1.2.29182 that (01/11/2026, REPL Xvfb):
//   - Shapes.AddShape(type, l, t, w, h): AutoShapeType 1 chu nhat, 5 bo goc, 9 elip, 52 chevron. Hinh moi mang style
//     theme: nen GRADIENT (Fill.Type 3), bong (Shadow.Visible -1), chu TRANG co bong, can GIUA. Gan Fill.ForeColor.RGB
//     khi chua Fill.Solid() thi van la gradient; Fill.Solid() roi ForeColor.RGB (BGR) moi ra nen phang (Type 1).
//   - Shapes.AddTextbox(1, ...): khong nen, khong vien, khong bong, chu den, can trai.
//   - ParagraphFormat.Bullet.Visible = -1 ma khong dat Type thi ra DANH SO ("1.", Type 2); Type = 1 + Character = 8226
//     moi la dau cham.
//   - TextRange.Words(n) tra NULL; Characters(start, len) (1-based, trong doan) dung duoc.
//   - TextRange.BoundHeight do duoc: moi doan = so dong x 1,2 x co chu + SpaceBefore; so ky tu mot dong =
//     floor((rong - MarginLeft - MarginRight - LeftIndent) / (0,5 x co chu)). Khop dung hai so do that: rong 300, co 20
//     -> 96; co 40 -> 288. Khung KHONG tu gian (AutoSize 0): chu tran thi BoundHeight > Height.
//   - TextFrame2.TextRange.Paragraphs(k).ParagraphFormat.LeftIndent / FirstLineIndent; ParagraphFormat.SpaceBefore,
//     LineRuleBefore chay. Adjustments.Item(1) = x KHONG gan duoc (loi cu phap) - fake bao loi khi dung Adjustments.
//   - App khong co UndoRecord (Application.UndoRecord = undefined tren WPP).
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
    set(_t, key, value) {
      if (Object.prototype.hasOwnProperty.call(props, key) && props[key].set) { props[key].set(value); return true; }
      throw new Error("fake: gan " + path + "." + String(key) + " chua mo phong");
    },
  });
}

const rw = (obj, key, path) => ({ get: () => obj[key], set: (v) => { obj[key] = typeof v === "number" ? v : v; } });

function newParagraph(text, base) {
  return {
    text, size: base.size, bold: base.bold, italic: base.italic, color: base.color, font: base.font, align: base.align,
    indentLevel: 1, bullet: { visible: 0, type: 0, character: 0 }, leftIndent: 0, firstIndent: 0, spaceBefore: 0,
    lineRuleBefore: -1, runs: [], textShadow: base.textShadow,
  };
}

// Chieu cao chu (pt) cua khung - mo hinh da khop so do that (dau file).
export function boundHeightOf(shape) {
  let total = 0;
  for (const p of shape.paragraphs) {
    const usable = shape.width - shape.marginLeft - shape.marginRight - p.leftIndent;
    const perLine = Math.max(1, Math.floor(usable / (0.5 * p.size)));
    let lines = 0;
    for (const piece of p.text.split("\v")) { lines += Math.max(1, Math.ceil(piece.length / perLine)); }
    total += lines * 1.2 * p.size + p.spaceBefore;
  }
  return total;
}

function makeFont(target, path) {
  // target: { paragraphs: [...] } (ca khung / mot doan) hoac { run } (mot doan chu Characters)
  const each = (fn) => { for (const p of target.paragraphs) { fn(p); } };
  const first = () => target.run || target.paragraphs[0];
  const setAll = (key) => (v) => {
    if (target.run) { target.run[key] = v; return; }
    each((p) => { p[key] = v; p.runs = p.runs.filter((r) => { delete r[key]; return Object.keys(r).length > 2; }); });
  };
  return strict(path, {
    Size: { get: () => first().size, set: setAll("size") },
    Bold: { get: () => first().bold, set: setAll("bold") },
    Italic: { get: () => first().italic, set: setAll("italic") },
    Name: { get: () => first().font, set: setAll("font") },
    Shadow: { get: () => first().textShadow, set: setAll("textShadow") },
    Color: { get: () => strict(path + ".Color", { RGB: { get: () => first().color, set: setAll("color") } }) },
  });
}

function makeParagraphFormat(paras, path) {
  const each = (fn) => { for (const p of paras) { fn(p); } };
  const bullet = strict(path + ".Bullet", {
    Visible: { get: () => paras[0].bullet.visible, set: (v) => each((p) => {
      p.bullet.visible = v;
      if (v === -1 && p.bullet.type === 0) { p.bullet.type = 2; } // WPS: bat bullet ma chua dat Type -> danh so
    }) },
    Type: { get: () => paras[0].bullet.type, set: (v) => each((p) => { p.bullet.type = v; }) },
    Character: { get: () => paras[0].bullet.character, set: (v) => each((p) => { p.bullet.character = v; }) },
  });
  return strict(path, {
    Alignment: { get: () => paras[0].align, set: (v) => each((p) => { p.align = v; }) },
    SpaceBefore: { get: () => paras[0].spaceBefore, set: (v) => each((p) => { p.spaceBefore = v; }) },
    LineRuleBefore: { get: () => paras[0].lineRuleBefore, set: (v) => each((p) => { p.lineRuleBefore = v; }) },
    Bullet: { get: () => bullet },
  });
}

function makeParagraphRange(shape, k, path) {
  const p = shape.paragraphs[k - 1];
  return strict(path, {
    Text: { get: () => p.text + (k < shape.paragraphs.length ? "\r" : "") },
    Font: { get: () => makeFont({ paragraphs: [p] }, path + ".Font") },
    ParagraphFormat: { get: () => makeParagraphFormat([p], path + ".ParagraphFormat") },
    IndentLevel: { get: () => p.indentLevel, set: (v) => { p.indentLevel = v; } },
  }, {
    Characters: (start, length) => {
      const s = Number(start), n = Number(length);
      if (!(s >= 1 && n >= 1 && s + n - 1 <= p.text.length)) { throw new Error("Characters(" + start + "," + length + ") ngoai doan"); }
      const run = { start: s, length: n };
      p.runs.push(run);
      return strict(path + ".Characters(" + s + "," + n + ")", {
        Text: { get: () => p.text.substr(s - 1, n) },
        Font: { get: () => makeFont({ run, paragraphs: [p] }, path + ".Characters.Font") },
      });
    },
    Words: () => null, // do tren WPS that: Words(n) tra null
  });
}

function makeTextRange(shape, path) {
  const base = () => shape.paragraphs[0];
  return strict(path, {
    Text: {
      get: () => shape.paragraphs.map((p) => p.text).join("\r"),
      set: (v) => {
        const b = base();
        shape.paragraphs = String(v).split("\r").map((t) => newParagraph(t, b));
      },
    },
    Font: { get: () => makeFont({ paragraphs: shape.paragraphs }, path + ".Font") },
    ParagraphFormat: { get: () => makeParagraphFormat(shape.paragraphs, path + ".ParagraphFormat") },
    BoundHeight: { get: () => boundHeightOf(shape) },
  }, {
    Paragraphs: (k) => {
      if (k === undefined) {
        return strict(path + ".Paragraphs()", { Count: { get: () => shape.paragraphs.length } });
      }
      const i = Number(k);
      if (!(i >= 1 && i <= shape.paragraphs.length)) { throw new Error("Paragraphs(" + k + ") ngoai khung"); }
      return makeParagraphRange(shape, i, path + ".Paragraphs(" + i + ")");
    },
    Characters: (start, length) => strict(path + ".Characters", {
      Text: { get: () => shape.paragraphs.map((p) => p.text).join("\r").substr(Number(start) - 1, Number(length)) },
    }),
    Words: () => null,
  });
}

function makeShape(slide, shape, path) {
  const frame2 = strict(path + ".TextFrame2", {
    TextRange: { get: () => strict(path + ".TextFrame2.TextRange", {}, {
      Paragraphs: (k) => {
        const p = shape.paragraphs[Number(k) - 1];
        if (!p) { throw new Error("TextFrame2.Paragraphs(" + k + ") ngoai khung"); }
        return strict(path + ".TextFrame2.Paragraphs(" + k + ")", {
          ParagraphFormat: { get: () => strict(path + ".TextFrame2.ParagraphFormat", {
            LeftIndent: rw(p, "leftIndent"), FirstLineIndent: rw(p, "firstIndent"),
          }) },
        });
      },
    }) },
  });
  const frame = strict(path + ".TextFrame", {
    MarginLeft: rw(shape, "marginLeft"), MarginRight: rw(shape, "marginRight"),
    MarginTop: rw(shape, "marginTop"), MarginBottom: rw(shape, "marginBottom"),
    AutoSize: rw(shape, "autoSize"), WordWrap: rw(shape, "wordWrap"), VerticalAnchor: rw(shape, "anchor"),
    TextRange: { get: () => makeTextRange(shape, path + ".TextFrame.TextRange") },
  });
  return strict(path, {
    Name: { get: () => shape.name },
    AutoShapeType: { get: () => shape.autoShapeType },
    Left: rw(shape, "left"), Top: rw(shape, "top"), Width: rw(shape, "width"), Height: rw(shape, "height"),
    Fill: { get: () => strict(path + ".Fill", {
      Visible: rw(shape.fill, "visible"),
      Type: { get: () => shape.fill.type },
      ForeColor: { get: () => strict(path + ".Fill.ForeColor", { RGB: rw(shape.fill, "rgb") }) },
    }, { Solid: () => { shape.fill.type = 1; } }) },
    Line: { get: () => strict(path + ".Line", {
      Visible: rw(shape.line, "visible"), Weight: rw(shape.line, "weight"), DashStyle: rw(shape.line, "dash"),
      ForeColor: { get: () => strict(path + ".Line.ForeColor", { RGB: rw(shape.line, "rgb") }) },
    }) },
    Shadow: { get: () => strict(path + ".Shadow", { Visible: rw(shape, "shadow") }) },
    TextFrame: { get: () => frame },
    TextFrame2: { get: () => frame2 },
  }, {
    Delete: () => { slide.shapes.splice(slide.shapes.indexOf(shape), 1); },
  });
}

function newShape(slide, kind, type, l, t, w, h) {
  slide.seq += 1;
  const auto = kind === "shape";
  const shape = {
    kind, name: (auto ? "AutoShape " : "Text Box ") + slide.seq, autoShapeType: auto ? type : 1,
    left: l, top: t, width: w, height: h,
    fill: auto ? { visible: -1, type: 3, rgb: 0xC47244 } : { visible: 0, type: 1, rgb: 0xFFFFFF },
    line: auto ? { visible: -1, rgb: 0x7F4F2F, weight: 0.75, dash: 1 } : { visible: 0, rgb: 0, weight: 0.75, dash: 1 },
    shadow: auto ? -1 : 0,
    marginLeft: 7.2, marginRight: 7.2, marginTop: 3.6, marginBottom: 3.6, autoSize: 0, wordWrap: -1, anchor: auto ? 3 : 1,
    paragraphs: [],
  };
  shape.paragraphs = [newParagraph("", { size: 18, bold: 0, italic: 0, color: auto ? 0xFFFFFF : 0, font: "Calibri",
    align: auto ? 2 : 1, textShadow: auto ? -1 : 0 })];
  slide.shapes.push(shape);
  return shape;
}

function makeSlide(slide, index) {
  const path = "Slides.Item(" + index + ")";
  return strict(path, {
    SlideIndex: { get: () => index },
    Shapes: { get: () => strict(path + ".Shapes", { Count: { get: () => slide.shapes.length } }, {
      Item: (n) => {
        const shape = slide.shapes[Number(n) - 1];
        if (!shape) { throw new Error("Shapes.Item(" + n + ") ngoai slide"); }
        return makeShape(slide, shape, path + ".Shapes.Item(" + n + ")");
      },
      AddShape: (type, l, t, w, h) => {
        const shape = newShape(slide, "shape", Number(type), l, t, w, h);
        return makeShape(slide, shape, path + ".Shapes.Item(" + slide.shapes.length + ")");
      },
      AddTextbox: (orientation, l, t, w, h) => {
        const shape = newShape(slide, "textbox", 1, l, t, w, h);
        return makeShape(slide, shape, path + ".Shapes.Item(" + slide.shapes.length + ")");
      },
    }) },
  });
}

// options: slides (so slide trang, mac dinh 1), width/height (mac dinh 720x540 nhu default.pptx)
export function makeShapesApp(options = {}) {
  const state = {
    slides: Array.from({ length: options.slides === undefined ? 1 : options.slides }, () => ({ shapes: [], seq: 0 })),
    width: options.width || 720, height: options.height || 540,
  };
  const presentation = strict("ActivePresentation", {
    Name: { get: () => "deck.pptx" },
    FullName: { get: () => "/tmp/deck.pptx" },
    PageSetup: { get: () => strict("PageSetup", {
      SlideWidth: { get: () => state.width }, SlideHeight: { get: () => state.height },
    }) },
    Slides: { get: () => strict("Slides", { Count: { get: () => state.slides.length } }, {
      Item: (n) => {
        const slide = state.slides[Number(n) - 1];
        if (!slide) { throw new Error("Slides.Item(" + n + ") ngoai bai"); }
        return makeSlide(slide, Number(n));
      },
    }) },
  });
  const app = strict("Application", {
    Name: { get: () => "WPS Presentation" },
    UndoRecord: { get: () => undefined },
    ActivePresentation: { get: () => presentation },
  });
  APPS.set(app, state);
  return app;
}

// Trang thai de kiem: slides[i].shapes[j] = {name, kind, autoShapeType, left, top, width, height, fill, line, shadow,
// margin*, anchor, paragraphs: [{text, size, bold, italic, color, font, align, indentLevel, bullet, leftIndent,
// firstIndent, spaceBefore, runs: [{start, length, bold?, color?}]}]}.
export function shapesOf(app) {
  return APPS.get(app).slides;
}
