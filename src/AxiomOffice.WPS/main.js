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
  var taskPane = null;

  function paneUrl() {
    try {
      var href = "";
      try { href = (window.location && window.location.href) || ""; } catch (e) { href = ""; }
      var idx = String(href).lastIndexOf("/");
      if (idx >= 0) {
        return String(href).substring(0, idx + 1) + "pane.html";
      }
    } catch (e2) {}
    return "pane.html";
  }

  function openPane() {
    try {
      var app = getApp();
      if (!app) { return false; }
      if (taskPane) {
        try { taskPane.Visible = true; } catch (e) {}
        return true;
      }
      var pane = null;
      try {
        pane = app.CreateTaskPane(paneUrl());
      } catch (e2) {
        return false;
      }
      if (!pane) { return false; }
      taskPane = pane;
      try { taskPane.Visible = true; } catch (e3) {}
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

  function getCommands() {
    try {
      if (window.AxiomCommands && typeof window.AxiomCommands.catalog === "function") {
        return window.AxiomCommands.catalog();
      }
    } catch (e) {}
    try {
      if (typeof AxiomCommands !== "undefined" && AxiomCommands &&
          typeof AxiomCommands.catalog === "function") {
        return AxiomCommands.catalog();
      }
    } catch (e2) {}
    return [];
  }

  function currentDocument() {
    try {
      var app = getApp();
      if (!app) { return null; }
      var doc = app.ActiveDocument;
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
        app: "wps",
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

  function OnAction(control) {
    try {
      var id = "";
      try { id = (control && control.Id) || ""; } catch (e) { id = ""; }
      if (id === "axiomAskAI") {
        openPane();
        try { start(); } catch (e2) {}
        return;
      }
      start();
    } catch (e3) {}
  }

  try {
    window.ribbon = { OnAddinLoad: OnAddinLoad, OnAction: OnAction };
  } catch (e) {}
  try {
    window.AxiomBridge = { start: start, stop: stop };
  } catch (e2) {}
})();
