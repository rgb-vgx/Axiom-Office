/*
 * Axiom Office - WPS JS add-in entry point.
 *
 * Gui mot bao cao JSON do probe (AxiomProbe.collectReport) xay dung ve
 * window.AXIOM_REPORT_URL (do trinh cai ghi trong config.js).
 *
 * ES5 only (CEF Chrome 87 cua WPS): khong let/const/arrow/template strings/
 * optional chaining. Khong bao gio throw ra ngoai, khong dung alert().
 */
(function () {
  "use strict";

  var POLL_MS = 500;
  var TIMEOUT_MS = 30000;

  var loadSent = false;
  var polling = false;

  function reportUrl() {
    try {
      var url = window.AXIOM_REPORT_URL;
      if (typeof url === "string" && url !== "") { return url; }
      return null;
    } catch (e) {
      return null;
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

  function getProbe() {
    try {
      if (window.AxiomProbe && typeof window.AxiomProbe.collectReport === "function") {
        return window.AxiomProbe;
      }
    } catch (e) {}
    try {
      if (typeof AxiomProbe !== "undefined" && AxiomProbe &&
          typeof AxiomProbe.collectReport === "function") {
        return AxiomProbe;
      }
    } catch (e2) {}
    return null;
  }

  function sendReport(trigger) {
    try {
      var url = reportUrl();
      if (!url) { return; }
      var report;
      try {
        var probe = getProbe();
        if (probe) {
          report = probe.collectReport(getApp(), window);
        } else {
          report = { loaded: true, wpsVersion: "", undoRecord: false,
            errors: ["main: AxiomProbe is not available"] };
        }
      } catch (err) {
        report = { loaded: true, wpsVersion: "", undoRecord: false, errors: [] };
        try {
          report.errors.push("main.collectReport: " + String((err && err.message) || err));
        } catch (e2) {}
      }
      try {
        report.trigger = trigger;
      } catch (e3) {}
      var text;
      try {
        text = JSON.stringify(report);
      } catch (e4) {
        return;
      }
      try {
        var xhr = new XMLHttpRequest();
        xhr.open("POST", url, true);
        // text/plain tranh CORS preflight (Origin la file://).
        xhr.setRequestHeader("Content-Type", "text/plain");
        xhr.send(text);
      } catch (e5) {}
    } catch (e6) {}
  }

  function docReady() {
    try {
      var app = getApp();
      if (!app) { return false; }
      var doc = null;
      try {
        doc = app.ActiveDocument;
      } catch (e) {
        return false;
      }
      return doc !== undefined && doc !== null;
    } catch (e2) {
      return false;
    }
  }

  function waitAndSendOnce() {
    if (loadSent) { return; }
    if (docReady()) {
      loadSent = true;
      sendReport("load");
      return;
    }
    if (polling) { return; }
    polling = true;
    var start = (new Date()).getTime();
    var timer = setInterval(function () {
      try {
        if (loadSent) {
          try { clearInterval(timer); } catch (e) {}
          polling = false;
          return;
        }
        if (docReady()) {
          loadSent = true;
          try { clearInterval(timer); } catch (e2) {}
          polling = false;
          sendReport("load");
          return;
        }
        if (((new Date()).getTime() - start) >= TIMEOUT_MS) {
          loadSent = true;
          try { clearInterval(timer); } catch (e3) {}
          polling = false;
          sendReport("load");
        }
      } catch (e4) {}
    }, POLL_MS);
  }

  function OnAddinLoad() {
    try {
      waitAndSendOnce();
    } catch (e) {}
    return true;
  }

  function OnAction() {
    try {
      sendReport("button");
    } catch (e) {}
  }

  try {
    window.ribbon = { OnAddinLoad: OnAddinLoad, OnAction: OnAction };
  } catch (e) {}
})();
