/*
 * Axiom Office - WPS Ask AI task pane logic (khong DOM).
 *
 * Dinh nghia global AxiomPaneCore.create(options):
 *   options = { fetch, EventSource, base: "<...>/wps/v1/", onChange(state) }
 *   state   = { messages: [{role: "user"|"assistant"|"error", text}],
 *               running, steps: [{action, ok}],
 *               confirmation: null | {id, action, reason}, conversationId }
 *
 * controller.send(prompt)    -> POST base+"runs" {prompt, conversationId?} roi mo
 *                               EventSource(base+"runs/<id>/events"); dang chay thi bo qua (tra false).
 *                               409 (add-in chua ket noi) -> message "error" tieng Viet.
 * controller.confirm(ok)     -> POST base+"runs/<id>/confirm" {confirmationId, approved: ok}
 * controller.stop()          -> POST base+"runs/<id>/cancel"
 * controller.newConversation() -> xoa conversationId va messages.
 *
 * Su kien SSE (MessageEvent ma .data la chuoi JSON cua {type?, data: {...}}):
 * doc payload tu truong "data" (fallback: chinh object do).
 * tool.started -> steps += {action}; tool.finished -> step.ok;
 * confirm.required -> confirmation; confirm.resolved -> confirmation = null;
 * run.completed -> message assistant = reply, running = false, dong EventSource;
 * run.failed|run.cancelled|run.stopped|run.timedout -> message "error", running = false.
 *
 * Khong bao gio throw ra ngoai khoi handler; loi mang -> message "error" tieng Viet.
 *
 * ES5 only (CEF Chrome 87): khong let/const/arrow/template strings/
 * optional chaining/async.
 */
(function (root) {
  "use strict";

  var NOT_CONNECTED = "Ch\u01B0a k\u1EBFt n\u1ED1i v\u1EDBi WPS. M\u1EDF m\u1ED9t t\u00E0i li\u1EC7u trong WPS Writer r\u1ED3i th\u1EED l\u1EA1i.";
  var NET_ERROR = "L\u1ED7i m\u1EA1ng: kh\u00F4ng k\u1EBFt n\u1ED1i \u0111\u01B0\u1EE3c t\u1EDBi Core. Vui l\u00F2ng th\u1EED l\u1EA1i.";
  var SEND_ERROR = "Kh\u00F4ng g\u1EEDi \u0111\u01B0\u1EE3c y\u00EAu c\u1EA7u. Vui l\u00F2ng th\u1EED l\u1EA1i.";

  function safeString(value) {
    try {
      if (value === undefined || value === null) { return ""; }
      return String(value);
    } catch (e) {
      return "";
    }
  }

  function trimText(value) {
    try {
      return safeString(value).replace(/^\s+|\s+$/g, "");
    } catch (e) {
      return "";
    }
  }

  // Doc payload tu MessageEvent: .data la chuoi JSON cua {type?, data: {...}}.
  function payloadOf(evt) {
    try {
      var raw = evt ? evt.data : null;
      var obj = raw;
      if (typeof raw === "string") {
        try { obj = JSON.parse(raw); } catch (e) { return {}; }
      }
      if (obj && typeof obj === "object") {
        if (obj.data && typeof obj.data === "object") { return obj.data; }
        return obj;
      }
    } catch (e2) {}
    return {};
  }

  function postJson(fetchFn, url, body) {
    var text = "{}";
    try { text = JSON.stringify(body); } catch (e) { text = "{}"; }
    return fetchFn(url, {
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

  function resolved(value) {
    try { return Promise.resolve(value); } catch (e) { return value; }
  }

  function create(options) {
    options = options || {};
    var fetchFn = options.fetch;
    var Source = options.EventSource;
    var base = safeString(options.base);
    var onChange = options.onChange;
    if (typeof onChange !== "function") {
      onChange = function () {};
    }

    var state = {
      messages: [],
      running: false,
      steps: [],
      confirmation: null,
      conversationId: null
    };
    var runId = null;
    var source = null;

    function emit() {
      try {
        var snapshot = JSON.parse(JSON.stringify(state));
        onChange(snapshot);
      } catch (e) {
        try { onChange(state); } catch (e2) {}
      }
    }

    function pushError(text) {
      try {
        var msg = trimText(text);
        if (!msg) { msg = SEND_ERROR; }
        state.messages.push({ role: "error", text: msg });
      } catch (e) {}
    }

    function closeSource() {
      try {
        if (source && typeof source.close === "function") { source.close(); }
      } catch (e) {}
      source = null;
    }

    function setRunning(value) {
      try { state.running = !!value; } catch (e) {}
    }

    function findStep(callId, action) {
      var i;
      try {
        if (callId !== undefined && callId !== null && callId !== "") {
          for (i = 0; i < state.steps.length; i++) {
            if (state.steps[i] && state.steps[i].callId === callId) { return state.steps[i]; }
          }
        }
        if (action) {
          for (i = state.steps.length - 1; i >= 0; i--) {
            if (state.steps[i] && state.steps[i].action === action) { return state.steps[i]; }
          }
        }
        if (state.steps.length > 0) { return state.steps[state.steps.length - 1]; }
      } catch (e) {}
      return null;
    }

    function handlePayload(type, p) {
      try {
        if (type === "tool.started") {
          var action = trimText(p.action) || trimText(p.name) || "tool";
          var step = { action: action, ok: false };
          try {
            if (p.callId !== undefined && p.callId !== null) { step.callId = p.callId; }
          } catch (e) {}
          state.steps.push(step);
        } else if (type === "tool.finished") {
          var ok = true;
          try { ok = (p.ok === undefined || p.ok === null) ? true : !!p.ok; } catch (e2) {}
          var target = findStep(p.callId, trimText(p.action));
          if (target) {
            try { target.ok = ok; } catch (e3) {}
          }
        } else if (type === "confirm.required") {
          state.confirmation = {
            id: safeString(p.confirmationId),
            action: safeString(p.action),
            reason: safeString(p.reason)
          };
        } else if (type === "confirm.resolved") {
          state.confirmation = null;
        } else if (type === "run.completed") {
          var reply = "";
          try {
            if (p.reply !== undefined && p.reply !== null) { reply = safeString(p.reply); }
            else if (p.text !== undefined && p.text !== null) { reply = safeString(p.text); }
            else if (p.message !== undefined && p.message !== null) { reply = safeString(p.message); }
          } catch (e4) {}
          try { state.messages.push({ role: "assistant", text: reply }); } catch (e5) {}
          state.confirmation = null;
          setRunning(false);
          closeSource();
        } else if (type === "run.failed" || type === "run.cancelled" ||
                   type === "run.stopped" || type === "run.timedout") {
          var msg = "";
          try {
            if (p.error !== undefined && p.error !== null) { msg = safeString(p.error); }
            else if (p.message !== undefined && p.message !== null) { msg = safeString(p.message); }
            else if (p.reply !== undefined && p.reply !== null) { msg = safeString(p.reply); }
            else if (p.text !== undefined && p.text !== null) { msg = safeString(p.text); }
          } catch (e6) {}
          msg = trimText(msg);
          if (!msg) {
            if (type === "run.cancelled" || type === "run.stopped") {
              msg = "\u0110\u00E3 d\u1EEBng l\u1EA1i.";
            } else if (type === "run.timedout") {
              msg = "H\u1EBFt th\u1EDDi gian ch\u1EDD. Vui l\u00F2ng th\u1EED l\u1EA1i.";
            } else {
              msg = "L\u01B0\u1EE3t ch\u1EA1y th\u1EA5t b\u1EA1i. Vui l\u00F2ng th\u1EED l\u1EA1i.";
            }
          }
          pushError(msg);
          state.confirmation = null;
          setRunning(false);
          closeSource();
        }
      } catch (e7) {}
      try { emit(); } catch (e8) {}
    }

    function onEvent(type) {
      return function (evt) {
        try {
          handlePayload(type, payloadOf(evt));
        } catch (e) {
          try { emit(); } catch (e2) {}
        }
      };
    }

    function onMessage(evt) {
      try {
        var obj = null;
        try {
          obj = (typeof evt.data === "string") ? JSON.parse(evt.data) : evt.data;
        } catch (e) { obj = null; }
        if (!obj || typeof obj !== "object") { return; }
        var type = safeString(obj.type);
        if (!type) { return; }
        handlePayload(type, (obj.data && typeof obj.data === "object") ? obj.data : obj);
      } catch (e2) {
        try { emit(); } catch (e3) {}
      }
    }

    function onSourceError() {
      try {
        if (state.running) {
          pushError(NET_ERROR);
          state.confirmation = null;
          setRunning(false);
          closeSource();
          emit();
        }
      } catch (e) {}
    }

    function subscribe(id) {
      closeSource();
      var url = base + "runs/" + safeString(id) + "/events";
      var es = null;
      try {
        es = new Source(url);
      } catch (e) {
        pushError(NET_ERROR);
        setRunning(false);
        emit();
        return null;
      }
      if (!es) {
        pushError(NET_ERROR);
        setRunning(false);
        emit();
        return null;
      }
      source = es;
      var types = ["tool.started", "tool.finished", "confirm.required", "confirm.resolved",
        "run.completed", "run.failed", "run.cancelled", "run.stopped", "run.timedout"];
      try {
        for (var i = 0; i < types.length; i++) {
          try {
            if (es && typeof es.addEventListener === "function") {
              es.addEventListener(types[i], onEvent(types[i]));
            }
          } catch (e2) {}
        }
        try {
          if (es && typeof es.addEventListener === "function") {
            es.addEventListener("error", onSourceError);
          } else if (es) {
            try { es.onerror = onSourceError; } catch (e3) {}
          }
        } catch (e4) {}
        // Du phong khi server gui event mac dinh "message" chua {type, data}.
        try {
          if (es && typeof es.addEventListener === "function") {
            es.addEventListener("message", onMessage);
          } else if (es && !es.onmessage) {
            try { es.onmessage = onMessage; } catch (e5) {}
          }
        } catch (e6) {}
      } catch (e7) {}
      return es;
    }

    function send(prompt) {
      try {
        if (state.running) { return false; }
        var text = trimText(prompt);
        if (!text) { return false; }
        try {
          state.messages.push({ role: "user", text: text });
          state.steps = [];
          state.confirmation = null;
          setRunning(true);
        } catch (e) {}
        emit();
        var body = { prompt: text };
        try {
          if (state.conversationId) { body.conversationId = state.conversationId; }
        } catch (e2) {}
        var req = null;
        try {
          req = postJson(fetchFn, base + "runs", body);
        } catch (e3) {
          req = null;
        }
        if (!req || typeof req.then !== "function") {
          pushError(NET_ERROR);
          setRunning(false);
          emit();
          return resolved(true);
        }
        return req.then(function (resp) {
          var status = 0;
          try { status = (resp && resp.status !== undefined) ? resp.status : 0; } catch (e4) {}
          if (status === 409) {
            pushError(NOT_CONNECTED);
            setRunning(false);
            emit();
            return true;
          }
          var jr = null;
          try { jr = readJson(resp); } catch (e5) { jr = resolved(null); }
          return jr.then(function (data) {
            try {
              var result = (data && data.result !== undefined) ? data.result : data;
              var id = result ? (result.runId || result.id) : null;
              var conv = result ? (result.conversationId || result.conversation_id) : null;
              var failed = false;
              try { failed = !!resp && resp.ok === false; } catch (e6) {}
              if (failed || !id) {
                var detail = "";
                try {
                  if (result && (result.error || result.message)) {
                    detail = safeString(result.error || result.message);
                  } else if (data && (data.error || data.message)) {
                    detail = safeString(data.error || data.message);
                  }
                } catch (e7) {}
                if (status === 409 || /not connected/i.test(detail)) {
                  pushError(NOT_CONNECTED);
                } else if (trimText(detail)) {
                  pushError("Kh\u00F4ng g\u1EEDi \u0111\u01B0\u1EE3c y\u00EAu c\u1EA7u: " + trimText(detail));
                } else {
                  pushError(SEND_ERROR);
                }
                setRunning(false);
                emit();
                return true;
              }
              runId = id;
              if (conv) {
                try { state.conversationId = conv; } catch (e8) {}
              }
              emit();
              subscribe(runId);
              emit();
              return true;
            } catch (e9) {
              pushError(SEND_ERROR);
              setRunning(false);
              emit();
              return true;
            }
          });
        }).then(null, function () {
          pushError(NET_ERROR);
          setRunning(false);
          emit();
          return true;
        });
      } catch (e10) {
        try {
          pushError(SEND_ERROR);
          setRunning(false);
          emit();
        } catch (e11) {}
        return resolved(true);
      }
    }

    function confirm(approved) {
      try {
        var cid = null;
        try { cid = state.confirmation ? state.confirmation.id : null; } catch (e) { cid = null; }
        if (!runId || !cid) { return resolved(false); }
        var req = null;
        try {
          req = postJson(fetchFn, base + "runs/" + safeString(runId) + "/confirm",
            { confirmationId: cid, approved: !!approved });
        } catch (e2) { req = null; }
        if (!req || typeof req.then !== "function") {
          pushError(NET_ERROR);
          emit();
          return resolved(false);
        }
        return req.then(function () { return true; }).then(null, function () {
          pushError(NET_ERROR);
          emit();
          return false;
        });
      } catch (e3) {
        return resolved(false);
      }
    }

    function stop() {
      try {
        if (!runId) { return resolved(false); }
        var req = null;
        try {
          req = postJson(fetchFn, base + "runs/" + safeString(runId) + "/cancel", {});
        } catch (e) { req = null; }
        if (!req || typeof req.then !== "function") {
          pushError(NET_ERROR);
          emit();
          return resolved(false);
        }
        return req.then(function () { return true; }).then(null, function () {
          pushError(NET_ERROR);
          emit();
          return false;
        });
      } catch (e2) {
        return resolved(false);
      }
    }

    function newConversation() {
      try {
        closeSource();
        runId = null;
        state.messages = [];
        state.steps = [];
        state.confirmation = null;
        state.conversationId = null;
        setRunning(false);
        emit();
      } catch (e) {}
    }

    function getState() {
      try {
        return JSON.parse(JSON.stringify(state));
      } catch (e) {
        return state;
      }
    }

    return {
      send: send,
      confirm: confirm,
      stop: stop,
      newConversation: newConversation,
      getState: getState
    };
  }

  var api = { create: create };
  try {
    if (root) { root.AxiomPaneCore = api; }
  } catch (e) {}
})(typeof window !== "undefined" ? window : (typeof globalThis !== "undefined" ? globalThis : this));
