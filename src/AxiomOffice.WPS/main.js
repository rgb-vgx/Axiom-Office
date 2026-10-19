/*
 * Axiom Office - WPS JS add-in entry point (bridge long-poll).
 *
 * Trang add-in do Agent Core phuc vu (http://127.0.0.1:<core>/wps/addin/index.html):
 * vong lap poll -> thuc thi tung job -> gui result -> poll tiep.
 *
 * ES5 only (CEF Chrome 87): khong let/const/arrow/template strings/
 * optional chaining. Khong bao gio alert(), khong bao gio throw ra ngoai.
 */
(function () {
  "use strict";

  var VERSION = "0.1.0";
  var CLIENT = "wps-jsaddon";
  var RETRY_MS = 2000;

  var running = false;

  // Mot task pane cho MOI man hinh (khoa la ten man hinh, "" = khung chat). Khung chat phai dung lai
  // pane cu, neu khong moi lan bam Ask AI la mat doan hoi thoai dang do; cac man hinh phu thi moi man
  // hinh mot pane rieng de bam qua lai giua chung khong phai nap lai trang.
  //
  // Duong vao mot man hinh phu: ?view=<ten> (pane.html doc tham so nay luc nap - xem pane.html: VIEW).
  // Du lieu cua man hinh phu co the cu sau mot thoi gian: moi man hinh deu co nut "Tai lai" rieng, va
  // do la cach duy nhat - tu ribbon khong co kenh nao noi vao trang dang mo trong CEF (xem
  // docs/ribbon-ui.md muc 7).
  var panes = {};

  function paneUrl(view, check, action) {
    var kind = "wps";
    try { kind = currentAppKind() || "wps"; } catch (eK) { kind = "wps"; }
    var suffix = "";
    if (view) {
      suffix = "&view=" + view;
      // Man hinh "Soat ..." can biet DUNG lenh nao: ba nut soat la ba lenh khac nhau, moi lenh mot app.
      if (check) { suffix += "&check=" + check; }
      // Viec man hinh phai lam ngay khi mo (vi du "Kiem tra mo hinh" tren ribbon: bam la chay luon,
      // giong ban Windows chu khong phai mo man hinh roi bat nguoi dung bam them mot lan nua).
      if (action) { suffix += "&do=" + action; }
    }
    try {
      var href = "";
      try { href = (window.location && window.location.href) || ""; } catch (e) { href = ""; }
      var clean = String(href).split("#")[0].split("?")[0];
      var idx = clean.lastIndexOf("/");
      if (idx >= 0) {
        return clean.substring(0, idx + 1) + "pane.html?app=" + kind + suffix;
      }
    } catch (e2) {}
    return "pane.html?app=" + kind + suffix;
  }

  // An moi pane khac: nguoi dung chi lam viec voi mot man hinh mot luc, va mot pane bi bo quen ma van
  // hien thi thi khong con la "khung chat ben canh tai lieu" nua.
  function showOnly(key) {
    var names = [];
    for (var name in panes) {
      if (Object.prototype.hasOwnProperty.call(panes, name)) { names.push(name); }
    }
    for (var i = 0; i < names.length; i++) {
      if (names[i] === key) { continue; }
      try { panes[names[i]].Visible = false; } catch (e) {}
    }
  }

  function openPane(view, check, action) {
    try {
      var app = getApp();
      if (!app) { return false; }
      // Khoa gom ca ten lenh soat: neu WPS bo qua getVisible thi ca ba nut soat cung hien, va moi nut
      // phai giu dung lenh cua no chu khong dung lai pane cua nut vua bam.
      var key = view ? (check ? view + ":" + check : view) : "";
      var known = null;
      try { known = panes[key] || null; } catch (e0) { known = null; }
      // Nut mang theo viec-phai-lam (?do=) thi KHONG dung lai pane cu: "Kiem tra mo hinh" bam lan hai
      // phai chay lai that, khong phai mo lai ket qua cu. Pane cu duoc an di truoc khi thay.
      if (known && !action) {
        try { known.Visible = true; } catch (e) {}
        showOnly(key);
        return true;
      }
      if (known) {
        try { known.Visible = false; } catch (e1) {}
      }
      var pane = null;
      try {
        pane = app.CreateTaskPane(paneUrl(view, check, action));
      } catch (e2) {
        return false;
      }
      if (!pane) { return false; }
      panes[key] = pane;
      try { pane.Visible = true; } catch (e3) {}
      showOnly(key);
      return true;
    } catch (e4) {
      return false;
    }
  }

  function getApp() {
    try {
      var app = window.Application;
      if (app === undefined || app === null) { return null; }
      return app;
    } catch (e) {
      return null;
    }
  }

  function currentAppKind() {
    try {
      var app = getApp();
      try {
        if (window.AxiomCommands && typeof window.AxiomCommands.appKind === "function") {
          return window.AxiomCommands.appKind(app);
        }
      } catch (e) {}
      try {
        if (typeof AxiomCommands !== "undefined" && AxiomCommands &&
            typeof AxiomCommands.appKind === "function") {
          return AxiomCommands.appKind(app);
        }
      } catch (e2) {}
    } catch (e3) {}
    return "wps";
  }

  function hasHandlerOf(name) {
    try {
      if (window.AxiomCommands && typeof window.AxiomCommands.hasHandler === "function") {
        return window.AxiomCommands.hasHandler(name);
      }
    } catch (e) {}
    try {
      if (typeof AxiomCommands !== "undefined" && AxiomCommands &&
          typeof AxiomCommands.hasHandler === "function") {
        return AxiomCommands.hasHandler(name);
      }
    } catch (e2) {}
    return true;
  }

  // CATALOG la hop dong day du registry (ke ca lenh chua co handler): chi khai voi Core
  // nhung lenh add-in chay duoc, khong thi agent duoc moi goi tool luon that bai.
  function filterRunnable(list) {
    var out = [];
    for (var i = 0; i < list.length; i++) {
      try {
        if (hasHandlerOf(list[i].name)) { out.push(list[i]); }
      } catch (e) {}
    }
    return out;
  }

  function getCommands() {
    try {
      var kind = "wps";
      try { kind = currentAppKind(); } catch (eK) { kind = "wps"; }
      if (window.AxiomCommands && typeof window.AxiomCommands.catalogFor === "function") {
        return filterRunnable(window.AxiomCommands.catalogFor(kind));
      }
      if (window.AxiomCommands && typeof window.AxiomCommands.catalog === "function") {
        return filterRunnable(window.AxiomCommands.catalog());
      }
    } catch (e) {}
    try {
      var kind2 = "wps";
      try { kind2 = currentAppKind(); } catch (eK2) { kind2 = "wps"; }
      if (typeof AxiomCommands !== "undefined" && AxiomCommands &&
          typeof AxiomCommands.catalogFor === "function") {
        return filterRunnable(AxiomCommands.catalogFor(kind2));
      }
      if (typeof AxiomCommands !== "undefined" && AxiomCommands &&
          typeof AxiomCommands.catalog === "function") {
        return filterRunnable(AxiomCommands.catalog());
      }
    } catch (e2) {}
    return [];
  }

  function currentDocument() {
    try {
      var app = getApp();
      if (!app) { return null; }
      var kind = "wps";
      try { kind = currentAppKind(); } catch (eK) { kind = "wps"; }
      var doc = null;
      try {
        if (kind === "et") { doc = app.ActiveWorkbook; }
        else if (kind === "wpp") { doc = app.ActivePresentation; }
        else { doc = app.ActiveDocument; }
      } catch (eD) { return null; }
      if (doc === undefined || doc === null) { return null; }
      var name = "", fullName = "";
      try {
        var n = doc.Name;
        if (typeof n === "function") { n = n.call(doc); }
        if (n !== undefined && n !== null) { name = String(n); }
      } catch (e) {}
      try {
        var f = doc.FullName;
        if (typeof f === "function") { f = f.call(doc); }
        if (f !== undefined && f !== null) { fullName = String(f); }
      } catch (e2) {}
      if (name === "" && fullName === "") { return null; }
      return { name: name, fullName: fullName };
    } catch (e3) {
      return null;
    }
  }

  function endpoint(kind) {
    try {
      var href = "";
      try { href = (window.location && window.location.href) || ""; } catch (e) { href = ""; }
      var marker = "/wps/addin/";
      var idx = String(href).indexOf(marker);
      if (idx >= 0) {
        return String(href).substring(0, idx) + "/wps/v1/" + kind;
      }
    } catch (e2) {}
    return "../v1/" + kind;
  }

  function pollUrl() { return endpoint("poll"); }
  function resultUrl() { return endpoint("result"); }

  function postJson(url, body) {
    var text = "";
    try { text = JSON.stringify(body); } catch (e) { text = "{}"; }
    return window.fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: text
    });
  }

  function readJson(resp) {
    try {
      if (resp && typeof resp.json === "function") { return resp.json(); }
    } catch (e) {}
    try {
      if (resp && typeof resp.text === "function") {
        return resp.text().then(function (t) {
          try { return JSON.parse(t); } catch (e2) { return null; }
        });
      }
    } catch (e3) {}
    return Promise.resolve(null);
  }

  function runJob(job) {
    var out;
    try {
      var executor = null;
      try {
        if (window.AxiomCommands && typeof window.AxiomCommands.execute === "function") {
          executor = window.AxiomCommands;
        }
      } catch (e) {}
      if (!executor) {
        try {
          if (typeof AxiomCommands !== "undefined" && AxiomCommands &&
              typeof AxiomCommands.execute === "function") {
            executor = AxiomCommands;
          }
        } catch (e2) {}
      }
      if (!executor) {
        out = { ok: false, error: "AxiomCommands is not available" };
      } else {
        out = executor.execute(getApp() || window.Application, job.action, job.params || {});
      }
    } catch (err) {
      var msg = "unknown error";
      try { msg = String((err && err.message) || err); } catch (e3) {}
      out = { ok: false, error: msg };
    }
    if (!out || typeof out !== "object") {
      out = { ok: false, error: "handler returned nothing" };
    }
    var payload = { id: job.id, ok: !!out.ok };
    if (out.ok) {
      payload.result = (out.result === undefined) ? null : out.result;
    } else {
      var emsg = "unknown error";
      try { emsg = String(out.error || "unknown error"); } catch (e4) {}
      payload.error = emsg;
    }
    return postJson(resultUrl(), payload).then(
      function () { return null; },
      function () { return null; }
    );
  }

  function runJobsSequentially(jobs, index) {
    if (!running) { return Promise.resolve(null); }
    if (index >= jobs.length) { return Promise.resolve(null); }
    var job = jobs[index] || {};
    if (!job.id || !job.action) {
      return runJobsSequentially(jobs, index + 1);
    }
    var done;
    try {
      done = runJob(job);
    } catch (e) {
      done = Promise.resolve(null);
    }
    return done.then(function () {
      return runJobsSequentially(jobs, index + 1);
    });
  }

  function schedule(ms) {
    if (!running) { return; }
    try {
      setTimeout(function () { pollOnce(); }, ms);
    } catch (e) {}
  }

  function pollOnce() {
    if (!running) { return; }
    var body;
    try {
      body = {
        client: CLIENT,
        app: currentAppKind(),
        version: VERSION,
        document: currentDocument(),
        commands: getCommands()
      };
    } catch (e) {
      schedule(RETRY_MS);
      return;
    }
    var req;
    try {
      req = postJson(pollUrl(), body);
    } catch (e2) {
      schedule(RETRY_MS);
      return;
    }
    req.then(function (resp) {
      if (!running) { return null; }
      return readJson(resp);
    }).then(function (data) {
      if (!running) { return null; }
      if (!data) { schedule(RETRY_MS); return null; }
      var payload = data.result !== undefined ? data.result : data;
      var jobs = (payload && payload.jobs) || [];
      if (!Array.isArray(jobs)) { jobs = []; }
      return runJobsSequentially(jobs, 0).then(function () {
        if (running) { schedule(0); }
      });
    }).then(null, function () {
      schedule(RETRY_MS);
    });
  }

  function start() {
    try {
      if (running) { return; }
      if (typeof window.fetch !== "function") { return; }
      running = true;
      pollOnce();
    } catch (e) {}
  }

  function stop() {
    try { running = false; } catch (e) {}
  }

  function OnAddinLoad() {
    try { start(); } catch (e) {}
    return true;
  }

  // Nut nao mo man hinh nao (tag trong ribbon.xml -> tham so ?view= cua pane.html). Nut khong co o
  // day (vi du mot id la do WPS truyen xuong) thi chi mo khung chat - khong im lang bo qua.
  var VIEWS = {
    askAi: "",
    memory: "memory",
    skills: "skills",
    setup: "setup",
    status: "status",
    openLog: "log",
    checkLayout: "check",
    checkTables: "check",
    checkRange: "check",
    testModel: "status",
    copyUrl: "status"
  };

  // Viec man hinh phai lam ngay khi vua mo (gui qua ?do=): hai muc trong menu "Khac" khong phai la
  // "mo mot man hinh" ma la "lam mot viec roi cho xem ket qua" - giong ban Windows.
  var DO = { testModel: "testModel", copyUrl: "copyUrl" };

  // Id -> tag: dung khi host chi tra ve Id (mot so ban WPS khong dien control.Tag).
  var TAGS_BY_ID = {
    axiomAskAI: "askAi",
    axiomCheckLayout: "checkLayout",
    axiomCheckTables: "checkTables",
    axiomCheckRange: "checkRange",
    axiomMemory: "memory",
    axiomSkills: "skills",
    axiomSetup: "setup",
    axiomStatus: "status",
    axiomLog: "openLog",
    axiomTestModel: "testModel",
    axiomCopyUrl: "copyUrl",
    axiomMoreMenu: "more"
  };

  // Nut "Soat ..." nao ung voi lenh nao: pane nhan ten tag qua ?check= roi tra cuu o day, de mot nut
  // bam sai app van chay DUNG lenh cua no (add-in tra loi ro "lenh nay can ung dung kia").
  var CHECK_COMMANDS = {
    checkLayout: "wpp.checkLayout",
    checkTables: "writer.checkTables",
    checkRange: "et.checkRange"
  };

  // Kieu app ma moi nut soat thuoc ve (giong GetVisible cua ban Windows: tag -> Kind).
  var CHECK_KINDS = { checkLayout: "wpp", checkTables: "wps", checkRange: "et" };

  // ---------------------------------------------------------------- icon tren ribbon
  //
  // Ban Windows ve icon cho moi nut (imageMso= trong RibbonUi.cs). Ban WPS cung co icon, nhung KHONG
  // nhan data URI: getImage phai tra ve DUONG DAN TEP tuong doi so voi goc add-in. Do dung la cach SDK
  // chinh thuc cua WPS lam - @wps-jsapi/wps/js/ribbon.js co `return "images/1.svg"`, va bo tep do ship
  // kem images/1.svg (mot the <svg> chi co viewBox, khong width/height).
  //
  // Goc add-in o day la <core>/wps/addin/ (publish.xml: jspluginonline url=...), nen "images/askAi.svg"
  // thanh <core>/wps/addin/images/askAi.svg - Core phuc vu bang ServeAddin, va ContentTypeFor da co san
  // nhanh .svg -> image/svg+xml.
  //
  // Tep anh nam o src/AxiomOffice.WPS/images/<tag>.svg, mot tep cho moi tag trong ribbon.xml. Ten tep
  // CHINH LA tag, nen khong co bang tra nao phai giu cho khop voi ribbon.xml.
  // tests/wps/test_pane.mjs van doi chieu hai ben: them nut ma quen tep anh, hoac de lai tep anh khong
  // nut nao dung, deu la spec do ngay.
  //
  // MAU: dung dung ACCENT cua he thiet ke pane (theme.py ACCENT = 0x0F6CBD) de ribbon va pane trong nhu
  // cung mot san pham. Doi mau thi phai sua ca 12 tep trong images/ - spec kiem mau tung tep.
  var ICON_DIR = "images/";

  function tagOf(control) {
    var tag = "";
    try { tag = (control && control.Tag) || ""; } catch (e) { tag = ""; }
    if (tag !== "") { return String(tag); }
    var id = "";
    try { id = (control && control.Id) || ""; } catch (e2) { id = ""; }
    id = String(id);
    if (Object.prototype.hasOwnProperty.call(TAGS_BY_ID, id)) { return TAGS_BY_ID[id]; }
    return "";
  }

  function checkCommandOf(tag) {
    if (Object.prototype.hasOwnProperty.call(CHECK_COMMANDS, tag)) { return CHECK_COMMANDS[tag]; }
    return "";
  }

  // getVisible: an hai nut soat khong dung cho app dang mo. Chi de DON DEP - WPS co the khong goi
  // callback nay, nen moi ket qua sai o day khong duoc lam nut bien mat oan: khong nhan ra app thi
  // tra true (hien het), va bam sai app thi da co thong bao ro rang tu add-in.
  function OnGetVisible(control) {
    try {
      var tag = tagOf(control);
      if (!Object.prototype.hasOwnProperty.call(CHECK_KINDS, tag)) { return true; }
      var kind = "";
      try { kind = currentAppKind() || ""; } catch (eK) { kind = ""; }
      if (kind === "" || kind === "unknown") { return true; }
      return kind === CHECK_KINDS[tag];
    } catch (e) {
      return true;
    }
  }

  // getImage: WPS hoi anh cho tung control co getImage= trong ribbon.xml, va doi mot DUONG DAN tuong doi
  // (khong phai data URI - xem chu thich o dau muc "icon tren ribbon"). Ten tep chinh la tag nen chi con
  // viec ghep duong dan; spec test bao dam moi tag trong ribbon.xml deu co tep images/<tag>.svg.
  //
  // Khong biet tag thi tra chuoi rong: WPS ve nut khong co anh, dung nhu ban cu. Tra ve mot duong dan
  // khong ton tai thi con te hon - WPS ve dau "?" (dung cai da gap khi tra data URI).
  function OnGetImage(control) {
    try {
      var tag = tagOf(control);
      if (tag !== "") { return ICON_DIR + tag + ".svg"; }
    } catch (e) {}
    return "";
  }

  function OnAction(control) {
    try {
      var tag = tagOf(control);
      // Bam vao mot id la: coi nhu mo khung chat, dung hanh vi cu.
      if (!Object.prototype.hasOwnProperty.call(VIEWS, tag)) {
        openPane("", "", "");
        try { start(); } catch (e0) {}
        return;
      }
      openPane(VIEWS[tag], checkCommandOf(tag), DO[tag] || "");
      // Pane can add-in dang poll moi goi duoc /wps/v1/exec (nut Soat, Hoan tac, Chen) - mo pane thi
      // bao dam vong poll dang chay, ke ca khi OnAddinLoad da chay truoc do.
      try { start(); } catch (e2) {}
    } catch (e3) {}
  }

  try {
    window.ribbon = { OnAddinLoad: OnAddinLoad, OnAction: OnAction, OnGetVisible: OnGetVisible,
      OnGetImage: OnGetImage };
  } catch (e) {}
  try {
    window.AxiomBridge = { start: start, stop: stop };
  } catch (e2) {}
})();
