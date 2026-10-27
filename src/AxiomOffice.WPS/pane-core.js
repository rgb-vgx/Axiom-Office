/*
 * Axiom Office - WPS Ask AI task pane logic (khong DOM).
 *
 * Dinh nghia global AxiomPaneCore.create(options):
 *   options = { fetch, EventSource, base: "<...>/wps/v1/", kind: "wps"|"et"|"wpp", onChange(state),
 *               requireSelection? }  - requireSelection: moi luot gui kem options.requireSelection (khung chat mini canh
 *               doan boi den - mini.html): Core dung ngay neu khong co doan boi den, khong sua ca tai lieu.
 *   state   = { messages: [{role: "user"|"assistant"|"error"|"info", text}],
 *               running, steps: [{action, ok, error?}],
 *               confirmation: null | {id, action, reason, choices?: [{id, label, detail, input?}]}, conversationId,
 *               model, mode: ""|"plan"|"act", plan, planReady,
 *               reasoning: null | {round, text, live, expanded},
 *               skills: [{name}], notes: [{id, text, state}], progress,
 *               seconds, rounds, edits, error, status, lastReply, canUndo, canInsert }
 *
 * controller.send(prompt)    -> POST base+"runs" {prompt, conversationId?, options?: {mode: "plan"}} roi mo
 *                               EventSource(base+"runs/<id>/events"); dang chay thi bo qua (tra false).
 *                               409 (add-in chua ket noi) -> message "error" tieng Viet.
 * controller.loadQuickActions() -> GET base+"quick-actions" -> state.quickActions = [{id, label, edits}] - nhan cua
 *                               cac muc "Sua bang AI" (menu chuot phai); chi Writer, app khac giu []. Loi -> [].
 * controller.quick(id)       -> POST base+"runs" {quickAction: id, conversationId?} - cau lenh do Core lay theo id;
 *                               tin nhan nguoi dung hien NHAN cua muc. Bo qua "Lap ke hoach truoc" (viec nho, ro).
 * controller.confirm(ok)     -> POST base+"runs/<id>/confirm" {confirmationId, approved: ok}
 * controller.choose(id, text?) -> POST base+"runs/<id>/confirm" {confirmationId, choice: id, text?} (the nhieu lua
 *                               chon; text = chu nguoi dung go o lua chon input)
 * controller.stop()          -> POST base+"runs/<id>/cancel"
 * controller.newConversation() -> xoa conversationId va messages.
 * controller.setPlan(on)     -> bat/tat "Lap ke hoach truoc" cho luot sau (options.mode cua Core).
 * controller.toggleReasoning() -> mo/thu khoi suy luon cua luot nay.
 * controller.undoLastTurn()  -> POST base+"exec" {action: "<kind>.undo", params: {count: edits}} (khong qua agent).
 * controller.insertReply()   -> POST base+"exec" {action: "writer.insertStyledText", params: {text: lastReply}}.
 * controller.forget(id)      -> DELETE base+"memory/<id>" (the "Xoa" tren the ghi nho vua ghi).
 *
 * Su kien SSE (MessageEvent ma .data la chuoi JSON cua {type?, data: {...}}):
 * doc payload tu truong "data" (fallback: chinh object do).
 * run.started -> model/mode/conversationId; run.progress -> progress; run.reasoning|run.reasoning.live -> reasoning;
 * tool.started -> steps += {action}; tool.finished -> step.ok (load_skill/remember thanh dong rieng);
 * skill.loaded -> skills += {name}; memory.written -> notes += {id, text};
 * confirm.required -> confirmation; confirm.resolved -> confirmation = null;
 * run.completed -> message assistant = reply, running = false, dong EventSource;
 * run.failed|run.cancelled|run.stopped|run.timedout -> message "error", running = false.
 *   run.failed kind "selection" / "selection-lock" (lenh nhanh khong co doan boi den) -> cau tieng Viet rieng
 *   (QUICK_ERRORS) thay vi chuoi tieng Anh cua Core.
 *
 * Trang thai dem theo chat.py cua pane LibreOffice (cung he thiet ke): edits = so thao tac SUA tai lieu cua luot
 * (nut "Hoan tac luot nay"), seconds/rounds tu run.completed, summary(state) = dong trang thai o footer.
 *
 * Khong bao gio throw ra ngoai khoi handler; loi mang -> message "error" tieng Viet.
 *
 * ES5 only (CEF Chrome 87): khong let/const/arrow/template strings/
 * optional chaining/async.
 */
(function (root) {
  "use strict";

  var NOT_CONNECTED = "Ch\u01B0a k\u1EBFt n\u1ED1i v\u1EDBi WPS. M\u1EDF m\u1ED9t t\u00E0i li\u1EC7u trong WPS r\u1ED3i th\u1EED l\u1EA1i.";
  var NET_ERROR = "Kh\u00F4ng g\u1ECDi \u0111\u01B0\u1EE3c Agent Core. Ki\u1EC3m tra Core \u0111ang ch\u1EA1y tr\u00EAn m\u00E1y n\u00E0y r\u1ED3i th\u1EED l\u1EA1i.";
  // Lenh nhanh "Sua bang AI" dung ngay khi khong co doan boi den (agent/orchestrator.go: hasSelection) - ma
  // kind cua run.failed, dich sang cau noi duoc nguoi dung phai lam gi.
  var QUICK_ERRORS = {
    "selection": "H\u00E3y b\u00F4i \u0111en \u0111o\u1EA1n v\u0103n c\u1EA7n l\u00E0m tr\u01B0\u1EDBc, r\u1ED3i th\u1EED l\u1EA1i.",
    "selection-lock": "B\u1EA3n WPS n\u00E0y kh\u00F4ng kho\u00E1 \u0111\u01B0\u1EE3c v\u00F9ng b\u00F4i \u0111en, n\u00EAn kh\u00F4ng ch\u1EA1y l\u1EC7nh s\u1EEDa nhanh (AI c\u00F3 th\u1EC3 s\u1EEDa ra ngo\u00E0i \u0111o\u1EA1n \u0111\u00E3 ch\u1ECDn). H\u00E3y g\u00F5 y\u00EAu c\u1EA7u v\u00E0o \u00F4 so\u1EA1n."
  };
  var SEND_ERROR = "Agent Core tr\u1EA3 l\u1EDDi kh\u00F4ng h\u1EE3p l\u1EC7. Vui l\u00F2ng th\u1EED l\u1EA1i; n\u1EBFu v\u1EABn l\u1ED7i, kh\u1EDFi \u0111\u1ED9ng l\u1EA1i Core.";

  // Suy luan ngan hon nguong nay thi hien thang; dai hon thi thu lai thanh mot dong (bam "Hien" de mo) -
  // cung nguong voi chat.py:REASONING_INLINE_LIMIT.
  var REASONING_INLINE_LIMIT = 320;
  // Dong xem truoc khi thu lai (theme.reasoning_preview cua pane LibreOffice).
  var REASONING_PREVIEW_LIMIT = 200;

  // --- BEGIN SINH TU DONG (scripts/generate_mcp_commands.py) - KHONG SUA TAY ---
  // Lenh SUA tai lieu: co `undo` trong catalog = dung mot buoc Undo; pane dem de "Hoan tac luot nay".
  var EDIT_ACTIONS = {
    "et.addChart": true,
    "et.addSheet": true,
    "et.createTable": true,
    "et.fillRange": true,
    "et.formatRange": true,
    "et.formatRanges": true,
    "et.freezePanes": true,
    "et.importCsv": true,
    "et.renameSheet": true,
    "et.setConditionalFormat": true,
    "et.setValidation": true,
    "et.sort": true,
    "et.writeRange": true,
    "et.writeRanges": true,
    "wpp.addChart": true,
    "wpp.addConnector": true,
    "wpp.addGrid": true,
    "wpp.addImage": true,
    "wpp.addParagraphs": true,
    "wpp.addShape": true,
    "wpp.addSlide": true,
    "wpp.addSlideFromTemplate": true,
    "wpp.addTable": true,
    "wpp.addText": true,
    "wpp.addTextBox": true,
    "wpp.copyPicture": true,
    "wpp.deleteShapes": true,
    "wpp.deleteSlide": true,
    "wpp.deleteTagged": true,
    "wpp.setNotes": true,
    "wpp.setShapeText": true,
    "writer.addComment": true,
    "writer.addTableRows": true,
    "writer.appendText": true,
    "writer.deleteParagraphs": true,
    "writer.editCommentScope": true,
    "writer.formatSelection": true,
    "writer.formatTable": true,
    "writer.heading": true,
    "writer.insertBlocks": true,
    "writer.insertCaption": true,
    "writer.insertHyperlink": true,
    "writer.insertImage": true,
    "writer.insertLayoutTable": true,
    "writer.insertPageBreak": true,
    "writer.insertStyledText": true,
    "writer.insertTable": true,
    "writer.insertToc": true,
    "writer.replaceAll": true,
    "writer.replaceMany": true,
    "writer.replyComment": true,
    "writer.resolveComment": true,
    "writer.restyle": true,
    "writer.reviewRevisions": true,
    "writer.setHeaderFooter": true,
    "writer.setParagraphAlignment": true,
    "writer.setStyle": true,
    "writer.typeText": true,
    "writer.writeContent": true
  };
  // --- END SINH TU DONG ---
  // "Hoan tac luot nay" / "Chen tra loi": ban Windows chi co o Word (Excel/PowerPoint khong hoan tac duoc qua COM).
  // WPS co writer.undo va et.undo nhung KHONG co wpp.undo -> PowerPoint khong hien nut hoan tac.
  var UNDO_ACTIONS = { "wps": "writer.undo", "et": "et.undo" };
  var INSERT_ACTIONS = { "wps": "writer.insertStyledText" };
  // Cau gui khi bam "Thuc hien ke hoach" (AskAiPane.ExecutePlanPrompt cua ban Windows).
  var PLAN_EXECUTE_PROMPT = "Th\u1EF1c hi\u1EC7n k\u1EBF ho\u1EA1ch \u1EDF tr\u00EAn. C\u00E2u h\u1ECFi n\u00E0o t\u00F4i ch\u01B0a tr\u1EA3 l\u1EDDi th\u00EC d\u00F9ng ph\u01B0\u01A1ng \u00E1n m\u1EB7c \u0111\u1ECBnh \u0111\u00E3 ghi trong k\u1EBF ho\u1EA1ch.";

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
    var kind = trimText(options.kind) || "wps";
    var requireSelection = options.requireSelection === true;
    var onChange = options.onChange;
    if (typeof onChange !== "function") {
      onChange = function () {};
    }

    var state = {
      messages: [],
      running: false,
      steps: [],
      confirmation: null,
      conversationId: null,
      model: "",
      mode: "",
      plan: false,
      planReady: false,
      reasoning: null,
      skills: [],
      notes: [],
      progress: "",
      seconds: 0,
      rounds: 0,
      edits: 0,
      error: "",
      status: "",
      lastReply: "",
      canUndo: false,
      canInsert: false,
      quickActions: []
    };
    var runId = null;
    var source = null;

    function emit() {
      try {
        state.canUndo = canUndo();
        state.canInsert = canInsert();
      } catch (e0) {}
      try {
        var snapshot = JSON.parse(JSON.stringify(state));
        onChange(snapshot);
      } catch (e) {
        try { onChange(state); } catch (e2) {}
      }
    }

    function canUndo() {
      return !state.running && state.edits > 0 && !!UNDO_ACTIONS[kind];
    }

    function canInsert() {
      return !state.running && !!INSERT_ACTIONS[kind] && trimText(state.lastReply) !== "";
    }

    // fatal = loi that (khong gui duoc / Core tra loi sai / SSE dut): footer chi ve "nhấn Cài đặt"
    // (chat.py:fail). Nguoi dung tu bam "Dung" thi khong phai loi nen khong bat co nay.
    function pushError(text, fatal) {
      try {
        var msg = trimText(text);
        if (!msg) { msg = SEND_ERROR; }
        state.messages.push({ role: "error", text: msg });
        if (fatal) {
          state.error = msg;
          state.status = "";
        }
      } catch (e) {}
    }

    // Dong thong bao them tay (vd ket qua "Hoan tac luot nay") - chat.py:note cua pane LibreOffice.
    function pushInfo(text) {
      try {
        var msg = trimText(text);
        if (msg) { state.messages.push({ role: "info", text: msg }); }
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

    // Bo mot dong thao tac da them: load_skill/remember co dong rieng (skill.loaded/memory.written) nen
    // khong hien hai lan (chat.py bo qua tool.finished cua hai lenh do khi thanh cong).
    function dropStep(callId, action) {
      try {
        var i;
        if (callId !== undefined && callId !== null && callId !== "") {
          for (i = 0; i < state.steps.length; i++) {
            if (state.steps[i] && state.steps[i].callId === callId) { state.steps.splice(i, 1); return; }
          }
        }
        if (action) {
          for (i = state.steps.length - 1; i >= 0; i--) {
            if (state.steps[i] && state.steps[i].action === action) { state.steps.splice(i, 1); return; }
          }
        }
      } catch (e) {}
    }

    function handlePayload(type, p) {
      try {
        if (type === "run.started") {
          try {
            if (p.model !== undefined && p.model !== null) { state.model = safeString(p.model); }
            if (p.mode !== undefined && p.mode !== null) { state.mode = safeString(p.mode); }
            if (p.conversationId !== undefined && p.conversationId !== null && safeString(p.conversationId)) {
              state.conversationId = safeString(p.conversationId);
            }
          } catch (eStart) {}
          state.status = state.model ? "Đang chạy · " + state.model : "Đang chạy…";
        } else if (type === "run.progress") {
          // "Dang thiet ke phan 1-15: 7 slide..." - dong cho biet model dang lam gi (liveProgress cua Core).
          state.progress = trimText(p.text);
        } else if (type === "run.reasoning" || type === "run.reasoning.live") {
          // MOT muc duy nhat cho ca luot, moi vong ghi de bang phan suy luan moi nhat (chat.py:_reasoning).
          // Ban "live" chi mang phan duoi cua suy luan nen luon thu lai; ban day du moi tinh nguong mo san.
          var rtext = trimText(p.text);
          if (rtext) {
            var round = 0;
            try { round = parseInt(p.round, 10) || 0; } catch (eRound) { round = 0; }
            var live = (type === "run.reasoning.live");
            if (!state.reasoning || state.reasoning.round !== round) {
              state.reasoning = { round: round, text: rtext, live: live,
                expanded: !live && rtext.length <= REASONING_INLINE_LIMIT };
            } else {
              state.reasoning.text = rtext;
              state.reasoning.live = live;
            }
          }
        } else if (type === "skill.loaded") {
          try { state.skills.push({ name: trimText(p.name) || "?" }); } catch (eSkill) {}
        } else if (type === "memory.written") {
          var noteText = trimText(p.text);
          var noteId = safeString(p.id);
          if (noteText) {
            var duplicate = false;
            try {
              for (var ni = 0; ni < state.notes.length; ni++) {
                if (noteId && state.notes[ni].id === noteId) { duplicate = true; }
              }
            } catch (eDup) {}
            if (!duplicate) {
              try { state.notes.push({ id: noteId, text: noteText, state: "saved" }); } catch (eNote) {}
            }
          }
        } else if (type === "tool.started") {
          var action = trimText(p.action) || trimText(p.name) || "tool";
          var step = { action: action, ok: false };
          try {
            if (p.callId !== undefined && p.callId !== null) { step.callId = p.callId; }
          } catch (e) {}
          state.steps.push(step);
        } else if (type === "tool.finished") {
          var ok = true;
          try { ok = (p.ok === undefined || p.ok === null) ? true : !!p.ok; } catch (e2) {}
          var doneAction = trimText(p.action) || trimText(p.tool);
          if (ok && (doneAction === "load_skill" || doneAction === "remember")) {
            dropStep(p.callId, doneAction);
          } else {
            var target = findStep(p.callId, doneAction);
            if (target) {
              try {
                target.ok = ok;
                if (!ok) { target.error = trimText(p.error); }
              } catch (e3) {}
            }
          }
          // Moi thao tac SUA tai lieu = mot buoc Undo cua bridge -> dem de "Hoan tac luot nay".
          if (ok && isEdit(doneAction)) {
            try { state.edits += 1; } catch (eEdit) {}
          }
        } else if (type === "confirm.required") {
          state.confirmation = {
            id: safeString(p.confirmationId),
            action: safeString(p.action),
            reason: safeString(p.reason)
          };
          // The nhieu lua chon (doc_design hoi kieu trinh bay).
          try {
            if (p.choices && p.choices.length) {
              var choices = [];
              for (var ci = 0; ci < p.choices.length; ci++) {
                var c = p.choices[ci] || {};
                if (c.id) {
                  var item = { id: safeString(c.id), label: safeString(c.label || c.id), detail: safeString(c.detail || "") };
                  if (c.input) { item.input = true; }
                  choices.push(item);
                }
              }
              if (choices.length) { state.confirmation.choices = choices; }
            }
          } catch (eChoices) {}
        } else if (type === "confirm.resolved") {
          state.confirmation = null;
        } else if (type === "run.completed") {
          applyUndoSteps(p);
          var reply = "";
          try {
            if (p.reply !== undefined && p.reply !== null) { reply = safeString(p.reply); }
            else if (p.text !== undefined && p.text !== null) { reply = safeString(p.text); }
            else if (p.message !== undefined && p.message !== null) { reply = safeString(p.message); }
          } catch (e4) {}
          try { state.messages.push({ role: "assistant", text: reply }); } catch (e5) {}
          try {
            state.lastReply = reply;
            state.seconds = Number(p.seconds) || 0;
            state.rounds = Number(p.rounds) || 0;
            if (p.model !== undefined && p.model !== null && safeString(p.model)) { state.model = safeString(p.model); }
          } catch (eDone) {}
          state.confirmation = null;
          state.progress = "";
          // Luot "Lap ke hoach truoc": chua sua gi, the ke hoach duoi cau tra loi cho chay that
          // (AskAiPane: "Đã lập kế hoạch trong Ns · chưa sửa gì").
          if (state.mode === "plan") {
            state.planReady = true;
            state.status = "Đã lập kế hoạch trong " + Math.floor(state.seconds) + "s · chưa sửa gì";
          } else {
            state.status = "";
          }
          setRunning(false);
          closeSource();
        } else if (type === "run.failed" || type === "run.cancelled" ||
                   type === "run.stopped" || type === "run.timedout") {
          applyUndoSteps(p);
          var msg = "";
          try {
            if (p.error !== undefined && p.error !== null) { msg = safeString(p.error); }
            else if (p.message !== undefined && p.message !== null) { msg = safeString(p.message); }
            else if (p.reply !== undefined && p.reply !== null) { msg = safeString(p.reply); }
            else if (p.text !== undefined && p.text !== null) { msg = safeString(p.text); }
          } catch (e6) {}
          msg = trimText(msg);
          try {
            if (type === "run.failed" && Object.prototype.hasOwnProperty.call(QUICK_ERRORS, safeString(p.kind))) {
              msg = QUICK_ERRORS[safeString(p.kind)];
            }
          } catch (eKind) {}
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
          try {
            state.progress = "";
            // Nguoi dung tu dung thi khong phai "loi" - footer noi "\u0110\u00E3 d\u1EEBng \u00B7 N thao t\u00E1c" (AskAiPane),
            // con loi that (run.failed/het gio) moi bat co de footer chi ve "nh\u1EA5n C\u00E0i \u0111\u1EB7t".
            if (type === "run.failed" || type === "run.timedout") {
              state.error = msg;
              state.status = "";
            } else {
              state.status = state.edits > 0 ? "\u0110\u00E3 d\u1EEBng \u00B7 " + state.edits + " thao t\u00E1c" : "\u0110\u00E3 d\u1EEBng";
            }
          } catch (eStop) {}
          state.confirmation = null;
          setRunning(false);
          closeSource();
        }
      } catch (e7) {}
      try { emit(); } catch (e8) {}
    }

    // Core bao so buoc hoan tac Word CHAC CHAN cua luot (undoSteps - da tru lan agent tu writer.undo). Tu dem tool.finished
    // thi khong tru duoc va "Hoan tac luot nay" lui vao noi dung co tu truoc luot (audit agent runtime F02). Chi Writer:
    // undoSteps cua Core la so buoc writer.*; ET giu cach dem cu. Core cu khong gui truong nay thi giu nguyen.
    function applyUndoSteps(p) {
      try {
        if (kind !== "wps" || !p || p.undoSteps === undefined || p.undoSteps === null) { return; }
        var steps = Number(p.undoSteps);
        if (!isNaN(steps) && steps >= 0) { state.edits = Math.floor(steps); }
      } catch (e) {}
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
          pushError(NET_ERROR, true);
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
        pushError(NET_ERROR, true);
        setRunning(false);
        emit();
        return null;
      }
      if (!es) {
        pushError(NET_ERROR, true);
        setRunning(false);
        emit();
        return null;
      }
      source = es;
      var types = ["run.started", "run.progress", "run.reasoning", "run.reasoning.live",
        "tool.started", "tool.finished", "skill.loaded", "memory.written",
        "confirm.required", "confirm.resolved",
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
      var text = trimText(prompt);
      if (!text) { return false; }
      var body = { prompt: text };
      // "Lap ke hoach truoc": Core chi cho doc tai lieu va tra ve ke hoach (CoreClient: mode "plan").
      try { if (state.plan) { body.options = { mode: "plan" }; } } catch (e) {}
      return startRun(text, body);
    }

    // quick: mot muc cua menu "Sua bang AI". Chi gui id - cau lenh nam trong Core (agent/quickactions.go) de ba
    // ban ra cung mot cau; khung chat hien NHAN cua muc ("Trau chuot") thay vi ca cau lenh dai.
    function quick(id) {
      var actionId = trimText(id);
      if (!actionId) { return false; }
      var label = actionId;
      try {
        for (var i = 0; i < state.quickActions.length; i++) {
          if (state.quickActions[i].id === actionId) { label = state.quickActions[i].label; }
        }
      } catch (e) {}
      // Muc chon tu menu chuot phai (main.js -> Core -> pane) den bat ky luc nao, ke ca giua mot luot dang chay - noi
      // ro la bi bo qua, khong im lang nuot mat viec nguoi dung vua chon.
      if (state.running) {
        pushInfo("\u0110ang c\u00F3 m\u1ED9t l\u01B0\u1EE3t ch\u1EA1y \u2014 ch\u1EDD xong r\u1ED3i ch\u1ECDn l\u1EA1i \u201C" + label + "\u201D.");
        emit();
        return false;
      }
      return startRun(label, { quickAction: actionId });
    }

    // loadQuickActions: danh sach muc lay tu Core, khong chep tay. Chi Writer: lenh nhanh can khoa vung boi den,
    // ma khoa vung moi co cho van ban (Calc/Impress khong co doan van de boi den).
    function loadQuickActions() {
      try {
        if (kind !== "wps") { return resolved(false); }
        var req = null;
        try { req = fetchFn(base + "quick-actions", { method: "GET" }); } catch (e) { req = null; }
        if (!req || typeof req.then !== "function") { return resolved(false); }
        return req.then(function (resp) {
          if (!resp || resp.ok === false) { return null; }
          return readJson(resp);
        }).then(function (data) {
          var list = [];
          try {
            var result = (data && data.result !== undefined) ? data.result : data;
            var raw = (result && result.actions) || [];
            for (var i = 0; i < raw.length; i++) {
              var id = trimText(raw[i] && raw[i].id);
              var label = trimText(raw[i] && raw[i].label);
              if (id && label) { list.push({ id: id, label: label, edits: !!raw[i].edits }); }
            }
          } catch (e2) { list = []; }
          state.quickActions = list;
          emit();
          return list.length > 0;
        }).then(null, function () {
          state.quickActions = [];
          emit();
          return false;
        });
      } catch (e3) {
        return resolved(false);
      }
    }

    function startRun(text, body) {
      try {
        if (state.running) { return false; }
        try {
          state.messages.push({ role: "user", text: text });
          state.steps = [];
          state.confirmation = null;
          // Trang thai cua luot truoc: xoa het de "Hoan tac luot nay"/the ke hoach khong dinh luot cu
          // (chat.py:begin).
          state.edits = 0;
          state.notes = [];
          state.skills = [];
          state.reasoning = null;
          state.progress = "";
          state.error = "";
          state.planReady = false;
          state.lastReply = "";
          state.seconds = 0;
          state.rounds = 0;
          state.status = "Đang chạy…";
          setRunning(true);
        } catch (e) {}
        emit();
        try {
          if (state.conversationId) { body.conversationId = state.conversationId; }
          if (requireSelection) {
            body.options = body.options || {};
            body.options.requireSelection = true;
          }
        } catch (e2) {}
        var req = null;
        try {
          req = postJson(fetchFn, base + "runs", body);
        } catch (e3) {
          req = null;
        }
        if (!req || typeof req.then !== "function") {
          pushError(NET_ERROR, true);
          setRunning(false);
          emit();
          return resolved(true);
        }
        return req.then(function (resp) {
          var status = 0;
          try { status = (resp && resp.status !== undefined) ? resp.status : 0; } catch (e4) {}
          if (status === 409) {
            pushError(NOT_CONNECTED, true);
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
                  pushError(NOT_CONNECTED, true);
                } else if (trimText(detail)) {
                  pushError("Kh\u00F4ng g\u1EEDi \u0111\u01B0\u1EE3c y\u00EAu c\u1EA7u: " + trimText(detail), true);
                } else {
                  pushError(SEND_ERROR, true);
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
              pushError(SEND_ERROR, true);
              setRunning(false);
              emit();
              return true;
            }
          });
        }).then(null, function () {
          pushError(NET_ERROR, true);
          setRunning(false);
          emit();
          return true;
        });
      } catch (e10) {
        try {
          pushError(SEND_ERROR, true);
          setRunning(false);
          emit();
        } catch (e11) {}
        return resolved(true);
      }
    }

    function confirm(approved) {
      return answer({ approved: !!approved });
    }

    function choose(choiceId, text) {
      var reply = { choice: safeString(choiceId) };
      var typed = "";
      try { typed = trimText(text); } catch (e) { typed = ""; }
      if (typed) { reply.text = typed; }
      return answer(reply);
    }

    function answer(reply) {
      try {
        var cid = null;
        try { cid = state.confirmation ? state.confirmation.id : null; } catch (e) { cid = null; }
        if (!runId || !cid) { return resolved(false); }
        var req = null;
        try {
          var body = { confirmationId: cid };
          for (var key in reply) {
            if (Object.prototype.hasOwnProperty.call(reply, key)) { body[key] = reply[key]; }
          }
          req = postJson(fetchFn, base + "runs/" + safeString(runId) + "/confirm", body);
        } catch (e2) { req = null; }
        if (!req || typeof req.then !== "function") {
          pushError(NET_ERROR, true);
          emit();
          return resolved(false);
        }
        return req.then(function () { return true; }).then(null, function () {
          pushError(NET_ERROR, true);
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
        try { state.status = "Đang dừng…"; emit(); } catch (eWait) {}
        var req = null;
        try {
          req = postJson(fetchFn, base + "runs/" + safeString(runId) + "/cancel", {});
        } catch (e) { req = null; }
        if (!req || typeof req.then !== "function") {
          pushError(NET_ERROR, true);
          emit();
          return resolved(false);
        }
        return req.then(function () { return true; }).then(null, function () {
          pushError(NET_ERROR, true);
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
        state.reasoning = null;
        state.skills = [];
        state.notes = [];
        state.progress = "";
        state.edits = 0;
        state.seconds = 0;
        state.rounds = 0;
        state.error = "";
        state.status = "";
        state.planReady = false;
        state.lastReply = "";
        setRunning(false);
        emit();
      } catch (e) {}
    }

    function setPlan(on) {
      try {
        state.plan = !!on;
        emit();
      } catch (e) {}
      return state.plan;
    }

    function toggleReasoning() {
      try {
        if (state.reasoning) {
          state.reasoning.expanded = !state.reasoning.expanded;
          emit();
        }
      } catch (e) {}
    }

    // POST base+"exec": goi thang mot lenh bridge, khong qua agent (Core: /wps/v1/exec). Tra {ok, error?, result?}.
    function exec(action, params) {
      var req = null;
      try {
        req = postJson(fetchFn, base + "exec", { action: action, params: params || {} });
      } catch (e) {
        req = null;
      }
      if (!req || typeof req.then !== "function") {
        return resolved({ ok: false, error: NET_ERROR });
      }
      return req.then(function (resp) {
        var status = 0;
        try { status = (resp && resp.status !== undefined) ? resp.status : 0; } catch (e2) {}
        var jr = null;
        try { jr = readJson(resp); } catch (e3) { jr = resolved(null); }
        return jr.then(function (data) {
          var payload = (data && data.result !== undefined) ? data.result : data;
          if (status >= 400) {
            var detail = "";
            try {
              detail = safeString((payload && payload.error) || (data && (data.error || data.message)));
            } catch (e4) {}
            return { ok: false, error: trimText(detail) || SEND_ERROR };
          }
          if (payload && typeof payload === "object" && payload.ok === false) {
            return { ok: false, error: trimText(payload.error) || SEND_ERROR };
          }
          var value = null;
          try { value = (payload && payload.result !== undefined) ? payload.result : payload; } catch (e5) {}
          return { ok: true, result: value };
        });
      }).then(null, function () {
        return { ok: false, error: NET_ERROR };
      });
    }

    // "Hoan tac luot nay" (AskAiPane.UndoLastRun; chi khi app co lenh undo va luot vua roi co sua tai lieu).
    function undoLastTurn() {
      try {
        if (!canUndo()) { return resolved(false); }
        var action = UNDO_ACTIONS[kind];
        var count = state.edits;
        return exec(action, { count: count }).then(function (outcome) {
          try {
            if (outcome && outcome.ok) {
              var result = outcome.result || {};
              var undone = Number(result.undone);
              if (isNaN(undone)) { undone = count; }
              state.edits = 0;
              // et.undo: dinh dang, bieu do... WPS Spreadsheets chua hoan tac duoc - noi ro, khong im lang.
              var skipped = (result.skipped && result.skipped.length) || 0;
              pushInfo("Đã hoàn tác " + undone + " thao tác của lượt vừa rồi." + (skipped > 0
                ? " " + skipped + " thao tác chưa hoàn tác được trên bảng tính WPS (định dạng, biểu đồ…), hãy sửa tay nếu cần."
                : ""));
            } else {
              pushInfo("Không hoàn tác được: " + trimText(outcome && outcome.error));
            }
          } catch (e) {}
          emit();
          return !!(outcome && outcome.ok);
        });
      } catch (e2) {
        return resolved(false);
      }
    }

    // Bam trich dan trong cau tra loi ([§12], [Data!C42], [slide 3]): chon + cuon toi cho do (writer/et/wpp.goto - chi doc).
    function gotoCitation(target) {
      try {
        var call = citationCommand(target);
        if (!call || state.running) { return resolved(false); }
        return exec(call.action, call.params).then(function (outcome) {
          if (!(outcome && outcome.ok)) {
            try { pushInfo("Không nhảy tới được " + trimText(target) + ": " + trimText(outcome && outcome.error)); } catch (e) {}
            emit();
          }
          return !!(outcome && outcome.ok);
        });
      } catch (e2) {
        return resolved(false);
      }
    }

    // "Chen tra loi": dan nguyen cau tra loi cuoi vao tai lieu (AskAiPane.OnInsert - chi o Word).
    function insertReply() {
      try {
        if (!canInsert()) { return resolved(false); }
        var text = state.lastReply;
        return exec(INSERT_ACTIONS[kind], { text: text }).then(function (outcome) {
          try {
            if (outcome && outcome.ok) {
              state.status = "Đã chèn vào tài liệu";
            } else {
              state.error = "Chèn thất bại: " + trimText(outcome && outcome.error);
              state.status = "";
            }
          } catch (e) {}
          emit();
          return !!(outcome && outcome.ok);
        });
      } catch (e2) {
        return resolved(false);
      }
    }

    // "Xoa" tren the ghi nho vua ghi (chat.py/_memory: state deleting -> deleted | failed).
    function forget(id) {
      var memoryId = trimText(id);
      try {
        if (!memoryId) { return resolved(false); }
        setNoteState(memoryId, "deleting");
        emit();
        var req = null;
        try {
          req = fetchFn(base + "memory/" + encodeURIComponent(memoryId), { method: "DELETE" });
        } catch (e) { req = null; }
        if (!req || typeof req.then !== "function") {
          setNoteState(memoryId, "failed");
          emit();
          return resolved(false);
        }
        return req.then(function (resp) {
          var status = 0;
          try { status = (resp && resp.status !== undefined) ? resp.status : 0; } catch (e2) {}
          setNoteState(memoryId, status >= 400 ? "failed" : "deleted");
          emit();
          return status < 400;
        }).then(null, function () {
          setNoteState(memoryId, "failed");
          emit();
          return false;
        });
      } catch (e3) {
        return resolved(false);
      }
    }

    // Bam "Thuc hien ke hoach" tren the ke hoach: chay that (tat che do ke hoach cho luot nay).
    function executePlan() {
      try {
        if (state.running || !state.planReady) { return resolved(false); }
        state.plan = false;
        state.planReady = false;
        emit();
        return send(PLAN_EXECUTE_PROMPT);
      } catch (e) {
        return resolved(false);
      }
    }

    function setNoteState(id, value) {
      try {
        for (var i = 0; i < state.notes.length; i++) {
          if (state.notes[i] && state.notes[i].id === id) { state.notes[i].state = value; }
        }
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
      quick: quick,
      loadQuickActions: loadQuickActions,
      confirm: confirm,
      choose: choose,
      stop: stop,
      newConversation: newConversation,
      getState: getState,
      kind: kind,
      setPlan: setPlan,
      toggleReasoning: toggleReasoning,
      executePlan: executePlan,
      undoLastTurn: undoLastTurn,
      gotoCitation: gotoCitation,
      insertReply: insertReply,
      forget: forget
    };
  }

  // ---- He thiet ke pane LibreOffice (axiom.theme): du lieu + nhan lenh (ES5) ----
  var ACTION_LABELS = {
    "writer.getText": "Đọc nội dung tài liệu",
    "writer.selection": "Đọc vùng chọn",
    "writer.newDocument": "Tạo tài liệu mới",
    "writer.open": "Mở tài liệu",
    "writer.typeText": "Gõ văn bản",
    "writer.appendText": "Viết thêm nội dung",
    "writer.replaceAll": "Thay thế văn bản",
    "writer.replaceMany": "Thay thế nhiều chỗ",
    "writer.outline": "Đọc dàn bài",
    "writer.restyle": "Trình bày lại tài liệu",
    "writer.insertBlocks": "Chèn khối trình bày",
    "writer.insertStyledText": "Chèn đoạn văn",
    "writer.formatSelection": "Định dạng vùng chọn",
    "writer.setParagraphAlignment": "Căn lề đoạn văn",
    "writer.insertTable": "Chèn bảng",
    "writer.insertPageBreak": "Ngắt trang",
    "writer.insertImage": "Chèn ảnh",
    "writer.insertHyperlink": "Chèn liên kết",
    "writer.heading": "Thêm tiêu đề",
    "writer.closeAll": "Đóng tài liệu",
    "writer.formatTable": "Định dạng bảng",
    "writer.checkTables": "Kiểm tra bảng",
    "et.listSheets": "Đọc danh sách sheet",
    "et.newWorkbook": "Tạo bảng tính mới",
    "et.open": "Mở bảng tính",
    "et.readRange": "Đọc vùng dữ liệu",
    "et.writeRange": "Ghi dữ liệu vào ô",
    "et.formatRange": "Định dạng ô",
    "et.activateSheet": "Chuyển sheet",
    "et.checkRange": "Kiểm tra bảng dữ liệu",
    "wpp.listSlides": "Đọc danh sách slide",
    "wpp.newPresentation": "Tạo bản trình chiếu mới",
    "wpp.open": "Mở bản trình chiếu",
    "wpp.addSlide": "Thêm slide",
    "wpp.addTextBox": "Thêm hộp văn bản",
    "wpp.addText": "Thêm chữ vào slide",
    "wpp.addImage": "Chèn ảnh vào slide",
    "wpp.addTable": "Thêm bảng vào slide",
    "wpp.setNotes": "Ghi chú thuyết trình",
    "wpp.deleteSlide": "Xoá slide",
    "wpp.checkLayout": "Kiểm tra bố cục slide"
  };
  var SUFFIX_LABELS = {
    "save": "Lưu tệp",
    "saveAs": "Lưu tệp",
    "exportPdf": "Xuất PDF",
    "undo": "Hoàn tác thao tác trước"
  };
  var TOOL_LABELS = {
    "load_skill": "Dùng kỹ năng",
    "read_skill_file": "Đọc tài liệu kỹ năng",
    "remember": "Ghi nhớ",
    "recall": "Tra ghi nhớ",
    "look_at_document": "Xem ảnh tài liệu"
  };
  var NOUNS = {
    "wps": "văn bản",
    "et": "bảng tính",
    "wpp": "bản trình chiếu"
  };
  var APP_NAMES = {
    "wps": "Writer",
    "et": "Calc",
    "wpp": "Impress"
  };
  var SUGGESTIONS = {
    "wps": ["Soạn đơn xin việc vị trí kế toán", "Tóm tắt tài liệu này thành 5 ý", "Chèn bảng lịch họp tuần 3 cột"],
    "et": ["Tạo bảng điểm 5 học sinh, có cột trung bình", "Thêm cột Tổng có công thức cho bảng này", "Tô đậm và kẻ khung dòng tiêu đề"],
    "wpp": ["Tạo 5 slide giới thiệu công ty", "Thêm slide kế hoạch quý 4", "Viết ghi chú thuyết trình cho từng slide"]
  };

  function labelFor(action) {
    try {
      if (!action) { return "Thao tác trên tài liệu"; }
      var key = String(action);
      if (Object.prototype.hasOwnProperty.call(ACTION_LABELS, key)) { return ACTION_LABELS[key]; }
      if (Object.prototype.hasOwnProperty.call(TOOL_LABELS, key)) { return TOOL_LABELS[key]; }
      var suffix = key;
      try { suffix = key.split('.').slice(-1)[0]; } catch (e) { suffix = key; }
      if (key.indexOf('.') >= 0 && Object.prototype.hasOwnProperty.call(SUFFIX_LABELS, suffix)) { return SUFFIX_LABELS[suffix]; }
      if (key.indexOf('.') >= 0) { return "Thao tác trên tài liệu"; }
      return key;
    } catch (e2) { return "Thao tác trên tài liệu"; }
  }

  function emptyState(kind) {
    try {
      var noun = Object.prototype.hasOwnProperty.call(NOUNS, kind) ? NOUNS[kind] : "tài liệu";
      var sugg = Object.prototype.hasOwnProperty.call(SUGGESTIONS, kind) ? SUGGESTIONS[kind] : SUGGESTIONS["wps"];
      return {
        title: "Bạn muốn làm gì với " + noun + " này?",
        description: "Tôi đọc và chỉnh sửa trực tiếp tài liệu đang mở. Thay đổi hiện ngay trên trang khi tôi làm.",
        suggestions: sugg.slice ? sugg.slice(0) : sugg
      };
    } catch (e) { return { title: "", description: "", suggestions: [] }; }
  }

  // citationCommand: trich dan -> lenh nhay toi ({action, params}); null khi khong phai trich dan.
  // "§12" -> writer.goto {paragraph: 12}; "slide 3" -> wpp.goto {slide: 3}; "Data!C42" -> et.goto {range}.
  function citationCommand(target) {
    var text = trimText(target);
    if (!text) { return null; }
    if (text.charAt(0) === "\u00A7") {
      var paragraph = parseInt(trimText(text.substring(1)), 10);
      return isNaN(paragraph) ? null : { action: "writer.goto", params: { paragraph: paragraph } };
    }
    var slide = /^slide\s+(\d+)$/i.exec(text);
    if (slide) { return { action: "wpp.goto", params: { slide: parseInt(slide[1], 10) } }; }
    if (text.indexOf("!") > 0) { return { action: "et.goto", params: { range: text } }; }
    return null;
  }

  function plainReply(text) {
    try {
      return String(text === undefined || text === null ? "" : text)
        .split("**").join("").split("\u0060").join("")
        .replace(/^\s+|\s+$/g, "");
    } catch (e) { return ""; }
  }


  // ---- Trang thai luot chay (chat.py: ChatSession) ----

  // Lenh sua tai lieu = lenh co `undo` trong registry (EDIT_ACTIONS sinh tu dong o tren, cung nguon voi
  // chat.py:is_edit va AskAiPane.IsWordEdit): moi lenh = dung mot buoc Undo cua bridge.
  function isEdit(action) {
    try {
      return Object.prototype.hasOwnProperty.call(EDIT_ACTIONS, trimText(action));
    } catch (e) { return false; }
  }

  // Dong trang thai o footer (chat.py:summary + AskAiPane.FinishRun).
  function summary(state) {
    try {
      var st = state || {};
      if (st.running) { return trimText(st.status) || "Đang chạy…"; }
      if (trimText(st.error)) { return "Lỗi · nhấn Cài đặt để kiểm tra Agent Core"; }
      if (trimText(st.status)) { return trimText(st.status); }
      var bits = [];
      var seconds = Math.floor(Number(st.seconds) || 0);
      if (seconds > 0) { bits.push("Xong trong " + seconds + "s"); }
      var edits = Math.floor(Number(st.edits) || 0);
      if (edits > 0) { bits.push(edits + " thao tác"); }
      return bits.length ? bits.join(" · ") : "Sẵn sàng";
    } catch (e) { return "Sẵn sàng"; }
  }

  // Dong xem truoc khoi suy luan khi thu lai (theme.reasoning_preview cua pane LibreOffice).
  function reasoningPreview(text, limit) {
    try {
      var max = limit || REASONING_PREVIEW_LIMIT;
      var flat = trimText(text).replace(/\s+/g, " ");
      return flat.length > max ? flat.substring(0, max - 1) + "…" : flat;
    } catch (e) { return ""; }
  }

  // Thong bao loi tren dong thao tac: bo ten kieu loi ("ArgumentException: ...") - theme.error_summary.
  function errorSummary(error) {
    try {
      var text = trimText(error).replace(/^[A-Za-z_.]*(Exception|Error):\s*/, "");
      return text.length > 160 ? text.substring(0, 157) + "…" : (text || "lỗi");
    } catch (e) { return "lỗi"; }
  }

  var api = { create: create, ACTION_LABELS: ACTION_LABELS, SUFFIX_LABELS: SUFFIX_LABELS, TOOL_LABELS: TOOL_LABELS, NOUNS: NOUNS, APP_NAMES: APP_NAMES, SUGGESTIONS: SUGGESTIONS, labelFor: labelFor, emptyState: emptyState, plainReply: plainReply, citationCommand: citationCommand, isEdit: isEdit, summary: summary, reasoningPreview: reasoningPreview, errorSummary: errorSummary, EDIT_ACTIONS: EDIT_ACTIONS, UNDO_ACTIONS: UNDO_ACTIONS, INSERT_ACTIONS: INSERT_ACTIONS, PLAN_EXECUTE_PROMPT: PLAN_EXECUTE_PROMPT, REASONING_INLINE_LIMIT: REASONING_INLINE_LIMIT };
  try {
    if (root) { root.AxiomPaneCore = api; }
  } catch (e) {}
})(typeof window !== "undefined" ? window : (typeof globalThis !== "undefined" ? globalThis : this));
