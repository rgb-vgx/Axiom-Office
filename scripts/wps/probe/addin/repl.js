var BASE = "http://127.0.0.1:" + (location.port || "47990");
function OnAddinLoad() { return true; }
function OnAction() { return true; }
function post(path, obj) {
  var x = new XMLHttpRequest(); x.open("POST", BASE + path, true);
  x.setRequestHeader("Content-Type", "text/plain"); x.send(JSON.stringify(obj));
}
function show(v, depth) {
  depth = depth || 0;
  try {
    if (v === null || v === undefined) { return v === null ? null : "undefined"; }
    var t = typeof v;
    if (t === "number" || t === "string" || t === "boolean") { return v; }
    if (t === "function") { return "[function]"; }
    if (Object.prototype.toString.call(v) === "[object Array]") {
      var a = []; for (var i = 0; i < v.length && i < 50; i++) { a.push(depth < 3 ? show(v[i], depth + 1) : "..."); } return a;
    }
    if (depth >= 2) { return "[object]"; }
    var o = {}; for (var k in v) { try { o[k] = show(v[k], depth + 1); } catch (e) { o[k] = "!" + e.message; } } return o;
  } catch (e) { return "!" + e.message; }
}
function tick() {
  var x = new XMLHttpRequest(); x.open("GET", BASE + "/job?t=" + Date.now(), true);
  x.onload = function () {
    var job = {}; try { job = JSON.parse(x.responseText); } catch (e) {}
    if (job.id) {
      var out = { id: job.id, app: String(Application.Name) };
      try { out.result = show(new Function("app", "show", job.code)(Application, show)); }
      catch (e) { out.error = String(e && e.message || e); }
      post("/result", out);
    }
    setTimeout(tick, 400);
  };
  x.onerror = function () { setTimeout(tick, 1000); };
  x.send();
}
post("/hello", { app: String(Application.Name), version: String(Application.Version) });
tick();
function OnGetImage(control) { return "images/" + control.Tag + ".svg"; }
