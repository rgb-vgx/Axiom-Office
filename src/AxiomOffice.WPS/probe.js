/*
 * Axiom Office - WPS JS add-in probe.
 *
 * collectReport(app, win) -> plain object describing what the WPS JS environment offers
 * (version, UndoRecord, globals, transports). Never throws: each failing step is recorded
 * in report.errors as "<step>: <message>".
 *
 * ES5 only (the embedded CEF of WPS may be old): no optional chaining, arrow functions,
 * let/const, async/await or template strings.
 * Browser: defines global AxiomProbe. Node: module.exports = { collectReport }.
 */
(function (root) {
  "use strict";

  var UNDO_LABEL = "Axiom probe";
  var GLOBAL_NAMES = ["Application", "wps", "WpsInvoke", "cef", "external"];

  function message(err) {
    try {
      if (err && typeof err.message === "string" && err.message) { return err.message; }
      return String(err);
    } catch (e) {
      return "unknown error";
    }
  }

  // Reads obj[name]; some WPS/COM bridges expose properties as methods, so call it when it is
  // a function and callWhenFunction is set.
  function getProp(obj, name, callWhenFunction) {
    var value = obj[name];
    if (callWhenFunction && typeof value === "function") { value = value.call(obj); }
    return value;
  }

  function isPresent(value) {
    return value !== undefined && value !== null;
  }

  function collectReport(app, win) {
    var report = {
      loaded: true,
      wpsVersion: "",
      appName: "",
      undoRecord: false,
      undoRecordTried: false,
      undoRecordMethods: false,
      activeDocument: null,
      globals: {},
      transport: { fetch: false, xhr: false, webSocket: false },
      location: "",
      userAgent: "",
      errors: []
    };

    function step(name, fn) {
      try {
        return fn();
      } catch (err) {
        report.errors.push(name + ": " + message(err));
        return undefined;
      }
    }

    var hasApp = isPresent(app);
    if (!hasApp) {
      report.errors.push("app: Application object is " + (app === null ? "null" : "undefined"));
    }

    if (hasApp) {
      step("version", function () {
        var v = getProp(app, "Version", true);
        if (!isPresent(v) || String(v) === "") { v = getProp(app, "Build", true); }
        report.wpsVersion = isPresent(v) ? String(v) : "";
      });

      step("appName", function () {
        var n = getProp(app, "Name", true);
        report.appName = isPresent(n) ? String(n) : "";
      });

      var doc = step("activeDocument", function () {
        var d = getProp(app, "ActiveDocument", false);
        if (!isPresent(d)) { return null; }
        report.activeDocument = {
          name: step("activeDocument.Name", function () {
            var n = getProp(d, "Name", true);
            return isPresent(n) ? String(n) : "";
          }) || "",
          fullName: step("activeDocument.FullName", function () {
            var n = getProp(d, "FullName", true);
            return isPresent(n) ? String(n) : "";
          }) || ""
        };
        return d;
      });

      var hasDocument = isPresent(doc);
      if (!hasDocument) {
        hasDocument = step("documents.Count", function () {
          var docs = getProp(app, "Documents", false);
          if (!isPresent(docs)) { return false; }
          var count = getProp(docs, "Count", true);
          return Number(count) > 0;
        }) === true;
      }

      step("undoRecord", function () {
        var ur = getProp(app, "UndoRecord", false);
        if (!isPresent(ur)) {
          report.errors.push("undoRecord: Application.UndoRecord is not available");
          return;
        }
        // Doi tuong native cua WPS co the khong bao typeof "function" cho method: ghi lai typeof that, va cu co
        // thuoc tinh la thu goi (loi da duoc try/catch ghi vao errors) de khong bao sai undoRecord=false.
        report.undoRecordTypes = { start: typeof ur.StartCustomRecord, end: typeof ur.EndCustomRecord };
        var hasMethods = isPresent(ur.StartCustomRecord) && isPresent(ur.EndCustomRecord);
        report.undoRecordMethods = hasMethods;
        if (!hasDocument) {
          // No document open: only report whether the API surface exists (undoRecord stays false).
          report.undoRecordTried = false;
          if (!hasMethods) {
            report.errors.push("undoRecord: StartCustomRecord/EndCustomRecord missing");
          }
          return;
        }
        if (!hasMethods) {
          report.errors.push("undoRecord: StartCustomRecord/EndCustomRecord missing");
          return;
        }
        report.undoRecordTried = true;
        var started = false;
        var ended = false;
        try {
          ur.StartCustomRecord(UNDO_LABEL);
          started = true;
        } catch (err) {
          report.errors.push("undoRecord.StartCustomRecord: " + message(err));
        } finally {
          if (started) {
            try {
              ur.EndCustomRecord();
              ended = true;
            } catch (err2) {
              report.errors.push("undoRecord.EndCustomRecord: " + message(err2));
            }
          }
        }
        report.undoRecord = started && ended;
      });
    }

    if (isPresent(win)) {
      for (var i = 0; i < GLOBAL_NAMES.length; i++) {
        (function (name) {
          report.globals[name] = step("globals." + name, function () {
            return isPresent(win[name]);
          }) === true;
        })(GLOBAL_NAMES[i]);
      }
      report.transport.fetch = step("transport.fetch", function () {
        return typeof win.fetch === "function";
      }) === true;
      report.transport.xhr = step("transport.xhr", function () {
        return isPresent(win.XMLHttpRequest);
      }) === true;
      report.transport.webSocket = step("transport.webSocket", function () {
        return isPresent(win.WebSocket);
      }) === true;
      step("location", function () {
        if (win.location && isPresent(win.location.href)) { report.location = String(win.location.href); }
      });
      step("userAgent", function () {
        if (win.navigator && isPresent(win.navigator.userAgent)) {
          report.userAgent = String(win.navigator.userAgent);
        }
      });
    } else {
      for (var j = 0; j < GLOBAL_NAMES.length; j++) { report.globals[GLOBAL_NAMES[j]] = false; }
      report.errors.push("win: window object is " + (win === null ? "null" : "undefined"));
    }

    return report;
  }

  var api = { collectReport: collectReport };
  if (typeof module !== "undefined" && module && module.exports) {
    module.exports = api;
  } else if (root) {
    root.AxiomProbe = api;
  }
})(typeof window !== "undefined" ? window : (typeof globalThis !== "undefined" ? globalThis : this));
