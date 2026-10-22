(function () {
  "use strict";

  var Dom = window.AxiomPaneDom;
  var $ = Dom.$;
  var textOf = Dom.textOf;
  var trimText = Dom.trimText;
  var clearNode = Dom.clearNode;
  var addText = Dom.addText;
  var baseUrl = Dom.baseUrl;
  var actOf = Dom.actOf;

  var KIND = "wps";
  var VIEW = "";
  var VIEW_CHECK = "";
  var VIEW_DO = "";
  var search = "";
  try {
    try { search = window.location ? window.location.search : ""; } catch (e0) { search = ""; }
    var m = /[?&]app=(wps|et|wpp)(?:&|$)/.exec(search || "");
    if (m) { KIND = m[1]; }
    // ?view=<ten>: ribbon mo thang mot man hinh phu (main.js: paneUrl). Danh sach nay phai khop
    // main.js:VIEWS - mot ten o do ma khong co o day thi nut ribbon mo ra pane trong.
    // "conversations" chi co o day: no nam trong menu "..." cua pane, ribbon khong co nut rieng.
    var mv = /[?&]view=(memory|audit|setup|skills|status|check|log|conversations)(?:&|$)/.exec(search || "");
    if (mv) { VIEW = mv[1]; }
    // ?check=<lenh>: ba nut "Soat ..." tren ribbon dung chung mot man hinh, nen man hinh phai biet
    // DUNG lenh nao (main.js: CHECK_COMMANDS).
    var mc = /[?&]check=([A-Za-z0-9_.]+)(?:&|$)/.exec(search || "");
    if (mc) { VIEW_CHECK = mc[1]; }
    // ?do=<viec>: viec man hinh phai lam ngay khi mo (menu "Khac" cua ribbon: main.js:DO).
    var md = /[?&]do=(testModel|copyUrl)(?:&|$)/.exec(search || "");
    if (md) { VIEW_DO = md[1]; }
  } catch (e) { KIND = "wps"; VIEW = ""; VIEW_CHECK = ""; VIEW_DO = ""; }

  var ctl = null;
  try {
    ctl = AxiomPaneCore.create({
      fetch: window.fetch,
      EventSource: window.EventSource,
      base: baseUrl(),
      kind: KIND,
      onChange: render
    });
  } catch (e5) {
    ctl = null;
  }

  function appName() {
    try {
      if (AxiomPaneCore.APP_NAMES && AxiomPaneCore.APP_NAMES[KIND]) { return AxiomPaneCore.APP_NAMES[KIND]; }
    } catch (e) {}
    return KIND;
  }

  // updateHint: chon cau goi y VUA cho trong chan o soan (theme.py PROMPT_HINT / PROMPT_HINT_SHORT). Do cho TRONG
  // THAT chu khong dung nguong co dinh: nguong 340px cu tung lam cau day du xuong hai dong va dinh vao nut ben canh
  // (anh chup 20/10/2026) - chan o soan them hay bot mot nut la nguong co dinh sai ngay.
  function setHint(hint, text) {
    while (hint.firstChild) { hint.removeChild(hint.firstChild); }
    hint.appendChild(document.createTextNode(text));
  }

  // fits: chu co nam tron trong o khong. null = khong do duoc (pane chua ve, hoac khong co layout).
  function fits(el) {
    try {
      if (!el.clientWidth && !el.scrollWidth) { return null; }
      return el.scrollWidth <= el.clientWidth + 1;
    } catch (e) { return null; }
  }

  function updateHint() {
    try {
      var hint = $("composerHint");
      if (!hint) { return; }
      var full = "Enter để gửi · Shift+Enter xuống dòng";
      var short = "Enter để gửi";
      setHint(hint, full);
      var ok = fits(hint);
      if (ok === null) {
        // Chua do duoc: doan theo be rong o soan.
        var w = 0;
        try {
          var box = $("composerBox");
          if (box && box.offsetWidth) { w = box.offsetWidth; }
          else if (window.innerWidth) { w = window.innerWidth; }
        } catch (e) { w = 0; }
        if (w > 0 && w < 340) { setHint(hint, short); }
        return;
      }
      if (ok) { return; }
      setHint(hint, short);
      if (fits(hint) === false) { setHint(hint, ""); }
    } catch (e2) {}
  }

  function updateSendDisabled(state) {
    try {
      var sendBtn = $("sendBtn");
      if (!sendBtn) { return; }
      var empty = true;
      try {
        var input = $("input");
        var v = input ? input.value : "";
        empty = !String(v).replace(/^\s+|\s+$/g, "");
      } catch (e) { empty = true; }
      sendBtn.disabled = !!state.running || empty;
    } catch (e2) {}
  }

  // The phu trong khung chat (Windows: PaneControls.cs - ReasoningCard / MemoryCard / PlanCard).
  function makeCard(titleText, cardClass, titleClass) {
    var card = document.createElement("div");
    try { card.className = "card" + (cardClass ? " " + cardClass : ""); } catch (e) {}
    var title = document.createElement("div");
    try { title.className = "cardTitle" + (titleClass ? " " + titleClass : ""); } catch (e2) {}
    addText(title, titleText);
    card.appendChild(title);
    return card;
  }

  function makeCardLink(titleEl, label, act, value) {
    var btn = document.createElement("button");
    try {
      btn.type = "button";
      btn.className = "cardLink";
      btn.setAttribute("data-act", act);
      if (value) { btn.setAttribute("data-value", value); }
    } catch (e) {}
    addText(btn, label);
    try { titleEl.appendChild(btn); } catch (e2) {}
    return btn;
  }

  function makeReasonCard(reasoning) {
    var round = 0;
    try { round = Number(reasoning.round) || 0; } catch (e) {}
    var card = makeCard("Suy luận · vòng " + round);
    var expanded = false;
    try { expanded = !!reasoning.expanded; } catch (e2) {}
    var body = document.createElement("div");
    try { body.className = "cardBody"; } catch (e3) {}
    var text = textOf(reasoning && reasoning.text);
    if (!expanded) {
      // Thu lai: chi xem truoc (theme.reasoning_preview cua pane LibreOffice).
      try { text = AxiomPaneCore.reasoningPreview(text); } catch (e4) {}
    }
    addText(body, text);
    card.appendChild(body);
    makeCardLink(card.firstChild, expanded ? "Ẩn" : "Hiện", "reason");
    return card;
  }

  function makeSkillRow(skill) {
    var row = document.createElement("div");
    try { row.className = "toolRow"; } catch (e) {}
    var icon = document.createElement("span");
    try { icon.className = "toolIcon iconOk"; } catch (e2) {}
    addText(icon, "✓");
    row.appendChild(icon);
    var lab = document.createElement("span");
    try { lab.className = "toolLabel"; } catch (e3) {}
    addText(lab, "Dùng kỹ năng: " + textOf(skill && skill.name));
    row.appendChild(lab);
    return row;
  }

  function makeNoteCard(note) {
    var card = makeCard("Đã ghi nhớ");
    var row = document.createElement("div");
    try { row.className = "memRow"; } catch (e) {}
    var state = "saved";
    try { state = String((note && note.state) || "saved"); } catch (e2) {}
    var text = document.createElement("span");
    try { text.className = state === "deleted" ? "memText memGone" : "memText"; } catch (e3) {}
    addText(text, textOf(note && note.text));
    row.appendChild(text);
    if (state === "deleting" || state === "deleted" || state === "failed") {
      var flag = document.createElement("span");
      try { flag.className = "memText"; } catch (e4) {}
      addText(flag, state === "deleting" ? "Đang xoá…" : (state === "deleted" ? "Đã xoá" : "Không xoá được"));
      row.appendChild(flag);
    }
    card.appendChild(row);
    if (state === "saved") {
      makeCardLink(card.firstChild, "Xoá", "forget", textOf(note && note.id));
    }
    return card;
  }

  function makePlanCard(state) {
    var card = makeCard("Kế hoạch đã sẵn sàng - tài liệu chưa bị sửa", "planCard", "planTitle");
    var body = document.createElement("div");
    try { body.className = "cardBody"; } catch (e) {}
    addText(body, textOf(state && state.lastReply));
    card.appendChild(body);
    makeCardLink(card.firstChild, "Thực hiện kế hoạch", "planExec");
    return card;
  }

  function makeBubble(kind) {
    var div = document.createElement("div");
    try { div.className = "bubble " + kind; } catch (e) {}
    return div;
  }

  function makeToolRow(step, running) {
    var row = document.createElement("div");
    try { row.className = "toolRow"; } catch (e) {}
    var icon = document.createElement("span");
    var action = "";
    try { action = String((step && step.action) || ""); } catch (e2) {}
    var failed = false;
    try { failed = !running && step && !step.ok; } catch (e3) {}
    var label = "";
    try { label = AxiomPaneCore.labelFor(action); } catch (e4) { label = action; }
    try {
      if (step && step.ok) {
        icon.className = "toolIcon iconOk";
        icon.appendChild(document.createTextNode("✓"));
      } else if (failed) {
        icon.className = "toolIcon iconErr";
        icon.appendChild(document.createTextNode("✕"));
      } else {
        icon.className = "toolIcon iconRun";
        icon.appendChild(document.createTextNode("…"));
      }
    } catch (e5) {}
    row.appendChild(icon);
    var lab = document.createElement("span");
    try { lab.className = failed ? "toolLabel toolLabelErr" : "toolLabel"; } catch (e6) {}
    addText(lab, label);
    row.appendChild(lab);
    var code = document.createElement("span");
    try { code.className = "toolCode"; } catch (e7) {}
    addText(code, action);
    row.appendChild(code);
    return row;
  }

  function makeErrorCard(text) {
    var card = document.createElement("div");
    try { card.className = "errCard"; } catch (e) {}
    try { card.setAttribute("role", "alert"); } catch (e2) {}
    var title = document.createElement("div");
    try { title.className = "errTitle"; } catch (e3) {}
    addText(title, "Có lỗi xảy ra");
    card.appendChild(title);
    var body = document.createElement("div");
    try { body.className = "errBody"; } catch (e4) {}
    addText(body, text);
    card.appendChild(body);
    var links = document.createElement("div");
    try { links.className = "btnRow"; } catch (e5) {}
    var retry = document.createElement("button");
    try {
      retry.type = "button";
      retry.className = "cardLink";
      retry.setAttribute("data-act", "retry");
    } catch (e6) {}
    addText(retry, "Thử lại");
    links.appendChild(retry);
    var settings = document.createElement("button");
    try {
      settings.type = "button";
      settings.className = "cardLink";
      settings.setAttribute("data-act", "view");
      settings.setAttribute("data-value", "setup");
    } catch (e7) {}
    addText(settings, "Mở Cài đặt");
    links.appendChild(settings);
    card.appendChild(links);
    return card;
  }

  function makeConfirmCard(confirmation) {
    var card = document.createElement("div");
    try { card.className = "confirmCard"; } catch (e) {}
    var label = "";
    try { label = AxiomPaneCore.labelFor(confirmation ? confirmation.action : ""); } catch (e2) {}
    var choices = null;
    try { choices = confirmation && confirmation.choices && confirmation.choices.length ? confirmation.choices : null; } catch (eC) { choices = null; }
    var title = document.createElement("div");
    try { title.id = "confirmTitle"; } catch (e3) {}
    addText(title, (choices ? "Cần bạn chọn: " : "Cần bạn xác nhận: ") + label);
    card.appendChild(title);
    var reason = document.createElement("p");
    try { reason.className = "confirmReason"; } catch (e4) {}
    var rtext = "";
    try { rtext = String((confirmation && confirmation.reason) || ""); } catch (e5) {}
    addText(reason, rtext);
    card.appendChild(reason);
    if (choices) {
      // Thẻ nhiều lựa chọn: mỗi lựa chọn một nút + dòng mô tả, cuối cùng "Bỏ qua".
      for (var ci = 0; ci < choices.length; ci++) {
        if (choices[ci].input) {
          // Ô tự gõ: nhãn, ô nhập (gợi ý = mô tả) và nút Gửi.
          var head = document.createElement("div");
          try { head.className = "confirmChoiceTitle"; head.style.marginTop = "8px"; head.style.fontWeight = "600"; } catch (eH) {}
          addText(head, choices[ci].label);
          card.appendChild(head);
          var typeRow = document.createElement("div");
          try { typeRow.className = "btnRow"; } catch (eR) {}
          var field = document.createElement("input");
          try {
            field.type = "text";
            field.id = "choiceText";
            field.setAttribute("placeholder", choices[ci].detail || "");
            field.setAttribute("aria-label", choices[ci].label);
            field.style.flex = "1";
          } catch (eF) {}
          typeRow.appendChild(field);
          var send = document.createElement("button");
          try {
            send.type = "button";
            send.className = "btnSecondary";
            send.setAttribute("data-act", "chooseText");
            send.setAttribute("data-value", choices[ci].id);
          } catch (eB) {}
          addText(send, "Gửi");
          typeRow.appendChild(send);
          card.appendChild(typeRow);
          continue;
        }
        var pick = document.createElement("button");
        try {
          pick.type = "button";
          pick.className = ci === 0 ? "btnPrimary" : "btnSecondary";
          pick.setAttribute("data-act", "choose");
          pick.setAttribute("data-value", choices[ci].id);
          pick.style.display = "block";
          pick.style.marginTop = "6px";
        } catch (eP) {}
        addText(pick, choices[ci].label);
        card.appendChild(pick);
        if (choices[ci].detail) {
          var note = document.createElement("p");
          try { note.className = "confirmReason"; } catch (eN) {}
          addText(note, choices[ci].detail);
          card.appendChild(note);
        }
      }
      var skip = document.createElement("button");
      try {
        skip.type = "button";
        skip.className = "link";
        skip.setAttribute("data-act", "deny");
      } catch (eS) {}
      addText(skip, "Bỏ qua");
      card.appendChild(skip);
      return card;
    }
    var row = document.createElement("div");
    try { row.className = "btnRow"; } catch (e6) {}
    var ok = document.createElement("button");
    try {
      ok.type = "button";
      ok.className = "btnPrimary";
      ok.setAttribute("data-act", "approve");
    } catch (e7) {}
    addText(ok, "Đồng ý");
    row.appendChild(ok);
    var no = document.createElement("button");
    try {
      no.type = "button";
      no.className = "btnSecondary";
      no.setAttribute("data-act", "deny");
    } catch (e8) {}
    addText(no, "Từ chối");
    row.appendChild(no);
    card.appendChild(row);
    return card;
  }

  function makeEmpty() {
    var frag = document.createDocumentFragment();
    var info = null;
    try { info = AxiomPaneCore.emptyState(KIND); } catch (e) { info = null; }
    if (!info) { return frag; }
    var title = document.createElement("div");
    try { title.id = "emptyTitle"; title.className = "emptyTitle"; } catch (e2) {}
    addText(title, info.title);
    frag.appendChild(title);
    var desc = document.createElement("p");
    try { desc.className = "emptyDesc"; } catch (e3) {}
    addText(desc, info.description);
    frag.appendChild(desc);
    var lab = document.createElement("p");
    try { lab.className = "suggestLabel"; } catch (e4) {}
    addText(lab, "GỢI Ý");
    frag.appendChild(lab);
    var sugg = info.suggestions || [];
    for (var i = 0; i < sugg.length; i++) {
      var chip = document.createElement("button");
      var text = "";
      try { text = String(sugg[i]); } catch (e5) {}
      try {
        chip.type = "button";
        chip.className = "chip";
        chip.setAttribute("data-act", "suggestion");
        chip.setAttribute("data-value", text);
        // Goi y chi DIEN vao o soan, khong gui ngay: noi ro de nguoi dung biet minh con sua duoc.
        chip.setAttribute("title", "Điền câu này vào ô soạn");
      } catch (e6) {}
      // Mui ten cho biet bam vao la DIEN VAO O SOAN chu khong gui ngay (khac nut Gui).
      var arrow = document.createElement("span");
      try { arrow.className = "chipArrow"; arrow.setAttribute("aria-hidden", "true"); } catch (e7) {}
      addText(arrow, "→");
      var body = document.createElement("span");
      try { body.className = "chipText"; } catch (e8) {}
      addText(body, text);
      chip.appendChild(body);
      chip.appendChild(arrow);
      frag.appendChild(chip);
    }
    return frag;
  }

  // ---- Man hinh phu cua pane (Windows: MemoryForm / AuditForm / SetupWizardForm; LibreOffice: dialogs.py) ----
  // Moi duong deu di qua kenh /wps/v1/* (cung goc voi Core, khong token) - xem docs/architecture/wps-bridge.md muc 7.

  var view = { name: "", busy: false, note: "", scope: "all", data: null, fields: null,
    check: "", todo: "", lines: 0, detail: "" };

  function api(path, method, body) {
    var opts = { method: method || "GET", headers: {} };
    if (body) {
      opts.headers["Content-Type"] = "application/json";
      opts.body = JSON.stringify(body);
    }
    return window.fetch(baseUrl() + path, opts).then(function (res) {
      return res.json().then(
        function (data) { return { status: res.status, data: data }; },
        function () { return { status: res.status, data: null }; });
    }, function () { return { status: 0, data: null }; });
  }

  function apiError(reply) {
    if (!reply) { return "Không gọi được Agent Core"; }
    if (reply.status === 0) { return "Không kết nối được Agent Core"; }
    if (reply.data && reply.data.error) { return textOf(reply.data.error); }
    return "Agent Core trả lỗi " + reply.status;
  }

  function resultOf(reply) {
    try {
      if (reply && reply.data && reply.data.ok && reply.data.result) { return reply.data.result; }
    } catch (e) {}
    return null;
  }

  // ---- Menu "..." tren header (Word Copilot: tieu de + mot nut ... o goc phai) ----
  // Chi la mot khoi hien/an trong trang, khong phai menu cua he dieu hanh: pane chay trong CEF nen tu lo
  // phan mo/dong (bam ra ngoai hoac Esc thi dong - xem phan boot o cuoi file).

  function menuOpen() {
    var menu = $("moreMenu");
    if (!menu) { return false; }
    try { return !menu.hasAttribute("hidden"); } catch (e) { return false; }
  }

  function setMenu(open) {
    var menu = $("moreMenu");
    if (menu) {
      try {
        if (open) { menu.removeAttribute("hidden"); }
        else { menu.setAttribute("hidden", "hidden"); }
      } catch (e) {}
    }
    var btn = $("moreBtn");
    if (btn) {
      try { btn.setAttribute("aria-expanded", open ? "true" : "false"); } catch (e2) {}
    }
  }

  // ---- Menu chuot phai -> khung chat nay (api/wpspane.go) ----
  // Nguoi dung boi den, bam chuot phai, chon "Trau chuot": main.js hien khung chat roi nho Core chuyen muc do toi day
  // (ribbon khong noi thang duoc vao trang nay - docs/architecture/ribbon-ui.md muc 7). Khung chat nghe SSE pane/events va chay
  // muc do trong CHINH hoi thoai dang mo. Day la loi vao DUY NHAT cua "Sua bang AI" - khung chat khong ve menu rieng
  // (20/10/2026, theo yeu cau nguoi dung). Chi khung chat cua Writer nghe: man hinh phu (?view=) va Calc/Impress
  // khong co menu nay. EventSource tu noi lai khi Core khoi dong lai.
  var quickReady = null;

  function listenPane() {
    try {
      if (VIEW || KIND !== "wps" || !ctl || typeof window.EventSource !== "function") { return; }
      var source = new window.EventSource(baseUrl() + "pane/events");
      source.addEventListener("pane.quick", function (evt) {
        var id = "";
        try {
          var data = JSON.parse(evt.data);
          id = trimText((data && data.data && data.data.quickAction) || (data && data.quickAction));
        } catch (e) { id = ""; }
        if (!id) { return; }
        // Khung chat vua tao: doi danh sach muc ve truoc de bong bong hien NHAN ("Trau chuot") chu khong phai id.
        var go = function () { try { ctl.quick(id); } catch (e2) {} };
        try {
          if (quickReady && typeof quickReady.then === "function") { quickReady.then(go, go); return; }
        } catch (e3) {}
        go();
      });
    } catch (e4) {}
  }

  function openView(name, check, action) {
    view.name = name;
    view.busy = true;
    view.note = "";
    view.data = null;
    view.fields = null;
    view.check = name === "check" ? textOf(check) : "";
    view.todo = textOf(action);
    view.lines = 0;
    view.detail = "";
    renderView();
    loadView();
  }

  function closeView() {
    view.name = "";
    view.busy = false;
    view.note = "";
    view.data = null;
    view.fields = null;
    view.check = "";
    view.todo = "";
    view.lines = 0;
    view.detail = "";
    renderView();
  }

  function loadView() {
    var name = view.name;
    if (name === "memory") {
      var query = view.scope && view.scope !== "all" ? "?scope=" + encodeURIComponent(view.scope) : "";
      api("memory" + query).then(function (reply) {
        if (view.name !== "memory") { return; }
        var result = resultOf(reply);
        view.busy = false;
        if (!result) { view.note = apiError(reply); } else { view.data = result.memories || []; }
        renderView();
      }, function () { view.busy = false; view.note = "Không kết nối được Agent Core"; renderView(); });
      return;
    }
    if (name === "audit") {
      api("audit").then(function (reply) {
        if (view.name !== "audit") { return; }
        var result = resultOf(reply);
        view.busy = false;
        if (!result) { view.note = apiError(reply); } else { view.data = result.calls || []; }
        renderView();
      }, function () { view.busy = false; view.note = "Không kết nối được Agent Core"; renderView(); });
      return;
    }
    if (name === "setup") {
      api("setup").then(function (reply) {
        if (view.name !== "setup") { return; }
        var result = resultOf(reply);
        view.busy = false;
        if (!result) { view.note = apiError(reply); } else { view.data = result; }
        renderView();
      }, function () { view.busy = false; view.note = "Không kết nối được Agent Core"; renderView(); });
      return;
    }
    if (name === "skills") {
      // app=<KIND>: ban Windows loc theo loai tai lieu dang mo (AdminForms.cs: SkillsForm).
      api("skills?app=" + encodeURIComponent(KIND)).then(function (reply) {
        if (view.name !== "skills") { return; }
        var result = resultOf(reply);
        view.busy = false;
        if (!result) { view.note = apiError(reply); } else { view.data = result; }
        renderView();
      }, function () { view.busy = false; view.note = "Không kết nối được Agent Core"; renderView(); });
      return;
    }
    if (name === "status") {
      api("status").then(function (reply) {
        if (view.name !== "status") { return; }
        var result = resultOf(reply);
        view.busy = false;
        if (!result) { view.note = apiError(reply); } else { view.data = result; }
        renderView();
        // ?do=... chi chay SAU khi co du lieu (can cong bridge / model de lam viec).
        runStatusTodo();
      }, function () { view.busy = false; view.note = "Không kết nối được Agent Core"; renderView(); });
      return;
    }
    if (name === "check") {
      runCheck();
      return;
    }
    if (name === "conversations") {
      // "Hoi thoai truoc": danh sach (GET /v1/conversations) roi mo mot hoi thoai
      // (GET /v1/conversations/{id} tra ve ca messages[]). Chi DOC - khong dung toi luong chat dang chay.
      if (view.detail) { loadConversation(view.detail); return; }
      api("conversations?limit=30").then(function (reply) {
        if (view.name !== "conversations" || view.detail) { return; }
        var result = resultOf(reply);
        view.busy = false;
        if (!result) { view.note = apiError(reply); } else { view.data = result.conversations || []; }
        renderView();
      }, function () { view.busy = false; view.note = "Không kết nối được Agent Core"; renderView(); });
      return;
    }
    if (name === "log") {
      // Nho so dong da xin: "Xem them" xin nhieu hon lan truoc (log.go tra ve N dong CUOI).
      var want = view.lines || DEFAULT_LOG_LINES;
      view.lines = want;
      api("log?lines=" + want).then(function (reply) {
        if (view.name !== "log") { return; }
        var result = resultOf(reply);
        view.busy = false;
        if (!result) { view.note = apiError(reply); } else { view.data = result; }
        renderView();
      }, function () { view.busy = false; view.note = "Không kết nối được Agent Core"; renderView(); });
      return;
    }
  }

  // ---- Vỏ chung của màn hình phụ -------------------------------------------------------------
  // Trước đây mỗi màn hình tự dựng tiêu đề rồi tự thêm nút "Đóng" ở cuối, nên thanh tiêu đề trôi mất khi
  // cuộn và người dùng phải cuộn hết danh sách mới đóng được. Nay mọi màn hình nằm trong một khung có
  // thanh tiêu đề DÍNH trên cùng (tiêu đề + nút đóng + Esc), phần thân cuộn riêng, chân dính dưới cùng.

  function viewSection(box, title) {
    var head = document.createElement("div");
    try { head.className = "sectionTitle"; } catch (e) {}
    addText(head, title);
    box.appendChild(head);
    return head;
  }

  // Tiêu đề màn hình nằm ở thanh DÍNH trên cùng (#viewHeading), không trôi theo danh sách.
  function viewTitle(box, text) {
    var head = $("viewHeading");
    if (head) {
      clearNode(head);
      addText(head, text);
      return;
    }
    var title = document.createElement("div");
    try { title.className = "viewTitle"; } catch (e) {}
    addText(title, text);
    box.appendChild(title);
  }

  function viewNote(box, text) {
    var note = document.createElement("p");
    try { note.className = "viewNote"; } catch (e) {}
    addText(note, text);
    box.appendChild(note);
  }

  function viewButton(row, label, act, value, primary) {
    var btn = document.createElement("button");
    try {
      btn.type = "button";
      btn.className = primary ? "btnPrimary" : "btnSecondary";
      btn.setAttribute("data-act", act);
      if (value) { btn.setAttribute("data-value", value); }
    } catch (e) {}
    addText(btn, label);
    row.appendChild(btn);
    return btn;
  }

  function viewFoot(box, extra) {
    var foot = document.createElement("div");
    try { foot.className = "viewFoot"; } catch (e) {}
    try { foot.setAttribute("role", "group"); } catch (e2) {}
    if (extra) { foot.appendChild(extra); }
    var row = document.createElement("div");
    try { row.className = "btnRow"; } catch (e2) {}
    var close = document.createElement("button");
    try {
      close.type = "button";
      close.className = "btnSecondary";
      close.setAttribute("data-act", "closeView");
    } catch (e3) {}
    addText(close, "Đóng");
    row.appendChild(close);
    foot.appendChild(row);
    box.appendChild(foot);
  }

  function scopeLabel(scope) {
    if (scope === "user") { return "Người dùng"; }
    if (scope === "document") { return "Tài liệu"; }
    if (scope === "skill") { return "Kỹ năng"; }
    return textOf(scope);
  }

  function renderMemoryView(box) {
    viewTitle(box, "Bộ nhớ AI");
    // Bộ lọc phạm vi đứng TRƯỚC dòng ghi chú: nó là điều khiển của màn hình, không phải thông tin phụ.
    var segs = document.createElement("div");
    try { segs.className = "segRow"; } catch (e0) {}
    segs.setAttribute("role", "group");
    segs.setAttribute("aria-label", "Lọc theo phạm vi");
    var scopes = [["all", "Tất cả"], ["user", "Người dùng"], ["document", "Tài liệu"]];
    for (var i = 0; i < scopes.length; i++) {
      var seg = document.createElement("button");
      var on = view.scope === scopes[i][0];
      try {
        seg.type = "button";
        seg.className = "segBtn" + (on ? " segBtnOn" : "");
        seg.setAttribute("data-act", "memScope");
        seg.setAttribute("data-value", scopes[i][0]);
        // Trang thai chon phai duoc bao cho trinh doc man hinh, khong chi bang mau.
        seg.setAttribute("aria-pressed", on ? "true" : "false");
      } catch (e2) {}
      addText(seg, scopes[i][1]);
      segs.appendChild(seg);
    }
    box.appendChild(segs);

    if (view.note) { viewNote(box, view.note); }
    if (view.busy) { viewNote(box, "Đang tải…"); return; }
    var items = view.data || [];
    if (items.length === 0) {
      viewNote(box, view.scope === "all"
        ? "Chưa có gì được ghi nhớ. AI ghi nhớ sở thích và ngữ cảnh sau mỗi lượt trò chuyện."
        : "Không có mục nào trong phạm vi này.");
    } else {
      viewNote(box, items.length + " mục · xoá tạm thì khôi phục được, xoá vĩnh viễn thì không");
    }

    // Hai nhom tach bang duong ke: muc dang dung va muc da xoa tam - truoc day lan lon cung mot danh sach
    // nen nguoi dung khong biet con bao nhieu muc AI thuc su dang dung.
    var live = [];
    var gone = [];
    for (var q = 0; q < items.length; q++) {
      if (items[q] && items[q].deletedAt) { gone.push(items[q]); } else { live.push(items[q]); }
    }
    appendMemoryGroup(box, live, false);
    appendMemoryGroup(box, gone, true);

    var extra = document.createElement("div");
    try { extra.className = "btnRow"; } catch (e8) {}
    viewButton(extra, "Tải lại", "memReload", "", false);
    // Xoa vinh vien la hanh dong khong khoi phuc duoc: to mau nguy hiem, tach khoi nut thuong.
    var purge = viewButton(extra, "Xoá vĩnh viễn…", "memPurge", view.scope, false);
    try { purge.className = "btnDanger"; } catch (e9) {}
    viewFoot(box, extra);
  }

  // Mot nhom muc ghi nho. Moi muc: chu (muc da xoa thi gach ngang) + dong meta + nut hanh dong rieng.
  function appendMemoryGroup(box, items, deleted) {
    if (items.length === 0) { return; }
    viewSection(box, deleted ? "Đã xoá tạm (" + items.length + ")" : "Đang dùng (" + items.length + ")");
    for (var k = 0; k < items.length; k++) {
      var item = items[k] || {};
      var card = document.createElement("div");
      try { card.className = "card memCard" + (deleted ? " memCardGone" : ""); } catch (e3) {}
      var line = document.createElement("div");
      try { line.className = "memRow"; } catch (e4) {}
      var text = document.createElement("span");
      try { text.className = deleted ? "memText memGone" : "memText"; } catch (e5) {}
      addText(text, textOf(item.text));
      line.appendChild(text);
      card.appendChild(line);
      var meta = document.createElement("div");
      try { meta.className = "cardBody"; } catch (e6) {}
      var bits = [scopeLabel(item.scope)];
      if (item.scopeKey) { bits.push(textOf(item.scopeKey)); }
      if (item.pinned) { bits.push("đã ghim"); }
      if (item.expiresAt) { bits.push("hết hạn " + textOf(item.expiresAt)); }
      if (item.deletedAt) { bits.push("đã xoá tạm"); }
      addText(meta, bits.join(" · "));
      card.appendChild(meta);
      var actions = document.createElement("div");
      try { actions.className = "btnRow"; } catch (e7) {}
      if (deleted) {
        viewButton(actions, "Khôi phục", "memRestore", textOf(item.id), false);
      } else {
        viewButton(actions, "Xoá tạm", "memDelete", textOf(item.id), false);
      }
      card.appendChild(actions);
      box.appendChild(card);
    }
  }

  function clockOf(value) {
    try {
      var when = new Date(textOf(value));
      if (!isNaN(when.getTime())) { return when.toLocaleTimeString("vi-VN"); }
    } catch (e) {}
    return textOf(value);
  }

  function renderAuditView(box) {
    viewTitle(box, "Nhật ký hoạt động");
    if (view.note) { viewNote(box, view.note); }
    if (view.busy) { viewNote(box, "Đang tải…"); return; }
    var calls = view.data || [];
    if (calls.length === 0) {
      viewNote(box, "Chưa có lượt gọi công cụ nào. Nhật ký ghi lại mọi thao tác AI làm trên tài liệu.");
    } else {
      viewNote(box, calls.length + " lượt gọi công cụ gần nhất (giờ máy)");
    }
    for (var i = 0; i < calls.length; i++) {
      var call = calls[i] || {};
      // Mot dong = mot the, khong phai doan chu mono lien: ten lenh, ma lenh va thoi gian la ba cap
      // thong tin khac nhau, nen tach bang khoang cach va co chu thay vi cung mot co.
      var line = document.createElement("div");
      try { line.className = call.ok ? "auditRow" : "auditRow auditRowErr"; } catch (e) {}
      var mark = document.createElement("span");
      try { mark.className = call.ok ? "toolIcon iconOk" : "toolIcon iconErr"; } catch (eM) {}
      addText(mark, call.ok ? "✓" : "✕");
      line.appendChild(mark);
      // Khoang trang that giua dau va nhan: khop cau cua ban Windows/LibreOffice ("✓ <nhan>") va
      // trang tri dua vao CSS (gap), khong dua vao chuoi trong dau.
      addText(line, " ");
      var label = "";
      try { label = AxiomPaneCore.labelFor(call.action); } catch (e3) { label = textOf(call.action); }
      var body = document.createElement("span");
      try { body.className = "auditBody"; } catch (eN) {}
      addText(body, label);
      var code = document.createElement("span");
      try { code.className = "toolCode"; } catch (eC) {}
      addText(code, " (" + textOf(call.action) + ", " + textOf(call.ms) + "ms)");
      body.appendChild(code);
      line.appendChild(body);
      var time = document.createElement("span");
      try { time.className = "auditTime"; } catch (e2) {}
      addText(time, clockOf(call.createdAt));
      line.appendChild(time);
      box.appendChild(line);
      if (!call.ok && call.error) {
        var why = document.createElement("p");
        try { why.className = "auditErr"; } catch (e4) {}
        addText(why, textOf(call.error));
        box.appendChild(why);
      }
    }
    var extra = document.createElement("div");
    try { extra.className = "btnRow"; } catch (e5) {}
    viewButton(extra, "Tải lại", "auditReload", "", false);
    viewFoot(box, extra);
  }

  // Thiet lap: doc trang thai THAT (GET /wps/v1/setup) roi ghi bang POST /wps/v1/config.
  // O nhap co NHAN THAT (label for=...) nam tren o, khong phai chu nam ngang ben canh: pane hep, de nhan
  // ngang thi o nhap chi con vai chuc pixel. Cong tac (bool) thi nhan nam ben phai hop kiem - bam vao chu
  // cung bat/tat duoc vi ca hai nam trong <label>.
  function settingField(label, key, value, kind, note) {
    var row = document.createElement("div");
    try { row.className = kind === "bool" ? "settingToggle" : "settingBlock"; } catch (e) {}
    var field = document.createElement("input");
    try {
      field.id = "set_" + key;
      field.setAttribute("data-key", key);
      if (kind === "bool") {
        field.className = "check";
        field.type = "checkbox";
        field.checked = !!value;
      } else {
        field.className = "field";
        field.type = "text";
        field.value = textOf(value);
      }
    } catch (e3) {}
    if (kind === "bool") {
      var wrap = document.createElement("label");
      try { wrap.className = "settingLabel checkLabel"; wrap.setAttribute("for", "set_" + key); } catch (eL) {}
      wrap.appendChild(field);
      addText(wrap, " " + label);
      row.appendChild(wrap);
    } else {
      var name = document.createElement("label");
      try { name.className = "settingLabel"; name.setAttribute("for", "set_" + key); } catch (e2) {}
      addText(name, label);
      row.appendChild(name);
      row.appendChild(field);
    }
    // Ghi chu duoi o: noi ro tac dung that (vd o chua co tac dung thi noi thang), khong giau trong tooltip.
    if (note) {
      var hint = document.createElement("div");
      try { hint.className = "settingHint"; } catch (eN) {}
      addText(hint, note);
      row.appendChild(hint);
    }
    return row;
  }

  function renderSetupView(box) {
    viewTitle(box, "Thiết lập");
    if (view.note) { viewNote(box, view.note); }
    if (view.busy) { viewNote(box, "Đang tải…"); return; }
    var data = view.data;
    if (!data) { viewFoot(box, null); return; }
    var current = data.current || {};
    var problems = data.problems || [];
    var item = null;
    if (problems.length > 0) {
      var card = document.createElement("div");
      try { card.className = "card"; } catch (e) {}
      var title = document.createElement("div");
      try { title.className = "cardTitle"; } catch (e2) {}
      addText(title, "Cần kiểm tra");
      card.appendChild(title);
      for (var i = 0; i < problems.length; i++) {
        var body = document.createElement("div");
        try { body.className = "cardBody"; } catch (e3) {}
        addText(body, textOf(problems[i].label) + " — " + textOf(problems[i].hint));
        card.appendChild(body);
      }
      box.appendChild(card);
    }
    if (!view.fields) {
      view.fields = {
        LlmEndpoint: textOf(current.endpoint),
        LlmModel: textOf(current.model),
        LlmApiKey: "",
        TavilyApiKey: "",
        VisualQaEnabled: !!current.visualQaEnabled,
        MemoryEnabled: !!current.memoryEnabled,
        LlmShowReasoning: !!current.llmShowReasoning,
        TavilyEnabled: !!current.tavilyEnabled,
        VerifyWorkEnabled: !!current.verifyWorkEnabled
      };
    }
    viewSection(box, "Máy chủ AI");
    box.appendChild(settingField("Máy chủ AI", "LlmEndpoint", view.fields.LlmEndpoint, "text"));
    box.appendChild(settingField("Model", "LlmModel", view.fields.LlmModel, "text"));
    box.appendChild(settingField("Khoá API", "LlmApiKey", view.fields.LlmApiKey, "text",
      "Để trống nghĩa là giữ khoá đã lưu (" + (current.hasKey ? "đã có khoá" : "chưa có khoá") + ")."));
    box.appendChild(settingField("Khoá Tavily", "TavilyApiKey", view.fields.TavilyApiKey, "text"));

    viewSection(box, "AI làm được gì");
    item = box.appendChild(settingField("Nhìn tài liệu", "VisualQaEnabled", view.fields.VisualQaEnabled, "bool"));
    // Cau that, khong phai loi khuyen: add-in WPS thieu lenh app.screenshot (ban Windows:
    // CommandDispatcher.AppScreenshot; LibreOffice: general.py app_screenshot), nen bat o day thi Core se
    // bat tool look_at_document va tool do se loi. Xem docs/architecture/ribbon-ui.md muc 5.1.
    var off = document.createElement("div");
    try { off.className = "settingHint"; } catch (eOff) {}
    addText(off, "WPS chưa chụp được trang — add-in thiếu lệnh app.screenshot, nên bật \"Nhìn tài liệu\" ở đây chưa có tác dụng.");
    item.appendChild(off);
    box.appendChild(settingField("Ghi nhớ", "MemoryEnabled", view.fields.MemoryEnabled, "bool"));
    box.appendChild(settingField("Hiện suy luận", "LlmShowReasoning", view.fields.LlmShowReasoning, "bool"));
    box.appendChild(settingField("Tìm web (Tavily)", "TavilyEnabled", view.fields.TavilyEnabled, "bool"));
    box.appendChild(settingField("Tự kiểm tra kết quả", "VerifyWorkEnabled", view.fields.VerifyWorkEnabled, "bool"));
    var extra = document.createElement("div");
    try { extra.className = "btnRow"; } catch (e4) {}
    viewButton(extra, "Lưu", "saveSetup", "", true);
    viewButton(extra, "Kiểm tra Core", "testCore", "", false);
    viewFoot(box, extra);
  }

  // ---- Man hinh "Ky nang" (Windows: AdminForms.cs - SkillsForm; LibreOffice: dialogs.py skill_lines) ----

  var SOURCE_LABELS = { builtin: "sẵn có", org: "tổ chức", user: "cá nhân" };

  function sourceLabel(name) {
    var key = trimText(name);
    if (Object.prototype.hasOwnProperty.call(SOURCE_LABELS, key)) { return SOURCE_LABELS[key]; }
    return key === "" ? "?" : key;
  }

  function renderSkillsView(box) {
    viewTitle(box, "Kỹ năng");
    if (view.note) { viewNote(box, view.note); }
    if (view.busy) { viewNote(box, "Đang tải…"); return; }
    var data = view.data;
    if (!data) { viewFoot(box, null); return; }
    var skills = data.skills || [];
    var failures = data.errors || [];
    var sources = data.sources || [];
    var bits = [skills.length + " kỹ năng cho " + appName()];
    if (failures.length > 0) { bits.push(failures.length + " thư mục nạp lỗi"); }
    var used = [];
    for (var s = 0; s < sources.length; s++) {
      if (sources[s] && sources[s].exists) { used.push(sourceLabel(sources[s].name)); }
    }
    if (used.length > 0) { bits.push("nguồn: " + used.join(", ")); }
    viewNote(box, bits.join(" · ") + ". AI tự chọn kỹ năng hợp với yêu cầu.");
    if (skills.length === 0 && failures.length === 0) {
      viewNote(box, "Chưa có kỹ năng nào. Đặt thư mục kỹ năng vào nguồn \"cá nhân\" rồi bấm Nạp lại.");
    }
    // Moi ky nang mot card: ten dam + nhan nguon nho o goc phai, mo ta xuong dong rieng. Nhan
    // "dung cho ..." chi hien khi ky nang bi gioi han app (dung cho moi app la mac dinh, khong can noi).
    if (skills.length > 0) { viewSection(box, "Đang có (" + skills.length + ")"); }
    for (var i = 0; i < skills.length; i++) {
      var skill = skills[i] || {};
      var apps = skill.apps || [];
      var card = makeCard(textOf(skill.name), "skillCard", "");
      var body = document.createElement("div");
      try { body.className = "cardBody"; } catch (e) {}
      // Mot dong "mo ta · dung cho · nguon" - cung thu tu voi ban Windows (AdminForms.cs: SkillsForm) va
      // ban LibreOffice (dialogs.py: skill_lines), spec test_pane_view ghim.
      var parts = [];
      if (trimText(skill.description) !== "") { parts.push(textOf(skill.description)); }
      parts.push(apps.length > 0 ? "dùng cho: " + apps.join(", ") : "dùng cho mọi ứng dụng");
      parts.push(sourceLabel(skill.source));
      addText(body, parts.join(" · "));
      card.appendChild(body);
      box.appendChild(card);
    }
    if (failures.length > 0) {
      viewSection(box, "Không nạp được (" + failures.length + ")");
    }
    for (var k = 0; k < failures.length; k++) {
      var failure = failures[k] || {};
      var bad = makeCard("⚠ Không nạp được", "cardBad", "cardTitleErr");
      bad.setAttribute("role", "alert");
      var why = document.createElement("div");
      try { why.className = "cardBody"; } catch (e2) {}
      // "<thu muc>: <loi>" - cung cau voi ban Windows/LibreOffice, spec ghim.
      addText(why, textOf(failure.directory) + ": " + textOf(failure.error));
      bad.appendChild(why);
      box.appendChild(bad);
    }
    var extra = document.createElement("div");
    try { extra.className = "btnRow"; } catch (e3) {}
    viewButton(extra, "Nạp lại", "skillsReload", "", false);
    viewFoot(box, extra);
  }

  // ---- Man hinh "Trang thai" (Windows: RibbonUi.StatusReport) ----
  // Mot loi goi GET /wps/v1/status tra CA HAI tang: tang add-in (Core biet vi add-in tu poll ve) va tang
  // Core. Pane khong tu hoi bridge duoc - bridge nam o cong rieng, ngoai origin cua pane.

  function numberOf(value) {
    var text = trimText(value);
    if (text === "") { return null; }
    var number = Number(text);
    return isNaN(number) ? null : number;
  }

  // Windows: RibbonUi.Uptime.
  function uptimeText(seconds) {
    var value = numberOf(seconds);
    if (value === null) { return "?"; }
    if (value >= 3600) { return Math.floor(value / 3600) + " giờ " + Math.floor((value % 3600) / 60) + " phút"; }
    if (value >= 60) { return Math.floor(value / 60) + " phút"; }
    return Math.floor(value) + " giây";
  }

  // Dia chi bridge cua add-in. /health la duong DUY NHAT tra loi that: "/" cua bridge la 404, con API cua
  // Core thi doi token - dan vao trinh duyet cung khong xem duoc gi. Cong 0 = bridge chua mo.
  function bridgeUrl(port) {
    var value = numberOf(port);
    if (value === null || value <= 0) { return ""; }
    return "http://127.0.0.1:" + Math.floor(value) + "/health";
  }

  // Mỗi dòng trả về {group, text}: text giữ NGUYÊN câu cũ (bản Windows/LibreOffice dùng chung câu chữ),
  // group chỉ để pane xếp thành nhóm có tiêu đề - người dùng cần biết dòng nào thuộc add-in, dòng nào
  // thuộc Agent Core, thay vì một khối chữ phẳng.
  function statusLine(group, text) {
    return { group: group, text: text };
  }

  function statusLines(data) {
    var addin = data.addin || {};
    var core = data.core || {};
    var doc = addin.document || {};
    var lines = [];
    if (addin.connected) {
      lines.push(statusLine("addin", "✓ Add-in: đang trả lời ở cổng " + textOf(addin.bridgePort) +
        " (phiên bản " + textOf(addin.addinVersion) + ")."));
    } else if (addin.everPoll) {
      lines.push(statusLine("addin", "✗ Add-in: không trả lời — lần trả lời cuối cách đây " + uptimeText(addin.lastPollSeconds) + "."));
      lines.push(statusLine("addin", "   AI sẽ không đọc/sửa được tài liệu này. Mở lại tài liệu trong WPS, hoặc bấm Ask AI."));
    } else {
      lines.push(statusLine("addin", "✗ Add-in: chưa kết nối lần nào kể từ khi Agent Core khởi động."));
      lines.push(statusLine("addin", "   Mở một tài liệu trong WPS để add-in kết nối."));
    }
    if (trimText(doc.name) !== "") { lines.push(statusLine("addin", "   Tài liệu: " + textOf(doc.name))); }
    var pending = (numberOf(addin.queued) || 0) + (numberOf(addin.waiting) || 0);
    if (addin.running) { lines.push(statusLine("addin", "• Add-in đang chạy một lệnh")); }
    else if (pending > 0) { lines.push(statusLine("addin", "• Đang chờ add-in: " + pending + " lệnh")); }

    if (trimText(core.version) === "") {
      // Tra loi duoc nhung thieu truong: ban Core cu hon pane. Noi thang con hon bia ra mot bao cao
      // trong nhu that.
      lines.push(statusLine("core", "✗ Agent Core: trả lời thiếu thông tin — bản Core này cũ hơn pane."));
      return lines;
    }
    lines.push(statusLine("core", "✓ Agent Core: đang chạy (phiên bản " + textOf(core.version) + ", pid " + textOf(core.pid) +
      ", đã chạy " + uptimeText(core.uptimeSeconds) + ")."));
    lines.push(statusLine("core", "   Model: " + (trimText(core.model) !== "" ? textOf(core.model) : "(chưa chọn - bấm Thiết lập…)")));
    if (trimText(core.memoryError) !== "") {
      lines.push(statusLine("core", "   ✗ Bộ nhớ AI lỗi: " + textOf(core.memoryError)));
    } else if (core.memory === "off") {
      lines.push(statusLine("core", "   Bộ nhớ AI: đang tắt"));
    } else if (core.memory === "unavailable") {
      lines.push(statusLine("core", "   ✗ Bộ nhớ AI: không mở được (kiểm tra thư mục dữ liệu)"));
    } else {
      lines.push(statusLine("core", "   Bộ nhớ AI: đang bật"));
    }
    var skillErrors = numberOf(core.skillErrors) || 0;
    lines.push(statusLine("core", "   Kỹ năng: " + textOf(core.skills) +
      (skillErrors > 0 ? " (" + skillErrors + " thư mục nạp lỗi)" : "")));

    var url = bridgeUrl(addin.bridgePort);
    if (url !== "") { lines.push(statusLine("paths", "API URL: " + url)); }
    lines.push(statusLine("paths", "Nhật ký: " + textOf(core.logFile)));
    return lines;
  }

  var STATUS_GROUPS = { addin: "Add-in của ", core: "Agent Core", paths: "Đường dẫn" };

  function renderStatusView(box) {
    viewTitle(box, "Trạng thái");
    if (view.note) { viewNote(box, view.note); }
    if (view.busy) { viewNote(box, "Đang kiểm tra…"); return; }
    var data = view.data;
    if (!data) { viewFoot(box, null); return; }
    var lines = statusLines(data);
    var shown = "";
    for (var i = 0; i < lines.length; i++) {
      var item = lines[i];
      if (item.group !== shown) {
        shown = item.group;
        var group = Object.prototype.hasOwnProperty.call(STATUS_GROUPS, shown) ? STATUS_GROUPS[shown] : shown;
        viewSection(box, shown === "addin" ? group + appName() : group);
      }
      var line = document.createElement("p");
      // Dong chinh (bat dau bang ✓/✗/•) dam hon dong giai thich thut le - mat nguoi doc quet theo cot.
      var isHead = /^[✓✗•]/.test(item.text);
      try { line.className = isHead ? "statusLine statusLineHead" : "statusLine"; } catch (e) {}
      if (/^✗/.test(item.text)) { try { line.className = "statusLine statusLineHead statusLineErr"; } catch (e2) {} }
      addText(line, item.text);
      box.appendChild(line);
    }
    var extra = document.createElement("div");
    try { extra.className = "btnRow"; } catch (e3) {}
    viewButton(extra, "Kiểm tra mô hình", "statusTest", "", false);
    if (bridgeUrl((data.addin || {}).bridgePort) !== "") {
      viewButton(extra, "Sao chép API URL", "statusCopyUrl", "", false);
    }
    viewButton(extra, "Tải lại", "statusReload", "", false);
    viewFoot(box, extra);
  }

  // "Kiem tra mo hinh" (Windows: RibbonUi.TestModel) va "Kiem tra Core" (man hinh Thiet lap) la cung mot
  // lenh; chay sau khi man hinh Trang thai da co du lieu.
  function runStatusTodo() {
    var todo = view.todo;
    if (todo === "") { return; }
    view.todo = "";
    if (todo === "testModel") { runLlmTest(); }
    else if (todo === "copyUrl") { copyApiUrl(); }
  }

  // Sao chep dia chi bridge. Ban Windows dung Clipboard.SetText; trong CEF khong co clipboard dong bo,
  // nen dung navigator.clipboard (Chrome 87 co, va http://127.0.0.1 la secure context) va LUON hien dia
  // chi ra man hinh lam duong lui - tu chep hong thi nguoi dung van chon duoc bang tay.
  function copyApiUrl() {
    var data = view.data || {};
    var url = bridgeUrl((data.addin || {}).bridgePort);
    if (url === "") {
      view.note = "Chưa biết cổng bridge của add-in — bấm Tải lại.";
      renderView();
      return;
    }
    var done = null;
    try {
      if (window.navigator && window.navigator.clipboard && window.navigator.clipboard.writeText) {
        done = window.navigator.clipboard.writeText(url);
      }
    } catch (e) { done = null; }
    view.note = done ? "Đã chép vào clipboard: " + url : "Không tự chép được — chọn và sao chép: " + url;
    renderView();
    if (done && typeof done.then === "function") {
      done.then(null, function () {
        view.note = "Không tự chép được — chọn và sao chép: " + url;
        renderView();
      });
    }
  }

  // ---- Man hinh "Soat tai lieu" (Windows: RibbonUi.CheckDocument; LibreOffice: dialogs.py) ----
  // Ba nut "Soat ..." tren ribbon dung chung man hinh nay; lenh that nam trong ?check= (main.js:
  // CHECK_COMMANDS). Ket qua do add-in tra ve theo dung hop dong cua checks.py, nen phan chuoi tieng
  // Viet duoi day phai khop checks.py - tests/wps/test_pane_view.mjs giu hai ben khop nhau.

  var CHECK_TITLES = {
    "wpp.checkLayout": "Soát bố cục",
    "writer.checkTables": "Soát bảng",
    "et.checkRange": "Soát vùng dữ liệu"
  };
  var CHECK_NOUNS = {
    "wpp.checkLayout": "bố cục trang",
    "writer.checkTables": "bảng trong văn bản",
    "et.checkRange": "vùng dữ liệu"
  };

  // Ban sao cua checks.py:ISSUE_LABELS.
  var ISSUE_LABELS = {
    overflow: "Chữ tràn khung",
    offslide: "Ra ngoài trang",
    "small-font": "Cỡ chữ nhỏ",
    overlap: "Chồng lên nhau",
    dense: "Quá nhiều chữ trên một trang",
    "empty-cells": "Có ô trống",
    "long-header-cell": "Ô tiêu đề quá dài",
    "outside-table": "Dữ liệu nằm ngoài bảng",
    "empty-reference": "Tham chiếu tới ô trống",
    "empty-header": "Cột thiếu tiêu đề",
    "numbers-as-text": "Số đang lưu dạng chữ",
    "mixed-types": "Cột trộn số và chữ",
    "error-values": "Có ô lỗi",
    "no-number-format": "Số chưa có định dạng"
  };

  // Loai la thi tra ve chinh ten do (checks.py: issue_label) - khong nuot van de.
  function issueLabel(kind) {
    var key = textOf(kind);
    if (Object.prototype.hasOwnProperty.call(ISSUE_LABELS, key)) { return ISSUE_LABELS[key]; }
    return key;
  }

  // checks.py: _count - int(value) cat ve phia 0, gia tri khong doc duoc thi dung fallback.
  function countText(value, fallback) {
    var text = trimText(value);
    if (text === "") { return textOf(fallback); }
    var number = Number(text);
    if (isNaN(number)) { return textOf(fallback); }
    return String(number < 0 ? Math.ceil(number) : Math.floor(number));
  }

  // checks.py: _number - "%g" cua Python (6 chu so co nghia, bo so 0 cuoi).
  function numberText(value) {
    var text = trimText(value);
    if (text === "") { return "?"; }
    var number = Number(text);
    if (isNaN(number)) { return "?"; }
    if (number !== 0 && Math.abs(number) >= 1e-4 && Math.abs(number) < 1e6) {
      return String(Number(number.toPrecision(6)));
    }
    return String(number);
  }

  // checks.py: _report_context - nhan dien pham vi theo khoa dac trung cua tung lenh.
  function checkContext(result) {
    if (result.slideWidth !== undefined) {
      return (result.slides || []).length + " trang · khổ " + numberText(result.slideWidth) +
        "×" + numberText(result.slideHeight) + " pt";
    }
    if (result.sheet !== undefined) {
      return "Trang tính " + (textOf(result.sheet) || "?") + " · vùng " +
        (textOf(result.range) || textOf(result.usedRange) || "?") + " · " +
        countText(result.rows, "?") + " dòng × " + countText(result.cols, "?") + " cột";
    }
    if (result.tableCount !== undefined) {
      return countText(result.tableCount, 0) + " bảng trong tài liệu";
    }
    return "";
  }

  // checks.py: report_lines.
  function checkReportLines(result) {
    var data = result || {};
    var issues = data.issues || [];
    var lines = [];
    var context = checkContext(data);
    if (context !== "") { lines.push(context); }
    if (issues.length === 0) {
      lines.push("Không phát hiện vấn đề.");
      return lines;
    }
    lines.push(countText(data.issueCount, issues.length) + " vấn đề:");
    for (var i = 0; i < issues.length; i++) {
      var item = issues[i] || {};
      var label = issueLabel(item.type);
      var detail = trimText(item.detail);
      lines.push(detail === "" ? "• " + label : "• " + label + " — " + detail);
    }
    return lines;
  }

  // Ma loi cua POST /wps/v1/exec (wpsadmin.go: ErrorCode). Do chuoi tieng Anh cua Core thi mong manh,
  // ma hien nguyen chuoi tieng Anh trong pane tieng Viet thi vo duyen.
  var EXEC_ERRORS = {
    "run-in-progress": "Đang có một lượt AI chạy — đợi lượt đó xong rồi soát lại.",
    "addin-disconnected": "Add-in chưa kết nối: mở một tài liệu trong WPS rồi thử lại."
  };

  function execError(reply) {
    var code = "";
    try { code = textOf(reply.data.code); } catch (e) { code = ""; }
    if (Object.prototype.hasOwnProperty.call(EXEC_ERRORS, code)) { return EXEC_ERRORS[code]; }
    return apiError(reply);
  }

  function checkButtons() {
    var extra = document.createElement("div");
    try { extra.className = "btnRow"; } catch (e) {}
    if (view.check !== "") { viewButton(extra, "Soát lại", "checkRun", "", false); }
    return extra;
  }

  function renderCheckView(box) {
    var command = view.check;
    viewTitle(box, Object.prototype.hasOwnProperty.call(CHECK_TITLES, command) ? CHECK_TITLES[command] : "Soát tài liệu");
    if (view.note) { viewNote(box, view.note); }
    if (view.busy) { viewNote(box, "Đang soát…"); return; }
    var data = view.data;
    if (!data) { viewFoot(box, checkButtons()); return; }
    var lines = checkReportLines(data);
    // Dong dau la boi canh (so trang/vung/bang), dong thu hai la tong ket: tach hai dong nay ra khoi
    // danh sach van de, con moi van de la mot dong co dau "•" thut le - de quet hon mot khoi chu.
    var hasIssues = (data.issues || []).length > 0;
    var summaryAt = hasIssues ? 1 : -1;
    for (var i = 0; i < lines.length; i++) {
      var line = document.createElement("p");
      if (i === 0) {
        try { line.className = "checkContext"; } catch (e0) {}
      } else if (i === summaryAt) {
        try { line.className = "checkSummary"; } catch (e1) {}
      } else if (hasIssues) {
        try { line.className = "checkIssue"; } catch (e2) {}
      } else {
        try { line.className = "checkClean"; } catch (e3) {}
      }
      addText(line, lines[i]);
      box.appendChild(line);
    }
    viewFoot(box, checkButtons());
  }

  // ---- Man hinh "Nhat ky" (Windows: RibbonUi.OpenLog mo file bang trinh soan thao; trong CEF pane
  // phai doc noi dung - xem core-go/internal/api/log.go) ----

  var DEFAULT_LOG_LINES = 200;

  function renderLogView(box) {
    viewTitle(box, "Nhật ký Core");
    if (view.note) { viewNote(box, view.note); }
    if (view.busy) { viewNote(box, "Đang tải…"); return; }
    var data = view.data;
    if (!data) { viewFoot(box, null); return; }
    if (!data.exists) {
      viewNote(box, "Chưa có nhật ký — Core chưa ghi dòng nào kể từ khi cài.");
    } else {
      viewNote(box, (data.count || 0) + " dòng cuối" + (data.truncated ? " (còn dòng cũ hơn)" : "") +
        " · " + textOf(data.path));
    }
    var lines = data.lines || [];
    // Nhat ky la van ban dai: mot khoi mono co vien rieng, de phan biet voi cac dong ghi chu phia tren va
    // de mat khong phai nhay giua co chu thuong va mono tren tung dong.
    var block = document.createElement("div");
    try { block.className = "logBlock"; } catch (eB) {}
    block.setAttribute("role", "log");
    block.setAttribute("aria-label", "Dòng nhật ký của Agent Core");
    for (var i = 0; i < lines.length; i++) {
      var line = document.createElement("p");
      try { line.className = "auditLine"; } catch (e) {}
      addText(line, textOf(lines[i]));
      block.appendChild(line);
    }
    box.appendChild(block);
    var extra = document.createElement("div");
    try { extra.className = "btnRow"; } catch (e2) {}
    if (data.truncated) { viewButton(extra, "Xem thêm", "logMore", "", false); }
    viewButton(extra, "Tải lại", "logReload", "", false);
    viewFoot(box, extra);
  }

  // ---- Man hinh "Hoi thoai truoc" (muc dau tien trong menu "..." cua header) ----
  // Windows: nut "Hoi thoai truoc" trong nhom Quan ly (RibbonUi). WPS: chua co - day la ban bo sung.

  function loadConversation(id) {
    api("conversations/" + encodeURIComponent(id)).then(function (reply) {
      if (view.name !== "conversations" || view.detail !== id) { return; }
      var result = resultOf(reply);
      view.busy = false;
      if (!result) { view.data = null; view.note = apiError(reply); } else { view.data = result; }
      renderView();
    }, function () {
      if (view.detail !== id) { return; }
      view.busy = false;
      view.note = "Không kết nối được Agent Core";
      renderView();
    });
  }

  function deleteConversation(id) {
    if (!id) { return; }
    var ok = true;
    try {
      if (typeof window.confirm === "function") {
        ok = window.confirm("Xoá hội thoại này? Không khôi phục được.");
      }
    } catch (e) { ok = true; }
    if (!ok) { return; }
    api("conversations/" + encodeURIComponent(id), "DELETE").then(function (reply) {
      if (view.name !== "conversations") { return; }
      var result = resultOf(reply);
      view.busy = false;
      if (!result) { view.note = apiError(reply); renderView(); return; }
      view.detail = "";
      view.data = null;
      view.note = "Đã xoá hội thoại.";
      renderView();
      loadView();
    }, function () { view.busy = false; view.note = "Không kết nối được Agent Core"; renderView(); });
  }

  // Tieu de hoi thoai: Core luu title (do model dat) hoac summary; khong co thi lay cau hoi dau tien.
  function convoTitle(row) {
    var title = trimText(row && row.title);
    if (title) { return title; }
    var summary = trimText(row && row.summary);
    if (summary) { return summary; }
    return "(hội thoại chưa có tiêu đề)";
  }

  // createdAt/updatedAt la chuoi ISO cua Core: "2026-10-19T08:31:07Z" -> "2026-10-19 08:31".
  function convoWhen(value) {
    var text = trimText(value);
    if (!text) { return ""; }
    return text.replace("T", " ").substring(0, 16);
  }

  function convoMeta(row) {
    var line = convoWhen(row && (row.updatedAt || row.createdAt));
    var name = trimText(row && row.documentName);
    if (name) { line = line ? line + " · " + name : name; }
    var app = trimText(row && row.app);
    if (app) {
      try {
        if (AxiomPaneCore.APP_NAMES && AxiomPaneCore.APP_NAMES[app]) { app = AxiomPaneCore.APP_NAMES[app]; }
      } catch (e) {}
      line = line ? line + " · " + app : app;
    }
    return line;
  }

  function renderConvoList(box) {
    viewTitle(box, "Hội thoại trước");
    if (view.note) { viewNote(box, view.note); }
    if (view.busy) { viewNote(box, "Đang tải…"); return; }
    var rows = view.data || [];
    if (!rows.length) {
      viewNote(box, "Chưa có hội thoại nào được lưu. Core lưu hội thoại sau lượt trả lời đầu tiên.");
      viewFoot(box, null);
      return;
    }
    viewNote(box, rows.length + " hội thoại gần đây · bấm một dòng để xem lại nội dung");
    for (var i = 0; i < rows.length; i++) {
      var row = rows[i] || {};
      // Ca dong la mot nut: tieu de dam o dong dau, ngay + tai lieu + app o dong duoi. Mot dong chip
      // nhu cu thi tieu de dai bi cat mat duoi.
      var item = document.createElement("button");
      try {
        item.type = "button";
        item.className = "convoRow";
        item.setAttribute("data-act", "convoOpen");
        item.setAttribute("data-value", textOf(row.id));
      } catch (e) {}
      var head = document.createElement("span");
      try { head.className = "convoTitle"; } catch (eT) {}
      addText(head, convoTitle(row));
      item.appendChild(head);
      var when = convoWhen(row.updatedAt || row.createdAt);
      var name = trimText(row.documentName);
      var metaBits = [];
      if (when) { metaBits.push(when); }
      if (name) { metaBits.push(name); }
      if (metaBits.length > 0) {
        var meta = document.createElement("span");
        try { meta.className = "convoMeta"; } catch (eM) {}
        addText(meta, metaBits.join(" · "));
        item.appendChild(meta);
      }
      box.appendChild(item);
    }
    var extra = document.createElement("div");
    try { extra.className = "btnRow"; } catch (e2) {}
    viewButton(extra, "Tải lại", "convoReload", "", false);
    viewFoot(box, extra);
  }

  function renderConvoDetail(box) {
    var row = view.data || {};
    viewTitle(box, convoTitle(row));
    var meta = convoMeta(row);
    if (meta) { viewNote(box, meta); }
    if (view.note) { viewNote(box, view.note); }
    if (view.busy) { viewNote(box, "Đang tải…"); return; }
    var messages = row.messages || [];
    if (!messages.length) { viewNote(box, "Hội thoại này chưa có nội dung."); }
    for (var i = 0; i < messages.length; i++) {
      var message = messages[i] || {};
      var bubble = document.createElement("div");
      try { bubble.className = textOf(message.role) === "user" ? "bubble bubbleUser" : "bubble bubbleAi"; } catch (e) {}
      addText(bubble, textOf(message.content));
      box.appendChild(bubble);
    }
    var extra = document.createElement("div");
    try { extra.className = "btnRow"; } catch (e2) {}
    viewButton(extra, "Quay lại", "convoBack", "", false);
    if (trimText(row.id)) {
      var del = viewButton(extra, "Xoá hội thoại này", "convoDelete", textOf(row.id), false);
      try { del.className = "btnDanger"; } catch (e3) {}
    }
    viewFoot(box, extra);
  }

  function renderConversationsView(box) {
    if (view.detail) { renderConvoDetail(box); }
    else { renderConvoList(box); }
  }

  // Mỗi màn hình phụ có ba phần cố định: thanh tiêu đề dính trên (tiêu đề + Đóng), thân cuộn, chân dính.
  // "Đóng" nằm trong thanh tiêu đề nên luôn bấm được, kể cả khi danh sách dài; Esc cũng đóng (xem boot).
  function renderView() {
    var host = $("view");
    if (!host) { return; }
    try {
      clearNode(host);
      if (!view.name) { host.setAttribute("hidden", "hidden"); return; }
      host.removeAttribute("hidden");

      var bar = document.createElement("div");
      try { bar.className = "viewBar"; } catch (eBar) {}
      var heading = document.createElement("div");
      try { heading.id = "viewHeading"; heading.className = "viewHeading"; } catch (eHead) {}
      heading.setAttribute("role", "heading");
      heading.setAttribute("aria-level", "2");
      bar.appendChild(heading);
      var close = document.createElement("button");
      try {
        close.type = "button";
        close.className = "viewClose";
        close.setAttribute("data-act", "closeView");
        close.setAttribute("title", "Đóng (Esc)");
        close.setAttribute("aria-label", "Đóng");
      } catch (eClose) {}
      addText(close, "×");
      bar.appendChild(close);
      host.appendChild(bar);

      var body = document.createElement("div");
      try { body.className = "viewBody"; } catch (eBody) {}
      host.appendChild(body);

      if (view.name === "memory") { renderMemoryView(body); }
      else if (view.name === "audit") { renderAuditView(body); }
      else if (view.name === "setup") { renderSetupView(body); }
      else if (view.name === "skills") { renderSkillsView(body); }
      else if (view.name === "status") { renderStatusView(body); }
      else if (view.name === "check") { renderCheckView(body); }
      else if (view.name === "log") { renderLogView(body); }
      else if (view.name === "conversations") { renderConversationsView(body); }
    } catch (e) {}
  }

  // Doc cac o nhap cua man hinh Thiet lap truoc khi ghi (o trong = khong gui len).
  function readSetupFields() {
    var out = {};
    var keys = ["LlmEndpoint", "LlmModel", "LlmApiKey", "TavilyApiKey", "VisualQaEnabled", "MemoryEnabled",
      "LlmShowReasoning", "TavilyEnabled", "VerifyWorkEnabled"];
    for (var i = 0; i < keys.length; i++) {
      var el = $("set_" + keys[i]);
      if (!el) { continue; }
      if (el.type === "checkbox") { out[keys[i]] = !!el.checked; }
      else if (trimText(el.value) !== "") { out[keys[i]] = el.value; }
    }
    return out;
  }

  function saveSetup() {
    var body = readSetupFields();
    view.busy = true;
    view.note = "Đang lưu…";
    renderView();
    api("config", "POST", body).then(function (reply) {
      var result = resultOf(reply);
      view.busy = false;
      if (!result) {
        view.note = apiError(reply);
      } else {
        var saved = result.saved || [];
        var ignored = result.ignored || [];
        view.note = "Đã lưu " + saved.length + " mục" + (ignored.length ? " · bỏ qua " + ignored.join(", ") : "");
        view.data = null;
        view.fields = null;
      }
      renderView();
      if (result) { loadView(); }
    }, function () { view.busy = false; view.note = "Không kết nối được Agent Core"; renderView(); });
  }

  // Body cho POST /wps/v1/llm/test. Core doc khoa CHU THUONG (providerId/provider/endpoint/model/apiKey)
  // - giong ban Windows (CoreSetup.TestLlm) va ban LibreOffice (setup.py: test_payload). Gui khoa cau
  // hinh (LlmEndpoint, LlmModel...) thi Core doc ra endpoint rong va tra "Endpoint is not configured":
  // nut kiem tra bao loi du cau hinh dung.
  function llmTestBody() {
    var typed = view.name === "setup" ? readSetupFields() : {};
    var current = {};
    if (view.name === "setup") { current = (view.data && view.data.current) || {}; }
    else if (view.name === "status") { current = (view.data && view.data.core) || {}; }
    var endpoint = trimText(typed.LlmEndpoint);
    var model = trimText(typed.LlmModel);
    return {
      providerId: "",
      provider: textOf(current.provider),
      endpoint: endpoint !== "" ? endpoint : textOf(current.endpoint),
      model: model !== "" ? model : textOf(current.model),
      apiKey: trimText(typed.LlmApiKey)
    };
  }

  // Nut "Kiem tra Core" (man hinh Thiet lap) va "Kiem tra mo hinh" (menu Khac cua ribbon): cung mot lenh,
  // cung cau tra loi. Khong dat view.busy: man hinh dang xem phai o lai, cau tra loi hien o dong ghi chu.
  function runLlmTest() {
    var body = llmTestBody();
    if (body.endpoint === "" || body.model === "") {
      view.note = "Chưa cấu hình AI. Bấm Thiết lập… để chọn máy chủ AI và model.";
      renderView();
      return;
    }
    view.note = "Đang kiểm tra…";
    renderView();
    api("llm/test", "POST", body).then(function (reply) {
      var result = resultOf(reply);
      if (result && result.kind) {
        view.note = "Không kết nối được: " + textOf(result.message) + (result.hint ? " — " + result.hint : "");
      } else if (result) {
        view.note = "Kết nối được với " + textOf(result.model) + " (" + textOf(result.seconds) +
          " giây). Model trả lời: " + textOf(result.reply);
      } else {
        view.note = apiError(reply);
      }
      renderView();
    }, function () { view.note = "Không kết nối được Agent Core"; renderView(); });
  }

  // Tai lai man hinh dang mo (moi nut "Tai lai" deu di duong nay).
  function reloadView() {
    view.busy = true;
    view.data = null;
    view.note = "";
    renderView();
    loadView();
  }

  function reloadSkills() {
    api("skills/reload", "POST", {}).then(function (reply) {
      if (view.name !== "skills") { return; }
      if (!resultOf(reply)) {
        view.busy = false;
        view.note = apiError(reply);
        renderView();
        return;
      }
      reloadView();
    }, function () { view.busy = false; view.note = "Không kết nối được Agent Core"; renderView(); });
  }

  // "Soat ..." tren ribbon: mot lenh bridge that, chay ngay trong add-in (khong qua agent, khong ton
  // token) - Windows: RibbonUi.CheckDocument.
  function runCheck() {
    if (view.check === "") {
      view.busy = false;
      view.note = "Thiếu tên lệnh soát — mở màn hình này từ nút trên ribbon.";
      renderView();
      return;
    }
    view.busy = true;
    view.data = null;
    view.note = "";
    renderView();
    api("exec", "POST", { action: view.check, params: {} }).then(function (reply) {
      if (view.name !== "check") { return; }
      var result = resultOf(reply);
      view.busy = false;
      if (!result) {
        view.note = execError(reply);
      } else {
        view.data = result;
        view.note = "Đã soát " + (Object.prototype.hasOwnProperty.call(CHECK_NOUNS, view.check) ? CHECK_NOUNS[view.check] : view.check) +
          " · chỉ đọc, tài liệu không bị sửa";
      }
      renderView();
    }, function () { view.busy = false; view.note = "Không kết nối được Agent Core"; renderView(); });
  }

  function deleteMemory(id) {
    api("memory/" + encodeURIComponent(id), "DELETE").then(function (reply) {
      view.note = resultOf(reply) ? "Đã xoá tạm — khôi phục được trong danh sách này" : apiError(reply);
      loadView();
    }, function () { view.note = "Không kết nối được Agent Core"; renderView(); });
  }

  function restoreMemory(id) {
    api("memory/" + encodeURIComponent(id) + "/restore", "POST", {}).then(function (reply) {
      view.note = resultOf(reply) ? "Đã khôi phục" : apiError(reply);
      loadView();
    }, function () { view.note = "Không kết nối được Agent Core"; renderView(); });
  }

  function purgeMemory() {
    var scope = view.scope === "all" ? "all" : view.scope;
    var ask = "Xoá vĩnh viễn mọi ghi nhớ thuộc phạm vi \"" + scopeLabel(scope) + "\"? Không khôi phục được.";
    var yes = false;
    try { yes = window.confirm(ask); } catch (e) { yes = false; }
    if (!yes) { return; }
    api("memory?scope=" + encodeURIComponent(scope) + "&confirm=true", "DELETE").then(function (reply) {
      view.note = resultOf(reply) ? "Đã xoá vĩnh viễn theo phạm vi đang lọc" : apiError(reply);
      loadView();
    }, function () { view.note = "Không kết nối được Agent Core"; renderView(); });
  }

  function render(state) {
    try {
      if (!state) { return; }
      var chat = $("chat");
      if (chat) {
        while (chat.firstChild) { chat.removeChild(chat.firstChild); }
        var msgs = state.messages || [];
        var steps = state.steps || [];
        var running = !!state.running;
        var confirmation = state.confirmation || null;
        var skills = state.skills || [];
        var notes = state.notes || [];
        var progress = trimText(state.progress);
        if (msgs.length === 0 && steps.length === 0 && !running && !confirmation && skills.length === 0 &&
            notes.length === 0 && !state.reasoning && !state.planReady && !progress) {
          chat.appendChild(makeEmpty());
        } else {
          var lastUser = -1;
          var i;
          for (i = 0; i < msgs.length; i++) {
            var r = "assistant";
            try { r = String((msgs[i] && msgs[i].role) || "assistant"); } catch (e) {}
            if (r === "user") { lastUser = i; }
          }
          for (i = 0; i < msgs.length; i++) {
            var msg = msgs[i] || {};
            var role = "assistant";
            try { role = String(msg.role || "assistant"); } catch (e2) {}
            var text = "";
            try { text = String(msg.text === undefined || msg.text === null ? "" : msg.text); } catch (e3) {}
            if (role === "user") {
              var ub = makeBubble("bubbleUser");
              addText(ub, text);
              chat.appendChild(ub);
            } else if (role === "error") {
              chat.appendChild(makeErrorCard(text));
            } else if (role === "info") {
              // Dong he thong (vd "Đã hoàn tác 2 thao tác của lượt vừa rồi.") - khong phai cau tra loi.
              var info = document.createElement("p");
              try { info.className = "infoLine"; } catch (eInfo) {}
              addText(info, text);
              chat.appendChild(info);
            } else {
              var ab = makeBubble("bubbleAi");
              var plain = text;
              try { plain = AxiomPaneCore.plainReply(text); } catch (e4) {}
              addText(ab, plain);
              chat.appendChild(ab);
            }
            if (i === lastUser && steps.length > 0) {
              var wrap = document.createElement("div");
              try { wrap.className = "toolWrap"; } catch (e5) {}
              for (var k = 0; k < steps.length; k++) {
                wrap.appendChild(makeToolRow(steps[k] || {}, running));
              }
              chat.appendChild(wrap);
            }
          }
          if (lastUser < 0 && steps.length > 0) {
            var top = document.createElement("div");
            try { top.className = "toolWrap"; } catch (e6) {}
            for (var k2 = 0; k2 < steps.length; k2++) {
              top.appendChild(makeToolRow(steps[k2] || {}, running));
            }
            chat.insertBefore(top, chat.firstChild);
          }
          // Trong khi chay: dong "dang lam gi", khoi suy luan, ky nang da dung, ghi nho vua ghi, the ke hoach.
          if (progress) {
            var pl = document.createElement("div");
            try { pl.className = "progressLine"; } catch (ePl) {}
            addText(pl, progress);
            chat.appendChild(pl);
          }
          if (state.reasoning) { chat.appendChild(makeReasonCard(state.reasoning)); }
          if (skills.length > 0) {
            var skillWrap = document.createElement("div");
            try { skillWrap.className = "toolWrap"; } catch (eSk) {}
            for (var si = 0; si < skills.length; si++) { skillWrap.appendChild(makeSkillRow(skills[si] || {})); }
            chat.appendChild(skillWrap);
          }
          for (var ni = 0; ni < notes.length; ni++) { chat.appendChild(makeNoteCard(notes[ni] || {})); }
          if (state.planReady) { chat.appendChild(makePlanCard(state)); }
          if (running) {
            var tb = makeBubble("bubbleAi");
            var dots = document.createElement("span");
            try { dots.className = "typingDots"; } catch (e7) {}
            var d1 = document.createElement("span");
            try { d1.className = "tdot"; } catch (e8) {}
            dots.appendChild(d1);
            var d2 = document.createElement("span");
            try { d2.className = "tdot tdot2"; } catch (e9) {}
            dots.appendChild(d2);
            var d3 = document.createElement("span");
            try { d3.className = "tdot tdot3"; } catch (e10) {}
            dots.appendChild(d3);
            tb.appendChild(dots);
            chat.appendChild(tb);
          }
          if (confirmation) {
            chat.appendChild(makeConfirmCard(confirmation));
          }
        }
        try { chat.scrollTop = chat.scrollHeight; } catch (e11) {}
      }
      var title = $("paneTitle");
      if (title) {
        clearNode(title);
        title.appendChild(document.createTextNode(appName()));
      }
      var sub = $("paneSubtitle");
      if (sub) {
        clearNode(sub);
        // Phu de: ten app + model dang chay (Windows: pane hien model o dong phu).
        var model = trimText(state.model);
        sub.appendChild(document.createTextNode(model ? appName() + " · " + model : "Axiom Office"));
      }
      var planBtn = $("planToggle");
      if (planBtn) {
        try {
          planBtn.setAttribute("aria-pressed", state.plan ? "true" : "false");
          clearNode(planBtn);
          planBtn.appendChild(document.createTextNode(state.plan ? "● Lập kế hoạch trước" : "○ Lập kế hoạch trước"));
        } catch (ePlan) {}
      }
      var input = $("input");
      if (input) {
        try {
          input.setAttribute("placeholder", state.plan
            ? "Mô tả việc cần làm - AI lập kế hoạch trước, chưa sửa gì…"
            : "Nhập yêu cầu cho tài liệu này…");
        } catch (ePh) {}
      }
      var status = $("statusText");
      if (status) {
        clearNode(status);
        var stext = "Sẵn sàng";
        if (state.confirmation) {
          stext = "Đang chờ bạn xác nhận…";
        } else {
          try { stext = AxiomPaneCore.summary(state); }
          catch (eSt) { stext = state.running ? "Đang chạy…" : "Sẵn sàng"; }
        }
        status.appendChild(document.createTextNode(stext));
      }
      var busy = $("busyDot");
      if (busy) {
        try {
          if (state.running) { busy.removeAttribute("hidden"); }
          else { busy.setAttribute("hidden", "hidden"); }
        } catch (e12) {}
      }
      var stopBtn = $("stopBtn");
      if (stopBtn) {
        try {
          if (state.running) { stopBtn.removeAttribute("hidden"); }
          else { stopBtn.setAttribute("hidden", "hidden"); }
        } catch (e13) {}
      }
      var undoBtn = $("undoBtn");
      if (undoBtn) {
        try {
          if (state.canUndo) { undoBtn.removeAttribute("hidden"); }
          else { undoBtn.setAttribute("hidden", "hidden"); }
        } catch (e14) {}
      }
      var insertBtn = $("insertBtn");
      if (insertBtn) {
        try {
          if (state.canInsert) { insertBtn.removeAttribute("hidden"); }
          else { insertBtn.setAttribute("hidden", "hidden"); }
        } catch (e15) {}
      }
      updateHint();
      updateSendDisabled(state);
    } catch (e16) {}
  }

  function doSend() {
    try {
      if (!ctl) { return; }
      var input = $("input");
      if (!input) { return; }
      var value = "";
      try { value = input.value; } catch (e) { value = ""; }
      if (!value || !value.replace(/^\s+|\s+$/g, "")) { return; }
      var r = null;
      try { r = ctl.send(value); } catch (e2) { r = false; }
      if (r !== false) {
        try { input.value = ""; } catch (e3) {}
      }
      if (r && typeof r.then === "function") {
        try { r.then(null, function () {}); } catch (e4) {}
      }
      try { render(ctl.getState ? ctl.getState() : null); } catch (e5) {}
      updateSendDisabled({ running: true });
    } catch (e6) {}
  }

  // Cau hoi nguoi dung gui gan nhat - de nut "Thử lại" chay lai dung luot vua loi.
  function lastPrompt(state) {
    try {
      var msgs = (state && state.messages) || [];
      for (var i = msgs.length - 1; i >= 0; i--) {
        if (msgs[i] && msgs[i].role === "user") { return trimText(msgs[i].text); }
      }
    } catch (e) {}
    return "";
  }

  function doRetry() {
    try {
      var prompt = lastPrompt(ctl ? ctl.getState() : null);
      if (!prompt) { return; }
      if (ctl) { ctl.send(prompt); }
    } catch (e) {}
  }

  function currentState() {
    try { return ctl ? ctl.getState() : null; } catch (e) { return null; }
  }

  // Bang lenh cua pane: ten data-act -> ham. onAction chi tra cuu, khong con day chuoi if/else dai.
  // Nhanh nao can ctl thi boc withCtl de loi cua host khong lam chet cac nut khac.
  function withCtl(fn) {
    try { if (ctl) { fn(ctl); } } catch (e) {}
  }

  // Tai lai danh sach cua man hinh dang mo ma KHONG xoa dong ghi chu (khac reloadView).
  function reloadList() {
    view.busy = true;
    view.data = null;
    renderView();
    loadView();
  }

  var ACTIONS = {
    more: function () { setMenu(!menuOpen()); },
    "new": function () { withCtl(function (c) { c.newConversation(); }); },
    send: doSend,
    stop: function () { withCtl(function (c) { c.stop(); }); },
    approve: function () { withCtl(function (c) { c.confirm(true); }); },
    deny: function () { withCtl(function (c) { c.confirm(false); }); },
    choose: function (value) { withCtl(function (c) { c.choose(value); }); },
    chooseText: function (value) {
      try {
        var typed = $("choiceText");
        var text = typed ? String(typed.value || "") : "";
        if (text.replace(/\s+/g, "") === "") {
          if (typed) { typed.focus(); }
        } else {
          withCtl(function (c) { c.choose(value, text); });
        }
      } catch (e) {}
    },
    suggestion: function (value) {
      try {
        var input = $("input");
        if (!input) { return; }
        input.value = value;
        try { updateSendDisabled({ running: false }); } catch (e) {}
        try { input.focus(); } catch (e2) {}
        try {
          var len = String(input.value).length;
          if (input.setSelectionRange) { input.setSelectionRange(len, len); }
        } catch (e3) {}
      } catch (e4) {}
    },
    plan: function () {
      withCtl(function (c) { c.setPlan(!(currentState() || {}).plan); });
    },
    reason: function () { withCtl(function (c) { c.toggleReasoning(); }); },
    planExec: function () { withCtl(function (c) { c.executePlan(); }); },
    undo: function () { withCtl(function (c) { c.undoLastTurn(); }); },
    insert: function () { withCtl(function (c) { c.insertReply(); }); },
    forget: function (value) { withCtl(function (c) { c.forget(value); }); },
    retry: doRetry,
    view: function (value) { openView(value); },
    closeView: closeView,
    memScope: function (value) { view.scope = value || "all"; reloadList(); },
    memReload: reloadList,
    memDelete: deleteMemory,
    memRestore: restoreMemory,
    memPurge: purgeMemory,
    auditReload: reloadList,
    saveSetup: saveSetup,
    testCore: runLlmTest,
    statusTest: runLlmTest,
    statusReload: reloadView,
    logReload: reloadView,
    statusCopyUrl: copyApiUrl,
    skillsReload: reloadSkills,
    checkRun: runCheck,
    logMore: function () {
      view.lines = (view.lines || DEFAULT_LOG_LINES) + DEFAULT_LOG_LINES;
      reloadView();
    },
    convoOpen: function (value) { view.detail = value; reloadView(); },
    convoBack: function () { view.detail = ""; reloadView(); },
    convoReload: reloadView,
    convoDelete: deleteConversation
  };

  function onAction(found) {
    if (!found) { return; }
    // Chon bat ky muc nao khac nut "..." thi menu phai dong lai (nut "..." tu no bat/tat).
    if (found.act !== "more") { setMenu(false); }
    var fn = ACTIONS[found.act];
    if (!fn) { return; }
    try { fn(found.value); } catch (e) {}
  }

  function bindClicks(host) {
    if (!host) { return; }
    var onClick = function (ev) {
      try {
        ev = ev || window.event;
        var t = null;
        try { t = ev.target || ev.srcElement; } catch (e) { t = null; }
        var found = actOf(t);
        if (!found) { return; }
        onAction(found);
      } catch (e10) {}
    };
    if (host.addEventListener) { host.addEventListener("click", onClick); }
    else { host.onclick = onClick; }
  }

  try {
    bindClicks($("app"));
    bindClicks($("view"));
    var input2 = $("input");
    if (input2) {
      var onKey = function (ev) {
        try {
          ev = ev || window.event;
          var code = ev.keyCode || ev.which;
          if (code === 13 && !ev.shiftKey) {
            if (ev.preventDefault) { ev.preventDefault(); }
            try { ev.returnValue = false; } catch (e) {}
            doSend();
            return false;
          }
        } catch (e2) {}
        return true;
      };
      var onInput = function () {
        try {
          var s = null;
          try { s = ctl ? ctl.getState() : null; } catch (e) { s = null; }
          updateSendDisabled(s || { running: false });
        } catch (e2) {}
      };
      if (input2.addEventListener) {
        input2.addEventListener("keydown", onKey);
        input2.addEventListener("input", onInput);
      } else {
        input2.onkeydown = onKey;
        input2.oninput = onInput;
      }
    }
    try {
      if (window.addEventListener) {
        window.addEventListener("resize", updateHint);
      }
    } catch (e11) {}
    // Menu "..." dong khi bam ra ngoai no hoac bam Esc (Word cung vay). onAction da dong menu cho moi
    // muc co data-act; o day chi lo truong hop bam vao cho khong co lenh nao. Gan vao document.body chu
    // khong phai window: moi cu bam trong pane deu noi bot toi day, va pane khong bao gio tran ra ngoai.
    try {
      var page = document.body;
      if (page && page.addEventListener) {
        page.addEventListener("click", function (ev) {
          try {
            if (!menuOpen()) { return; }
            var target = null;
            try { target = ev.target || ev.srcElement; } catch (e) { target = null; }
            if (!actOf(target)) { setMenu(false); }
          } catch (e2) {}
        });
        page.addEventListener("keydown", function (ev) {
          try {
            var code = ev.keyCode || ev.which;
            var name = "";
            try { name = textOf(ev.key); } catch (e) { name = ""; }
            if ((code === 27 || name === "Escape") && menuOpen()) { setMenu(false); }
          } catch (e3) {}
        });
      }
    } catch (e13) {}
    render({ messages: [], running: false, steps: [], confirmation: null, conversationId: null });
    // Danh sach muc "Sua bang AI" (chi Writer). Khung chat KHONG ve menu nay - loi vao duy nhat la menu chuot phai
    // (20/10/2026, theo yeu cau) - nhung can nhan cua muc de bong bong hien "Trau chuot" chu khong phai "polish".
    try { if (ctl && !VIEW) { quickReady = ctl.loadQuickActions(); } } catch (eQuickLoad) {}
    listenPane();
    // Ribbon mo pane kem ?view=/?check=/?do= (main.js: paneUrl) - man hinh phai tu mo va tu lam viec
    // duoc yeu cau, khong doi nguoi dung bam lai nut.
    if (VIEW) { openView(VIEW, VIEW_CHECK, VIEW_DO); }
  } catch (e12) {}
})();
