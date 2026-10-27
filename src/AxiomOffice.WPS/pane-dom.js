// Helper DOM dung chung cho pane.html (khung chat lon) va mini.html (khung chat canh doan boi den).
// Chi chua ham thuan DOM, khong biet gi ve Agent Core - pane-core.js moi la phan du lieu/logic.
// ES5: CEF cua WPS la Chrome 87, nhung ca cay add-in giu cu phap ES5 (xem eslint.config.mjs).
(function (global) {
  "use strict";

  function $(id) {
    try { return document.getElementById(id); } catch (e) { return null; }
  }

  function textOf(value) {
    return String(value === undefined || value === null ? "" : value);
  }

  function trimText(value) {
    return textOf(value).replace(/^\s+|\s+$/g, "");
  }

  function clearNode(el) {
    try { while (el.firstChild) { el.removeChild(el.firstChild); } } catch (e) {}
  }

  function addText(el, text) {
    try {
      el.appendChild(document.createTextNode(String(text === undefined || text === null ? "" : text)));
    } catch (e) {}
  }

  // Trich dan theo CitationRule cua Core (agent/prompt.go): [§12], [Data!C42], ['Bao cao'!A1:B5], [slide 3]. Link Markdown
  // [chu](url) khong tinh (lookahead "(").
  var CITATION = /\[(\u00A7\s?\d{1,6}|[Ss]lide\s+\d{1,4}|(?:'[^'\]]+'|[^\]\s!'\[]+)![A-Za-z]{1,3}\d{1,7}(?::[A-Za-z]{1,3}\d{1,7})?)\](?!\()/g;

  // addCitedText: nhu addText nhung trich dan thanh link bam duoc (data-act="cite" - pane nhay toi cho do).
  function addCitedText(el, text) {
    var value = String(text === undefined || text === null ? "" : text);
    var last = 0;
    var match;
    try {
      CITATION.lastIndex = 0;
      while ((match = CITATION.exec(value)) !== null) {
        if (match.index > last) { addText(el, value.substring(last, match.index)); }
        // span (khong phai <a href="#">): bam khong doi dia chi trang, khung chat khong nhay len dau.
        var link = document.createElement("span");
        try {
          link.className = "cite";
          link.setAttribute("role", "link");
          link.setAttribute("tabindex", "0");
          link.setAttribute("data-act", "cite");
          link.setAttribute("data-value", match[1]);
          link.setAttribute("title", "Nhảy tới chỗ này trong tài liệu");
        } catch (eAttr) {}
        addText(link, match[0]);
        el.appendChild(link);
        last = match.index + match[0].length;
      }
    } catch (e) {}
    if (last < value.length) { addText(el, value.substring(last)); }
  }

  function make(tag, className, text) {
    var el = document.createElement(tag);
    try { if (className) { el.className = className; } } catch (e) {}
    if (text !== undefined && text !== null) { addText(el, text); }
    return el;
  }

  // Trang nam o <goc>/wps/addin/<ten>.html, API o <goc>/wps/v1/. Tinh tu href that de ca pane.html
  // lan mini.html (va ca ban chay trong CEF lan test DOM gia) deu ra cung mot goc.
  function baseUrl() {
    var base = "../v1/";
    try {
      var href = "";
      try { href = (global.location && global.location.href) || ""; } catch (e) { href = ""; }
      if (href) {
        try {
          var clean = String(href).split(String.fromCharCode(35))[0].split("?")[0];
          if (typeof global.URL === "function") {
            base = new global.URL("../v1/", clean || href).href;
          } else {
            var idx = String(clean).lastIndexOf("/");
            var dir = idx >= 0 ? String(clean).substring(0, idx + 1) : "";
            var trimmed = dir.charAt(dir.length - 1) === "/" ? dir.substring(0, dir.length - 1) : dir;
            var j = trimmed.lastIndexOf("/");
            base = (j >= 0 ? trimmed.substring(0, j + 1) : dir) + "v1/";
          }
        } catch (e2) {}
      }
    } catch (e3) {}
    try {
      if (base.charAt(base.length - 1) !== "/") { base = base + "/"; }
    } catch (e4) {}
    return base;
  }

  // Phan tu mang data-act gan nhat (tinh ca chinh no), dung cho ca pane lan mini.
  function actOf(el) {
    try {
      var stop = document.body || null;
      while (el && el !== stop) {
        try {
          if (el.getAttribute) {
            var act = el.getAttribute("data-act");
            if (act) { return { act: act, value: el.getAttribute("data-value") || "" }; }
          }
        } catch (e) {}
        el = el.parentNode;
      }
    } catch (e2) {}
    return null;
  }

  global.AxiomPaneDom = {
    $: $,
    textOf: textOf,
    trimText: trimText,
    clearNode: clearNode,
    addText: addText,
    addCitedText: addCitedText,
    make: make,
    baseUrl: baseUrl,
    actOf: actOf
  };
})(window);
