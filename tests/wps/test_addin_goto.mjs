// Trich dan bam duoc (docs/research/tinh-nang-con-thieu-so-voi-claude-m365.md W1): pane WPS bam [§12] / [Data!C42] /
// [slide 3] -> /wps/v1/exec writer.goto / et.goto / wpp.goto. Chi chon + cuon, khong sua tai lieu.
import { test } from "node:test";
import assert from "node:assert/strict";

import { loadAddin, run } from "./addin_harness.mjs";

function writerApp(texts) {
  const log = [];
  const paragraphs = texts.map((text, i) => ({
    Range: { Text: text + "\r", Select() { log.push("select " + (i + 1)); } },
  }));
  return {
    log,
    app: {
      Name: "WPS Writer",
      ActiveDocument: { Paragraphs: { Count: paragraphs.length, Item: (n) => paragraphs[n - 1] } },
      ActiveWindow: { ScrollIntoView() { log.push("scroll"); } },
    },
  };
}

test("writer.goto: chon dung doan theo so cua writer.outline va cuon toi", () => {
  const ctx = loadAddin();
  const { app, log } = writerApp(["Tiêu đề", "Mở đầu", "Điều 1. Phạm vi"]);
  const result = run(ctx, app, "writer.goto", { paragraph: 3 });
  assert.equal(result.paragraph, 3);
  assert.equal(result.text, "Điều 1. Phạm vi");
  assert.deepEqual(log, ["select 3", "scroll"]);
});

test("writer.goto: so doan lech sau khi tai lieu doi -> do theo chu dau doan", () => {
  const ctx = loadAddin();
  const { app, log } = writerApp(["Tiêu đề", "Đoạn mới chèn", "Mở đầu", "Điều 1. Phạm vi"]);
  const result = run(ctx, app, "writer.goto", { paragraph: 3, text: "Điều 1" });
  assert.equal(result.paragraph, 4);
  assert.deepEqual(log, ["select 4", "scroll"]);
});

test("writer.goto: so doan ngoai tai lieu -> loi ro rang", () => {
  const ctx = loadAddin();
  const { app } = writerApp(["a"]);
  const reply = ctx.AxiomCommands.execute(app, "writer.goto", { paragraph: 9 });
  assert.equal(reply.ok, false);
  assert.match(reply.error, /between 1 and 1/);
});

test("et.goto: nhan 'Sheet!A1' va 'Ten co dau cach'!A1:B2, kich hoat sheet roi chon vung", () => {
  const ctx = loadAddin();
  const log = [];
  const sheet = (name) => ({
    Name: name,
    Activate() { log.push("activate " + name); },
    Range(address) { return { Select() { log.push("select " + name + "!" + address); } }; },
  });
  const sheets = [sheet("Data"), sheet("Báo cáo Q3")];
  const app = {
    Name: "WPS Spreadsheets",
    ActiveSheet: sheets[0],
    ActiveWorkbook: {
      Worksheets: { Count: sheets.length, Item: (n) => sheets[n - 1] },
      Sheets: { Count: sheets.length, Item: (n) => sheets[n - 1] },
    },
  };
  let result = run(ctx, app, "et.goto", { range: "Data!C42" });
  assert.deepEqual(result, { sheet: "Data", range: "C42" });
  result = run(ctx, app, "et.goto", { range: "'Báo cáo Q3'!$A$1:B2" });
  assert.deepEqual(result, { sheet: "Báo cáo Q3", range: "A1:B2" });
  assert.deepEqual(log, ["activate Data", "select Data!C42", "activate Báo cáo Q3", "select Báo cáo Q3!A1:B2"]);
});

test("wpp.goto: chuyen toi slide, ngoai so slide thi loi", () => {
  const ctx = loadAddin();
  const log = [];
  const app = {
    Name: "WPS Presentation",
    ActivePresentation: { Slides: { Count: 5, Item: () => ({}) } },
    ActiveWindow: { View: { GotoSlide(n) { log.push("goto " + n); } } },
  };
  assert.deepEqual(run(ctx, app, "wpp.goto", { slide: 3 }), { slide: 3 });
  assert.deepEqual(log, ["goto 3"]);
  assert.equal(ctx.AxiomCommands.execute(app, "wpp.goto", { slide: 6 }).ok, false);
});
