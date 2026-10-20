(function () {
  "use strict";

  var Dom = window.AxiomPaneDom;
  var $ = Dom.$;
  var textOf = Dom.textOf;
  var trimText = Dom.trimText;
  var clearNode = Dom.clearNode;
  var make = Dom.make;
  var baseUrl = Dom.baseUrl;
  var actOf = Dom.actOf;

  // ?quick=<id>: muc chon tu menu chuot phai - chay ngay khi mo. Khong co = "Hoi AI ve doan nay" (cho chon/go).
  var QUICK = "";
  try {
    var mq = /[?&]quick=([a-z]+)/.exec(textOf(window.location && window.location.search));
    if (mq) { QUICK = mq[1]; }
  } catch (e0) { QUICK = ""; }

  var Core = window.AxiomPaneCore;
  var ctl = null;
  try {
    ctl = Core.create({
      fetch: window.fetch,
      EventSource: window.EventSource,
      base: baseUrl(),
      kind: "wps",
      requireSelection: true,
      onChange: render
    });
  } catch (e1) { ctl = null; }

  function stepRow(step, running) {
    var label = "";
    try { label = Core.labelFor(step.action); } catch (e) { label = textOf(step.action); }
    var mark = "…";
    var cls = "step";
    if (step.ok === true) { mark = "✓"; cls = "step stepOk"; }
    else if (step.ok === false) { mark = "✕"; cls = "step stepFail"; }
    else if (!running) { mark = "·"; }
    var text = mark + " " + label;
    if (step.ok === false && step.error) {
      try { text += " — " + Core.errorSummary(step.error); } catch (e2) {}
    }
    return make("div", cls, text);
  }

  function renderBody(state) {
    var body = $("miniBody");
    if (!body) { return; }
    clearNode(body);
    var messages = state.messages || [];
    if (!messages.length) {
      body.appendChild(make("div", "miniHint", "Chọn việc cần làm với đoạn đang bôi đen, hoặc gõ yêu cầu bên dưới."));
      var chips = make("div", "chips");
      var list = state.quickActions || [];
      for (var i = 0; i < list.length; i++) {
        var chip = make("button", "chip", list[i].label);
        try {
          chip.type = "button";
          chip.setAttribute("data-act", "quick");
          chip.setAttribute("data-value", list[i].id);
        } catch (e) {}
        chips.appendChild(chip);
      }
      body.appendChild(chips);
      return;
    }
    // Cac buoc cua luot dang chay/vua xong nam ngay sau tin nhan cuoi cua nguoi dung.
    var lastUser = -1;
    for (var k = 0; k < messages.length; k++) { if (messages[k].role === "user") { lastUser = k; } }
    for (var j = 0; j < messages.length; j++) {
      var m = messages[j];
      if (m.role === "user") { body.appendChild(make("div", "you", m.text)); }
      else if (m.role === "assistant") {
        var reply = m.text;
        try { reply = Core.plainReply(m.text); } catch (e3) {}
        body.appendChild(make("div", "reply", reply));
      } else if (m.role === "error") { body.appendChild(make("div", "error", m.text)); }
      else { body.appendChild(make("div", "info", m.text)); }
      if (j === lastUser) {
        var steps = state.steps || [];
        for (var s = 0; s < steps.length; s++) { body.appendChild(stepRow(steps[s], state.running)); }
        if (state.running && state.progress) { body.appendChild(make("div", "progress", state.progress)); }
      }
    }
    if (state.confirmation) {
      var card = make("div", "confirm", state.confirmation.reason || "AI cần bạn xác nhận trước khi làm tiếp.");
      var row = make("div", "row");
      var yes = make("button", "btnPrimary", "Đồng ý");
      var no = make("button", "link", "Không");
      try { yes.type = "button"; yes.setAttribute("data-act", "approve"); } catch (e4) {}
      try { no.type = "button"; no.setAttribute("data-act", "deny"); } catch (e5) {}
      row.appendChild(yes);
      row.appendChild(no);
      card.appendChild(row);
      body.appendChild(card);
    }
    try { body.scrollTop = body.scrollHeight; } catch (e6) {}
  }

  function render(state) {
    try {
      state = state || {};
      renderBody(state);
      var model = $("miniModel");
      if (model) { clearNode(model); model.appendChild(document.createTextNode(trimText(state.model))); }
      var status = $("miniStatus");
      if (status) {
        var text = state.running ? (state.status || "Đang chạy…") : "";
        if (!state.running) { try { text = Core.summary(state); } catch (e) { text = "Sẵn sàng"; } }
        clearNode(status);
        status.appendChild(document.createTextNode(text));
      }
      var undo = $("miniUndo");
      if (undo) {
        if (state.canUndo) { undo.removeAttribute("hidden"); } else { undo.setAttribute("hidden", "hidden"); }
      }
      var stop = $("miniStop");
      if (stop) {
        if (state.running) { stop.removeAttribute("hidden"); } else { stop.setAttribute("hidden", "hidden"); }
      }
      updateSend(state);
    } catch (e2) {}
  }

  function updateSend(state) {
    var send = $("miniSend");
    var input = $("miniInput");
    if (!send) { return; }
    var empty = !trimText(input ? input.value : "");
    try { send.disabled = !!(state && state.running) || empty; } catch (e) {}
  }

  function current() {
    try { return ctl ? ctl.getState() : {}; } catch (e) { return {}; }
  }

  function doSend() {
    var input = $("miniInput");
    var text = trimText(input ? input.value : "");
    if (!text || !ctl || current().running) { return; }
    try { input.value = ""; } catch (e) {}
    ctl.send(text);
  }

  // Dong: ShowDialog cua WPS la mot cua so CEF rieng - window.close() dong no. Dang chay thi dung luot truoc, de khong
  // con luot chay ngam sua tai lieu sau khi nguoi dung da dong khung.
  function closeMini() {
    try { if (ctl && current().running) { ctl.stop(); } } catch (e) {}
    try { window.close(); } catch (e2) {}
  }

  function onAction(found) {
    if (!ctl) { return; }
    if (found.act === "send") { doSend(); }
    else if (found.act === "quick") { ctl.quick(found.value); }
    else if (found.act === "undo") { ctl.undoLastTurn(); }
    else if (found.act === "stop") { ctl.stop(); }
    else if (found.act === "approve") { ctl.confirm(true); }
    else if (found.act === "deny") { ctl.confirm(false); }
    else if (found.act === "close") { closeMini(); }
  }

  try {
    var page = document.body;
    page.addEventListener("click", function (ev) {
      var found = null;
      try { found = actOf(ev.target || ev.srcElement); } catch (e) { found = null; }
      if (found) { onAction(found); }
    });
    page.addEventListener("keydown", function (ev) {
      var code = ev.keyCode || ev.which;
      var target = ev.target || ev.srcElement;
      if (code === 27) { closeMini(); return; }
      if (code === 13 && !ev.shiftKey && target && target.id === "miniInput") {
        try { ev.preventDefault(); } catch (e) {}
        doSend();
      }
    });
    var input = $("miniInput");
    if (input) { input.addEventListener("input", function () { updateSend(current()); }); }
  } catch (e7) {}

  render({ messages: [], running: false, steps: [] });
  if (ctl) {
    // Doi danh sach muc ve truoc: chip can nhan, va bong bong cua ?quick= hien "Trau chuot" chu khong phai "polish".
    var go = function () {
      if (QUICK) { ctl.quick(QUICK); }
      else { try { $("miniInput").focus(); } catch (e) {} }
    };
    try { ctl.loadQuickActions().then(go, go); } catch (e8) { go(); }
  }
})();
