// Hop dong poll add-in WPS -> Core: body POST /wps/v1/poll ma main.js THAT gui, ghi thanh file mau dung chung voi Go
// (core-go/internal/wpsbridge/contract_test.go doc chinh cac file nay, day qua bridge roi doc lai bang
// office.BridgeClient: LiveSession, Health, GetCommandsFor). Hai dau cua hop dong doc CUNG mot file, nen doi dang du
// lieu o mot ben ma quen ben kia thi mot trong hai test do (01/11/2026: LiveSession doc `document` dang chuoi trong khi
// add-in gui object {name, fullName} - other_document khong bao gio thay tai lieu WPS khac, khong test nao bat).
//
//   node --test tests/wps/test_contract_poll.mjs
//   AXIOM_UPDATE_CONTRACT=1 node --test tests/wps/test_contract_poll.mjs   # ghi lai file mau sau khi doi body poll
//
// File mau giu: moi truong cua body (clientId/version thay bang chuoi co dinh) va 3 lenh dau cua `commands` - du de
// Go kiem catalog, khong phai ghi lai moi lan them lenh.

import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { loadAddin } from "./addin_harness.mjs";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const DIR = path.join(HERE, "contract");

const APPS = {
  wps: { Name: "WPS Docs", Version: "12.0", ActiveDocument: { Name: "hop-dong.docx", FullName: "/home/u/hop-dong.docx" } },
  et: { Name: "WPS Spreadsheets", Version: "12.0", ActiveWorkbook: { Name: "ban-hang.xlsx", FullName: "/home/u/ban-hang.xlsx" },
    ActiveSheet: { Name: "Sheet1" } },
  wpp: { Name: "WPS Presentation", Version: "12.0", ActivePresentation: { Name: "bao-cao.pptx", FullName: "/home/u/bao-cao.pptx" } },
};

async function firstPoll(app) {
  const polls = [];
  const fetch = (url, options = {}) => {
    if (String(url).endsWith("/wps/v1/poll")) {
      polls.push(JSON.parse(options.body));
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ ok: true, result: { jobs: [] } }) });
    }
    return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ ok: true }) });
  };
  const ctx = loadAddin({ fetch, Application: app, location: { href: "http://127.0.0.1:47840/wps/addin/index.html" } });
  try {
    ctx.ribbon.OnAddinLoad({});
    const deadline = Date.now() + 2000;
    while (polls.length < 1 && Date.now() < deadline) { await new Promise((resolve) => setTimeout(resolve, 10)); }
  } finally {
    ctx.AxiomBridge.stop();
  }
  assert.ok(polls.length >= 1, "add-in phai poll");
  return polls[0];
}

function normalize(body, kind) {
  assert.match(String(body.clientId), /\S/, "clientId khong rong");
  const out = JSON.parse(JSON.stringify(body));
  out.clientId = "client-" + kind;
  out.version = "<version>";
  out.commands = out.commands.slice(0, 3);
  return out;
}

for (const kind of Object.keys(APPS)) {
  test("poll " + kind + ": body cua main.js khop file mau tests/wps/contract/poll-" + kind + ".json", async () => {
    const body = normalize(await firstPoll(APPS[kind]), kind);
    const file = path.join(DIR, "poll-" + kind + ".json");
    if (process.env.AXIOM_UPDATE_CONTRACT === "1") {
      fs.mkdirSync(DIR, { recursive: true });
      fs.writeFileSync(file, JSON.stringify(body, null, 2) + "\n");
    }
    const golden = JSON.parse(fs.readFileSync(file, "utf8"));
    assert.deepEqual(body, golden, "body poll doi: chay lai voi AXIOM_UPDATE_CONTRACT=1 VA sua ben Go doc no " +
      "(core-go/internal/wpsbridge/contract_test.go phai van xanh)");
    assert.equal(golden.commands.length, 3);
  });
}
