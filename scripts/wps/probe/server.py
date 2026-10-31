"""REPL do object model WPS that: add-in tam hoi /job, chay JS, gui /result (xem README.md canh tep).

    python3 server.py PORT WORKDIR
WORKDIR/jobs/<name>.js -> chay lan luot (theo ten); ket qua ghi WORKDIR/jobs/<name>.out.json.
Than job la than mot ham (app, show) => gia tri tra ve; AxiomCommands (commands.js that) co san.
"""
import http.server
import json
import os
import sys

PORT = int(sys.argv[1])
WORK = sys.argv[2]
JOBS = os.path.join(WORK, "jobs")
HERE = os.path.dirname(os.path.abspath(__file__))
ADDIN = os.path.join(HERE, "addin")
# commands.js lay thang tu add-in that: job chay duoc handler dung nhu ban se cai.
REAL_ADDIN = os.path.join(HERE, "..", "..", "..", "src", "AxiomOffice.WPS")
os.makedirs(JOBS, exist_ok=True)


def next_job():
    for name in sorted(os.listdir(JOBS)):
        if name.endswith(".js"):
            stem = name[:-3]
            if not os.path.exists(os.path.join(JOBS, stem + ".out.json")) and \
                    not os.path.exists(os.path.join(JOBS, stem + ".taken")):
                open(os.path.join(JOBS, stem + ".taken"), "w").close()
                with open(os.path.join(JOBS, name), encoding="utf-8") as f:
                    return {"id": stem, "code": f.read()}
    return {}


class H(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a):
        with open(os.path.join(WORK, "http.log"), "a") as f:
            f.write("%s %s\n" % (self.command, self.path))

    def send(self, code, body, ctype="application/json"):
        data = body.encode("utf-8") if isinstance(body, str) else body
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Headers", "*")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_OPTIONS(self):
        self.send(204, b"")

    def do_GET(self):
        path = self.path.split("?")[0]
        if path == "/job":
            return self.send(200, json.dumps(next_job()))
        if path.startswith("/wps/addin/"):
            rel = path[len("/wps/addin/"):] or "index.html"
            full = os.path.join(REAL_ADDIN if rel == "commands.js" else ADDIN, rel)
            if os.path.isfile(full):
                ctype = {"xml": "text/xml", "html": "text/html", "js": "application/javascript"}.get(
                    rel.rsplit(".", 1)[-1], "application/octet-stream")
                with open(full, "rb") as f:
                    return self.send(200, f.read(), ctype + "; charset=utf-8")
        self.send(404, "{}")

    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length") or 0)).decode("utf-8")
        if self.path.startswith("/result"):
            msg = json.loads(body)
            with open(os.path.join(JOBS, msg["id"] + ".out.json"), "w", encoding="utf-8") as f:
                json.dump(msg, f, ensure_ascii=False, indent=1)
        elif self.path.startswith("/hello"):
            with open(os.path.join(WORK, "hello.json"), "a", encoding="utf-8") as f:
                f.write(body + "\n")
        self.send(200, "{}")


http.server.ThreadingHTTPServer(("127.0.0.1", PORT), H).serve_forever()
