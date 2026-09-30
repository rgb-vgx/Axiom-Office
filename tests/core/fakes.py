"""May chu gia trong tien trinh test (khong can file rieng, khong can mang).

Dung cho cac phan e2e chua co LLM gia tuong ung:
  - JsonServer: HTTP server nho, moi request POST duoc ghi lai roi giao cho ham `handler` tra ve JSON.
  - anthropic_handler: {endpoint}/messages theo dinh dang Anthropic (tool_use -> tool_result, anh base64).
  - embedding_handler: /embeddings kieu OpenAI-compatible voi vector dieu khien duoc (kiem thu chong
    trung bang cosine va tim theo nghia).
  - McpHttpServer: MCP server kieu Streamable HTTP (tra JSON hoac SSE, co Mcp-Session-Id) de kiem thu
    mcp.json dang `url`.
"""
from __future__ import annotations

import json
import threading
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class JsonServer:
    """HTTP server nho: ghi lai request roi tra JSON do `handler(path, body, requests)` quyet dinh."""

    def __init__(self, handler, status: int = 200):
        outer = self
        self.requests: list[dict] = []
        self.status = status

        class Handler(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *args) -> None:  # tat log ra stderr
                pass

            def _reply(self, payload: dict, status: int) -> None:
                data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def do_POST(self) -> None:
                length = int(self.headers.get("Content-Length", "0"))
                raw = self.rfile.read(length).decode("utf-8") if length else "{}"
                try:
                    body = json.loads(raw)
                except json.JSONDecodeError:
                    body = {"__raw": raw}
                record = {"path": self.path, "headers": {k.lower(): v for k, v in self.headers.items()}, "body": body}
                outer.requests.append(record)
                payload, status = handler(self.path, body, outer.requests)
                self._reply(payload, status or outer.status)

            def do_GET(self) -> None:
                outer.requests.append({"path": self.path, "headers": {k.lower(): v for k, v in self.headers.items()}, "body": None})
                payload, status = handler(self.path, None, outer.requests)
                self._reply(payload, status or outer.status)

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.port = self.server.server_address[1]
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    @property
    def base(self) -> str:
        return "http://127.0.0.1:%d" % self.port

    def stop(self) -> None:
        self.server.shutdown()
        self.server.server_close()


def anthropic_handler(tool_call: dict, reply: str):
    """Tra tool_use o request dau tien, sau do tra cau tra loi cuoi (giong Anthropic that)."""

    def handler(path: str, body: dict | None, requests: list[dict]):
        if not path.endswith("/messages"):
            return {"error": {"message": "not found"}}, 404
        messages = (body or {}).get("messages", [])
        # Da co ket qua tool (block tool_result) -> tra cau tra loi cuoi.
        has_result = any(
            isinstance(message.get("content"), list)
            and any(block.get("type") == "tool_result" for block in message["content"] if isinstance(block, dict))
            for message in messages
        )
        if has_result:
            return {"id": "msg_2", "type": "message", "role": "assistant",
                    "content": [{"type": "text", "text": reply}],
                    "usage": {"input_tokens": 20, "output_tokens": 5}}, 200

        blocks = []
        for index, call in enumerate(tool_call.get("calls", [])):
            blocks.append({"type": "tool_use", "id": "toolu_%d" % (index + 1), "name": call["name"],
                           "input": call.get("arguments", {})})
        blocks.append({"type": "text", "text": "Bắt đầu làm."})
        return {"id": "msg_1", "type": "message", "role": "assistant", "content": blocks,
                "usage": {"input_tokens": 10, "output_tokens": 3}}, 200

    return handler


def embedding_handler(marker_vectors: dict[str, list[float]], default: list[float] | None = None):
    """Tra vector theo dau hieu trong text: text chua tu khoa nao -> vector tuong ung.

    marker_vectors: {"TRUNG": [1,0,0], "GAN": [1,0.05,0]}. Nho vay kiem thu duoc ca chong trung bang
    cosine ~1) lan tim theo nghia (khong chung tu khoa nao).
    """

    def vector_for(text: str) -> list[float]:
        upper = text.upper()
        for marker, vector in marker_vectors.items():
            if marker in upper:
                return vector
        return default if default is not None else [0.0, 1.0, 0.0]

    def handler(path: str, body: dict | None, requests: list[dict]):
        if not path.endswith("/embeddings"):
            return {"error": {"message": "not found"}}, 404
        inputs = (body or {}).get("input", [])
        if isinstance(inputs, str):
            inputs = [inputs]
        data = [{"index": index, "embedding": vector_for(text)} for index, text in enumerate(inputs)]
        return {"object": "list", "data": data, "model": (body or {}).get("model", "fake-embed")}, 200

    return handler


class McpHttpServer:
    """MCP server kieu Streamable HTTP (JSON-RPC qua POST) de kiem thu mcp.json dang `url`.

    mode="json": tra application/json; mode="sse": tra text/event-stream voi mot dong `data:` — hai kieu
    response ma core phai doc duoc. Cap Mcp-Session-Id o initialize va BAT BUOC client gui lai o cac request
    sau (dung nhu MCP that, de bat loi khong giu session).
    """

    def __init__(self, mode: str = "json"):
        outer = self
        self.mode = mode
        self.requests: list[dict] = []
        self.session = "sess-" + uuid.uuid4().hex[:8]
        self.refused: list[str] = []
        self.tools = [
            {"name": "echo", "description": "Tra lai chuoi da gui",
             "inputSchema": {"type": "object", "properties": {"text": {"type": "string"}}, "required": ["text"]}},
        ]

        class Handler(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *args) -> None:
                pass

            def _send(self, payload: dict | None, status: int, session: str = "") -> None:
                if payload is None:
                    data = b""
                    content_type = "application/json"
                elif outer.mode == "sse":
                    data = ("data: " + json.dumps(payload, ensure_ascii=False) + "\n\n").encode("utf-8")
                    content_type = "text/event-stream"
                else:
                    data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
                    content_type = "application/json"
                self.send_response(status)
                self.send_header("Content-Type", content_type)
                if session:
                    self.send_header("Mcp-Session-Id", session)
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                if data:
                    self.wfile.write(data)

            def do_POST(self) -> None:
                length = int(self.headers.get("Content-Length", "0"))
                raw = self.rfile.read(length).decode("utf-8") if length else "{}"
                try:
                    body = json.loads(raw)
                except json.JSONDecodeError:
                    body = {}
                outer.requests.append({"headers": {k.lower(): v for k, v in self.headers.items()}, "body": body})
                payload, status, session = outer.handle(body, self.headers)
                self._send(payload, status, session)

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.port = self.server.server_address[1]
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    @property
    def url(self) -> str:
        return "http://127.0.0.1:%d/mcp" % self.port

    def handle(self, body: dict, headers) -> tuple[dict | None, int, str]:
        """Tra (payload, status, header phien can gui kem)."""
        method = body.get("method")
        message_id = body.get("id")
        session = headers.get("Mcp-Session-Id", "")

        if method == "initialize":
            return ({"jsonrpc": "2.0", "id": message_id,
                     "result": {"protocolVersion": "2025-06-18", "capabilities": {},
                                "serverInfo": {"name": "mau-http", "version": "1.0.0"}}}, 200, self.session)
        if method == "notifications/initialized":
            return None, 202, self.session
        if session != self.session:
            self.refused.append("thieu Mcp-Session-Id o %s" % method)
            return {"jsonrpc": "2.0", "id": message_id,
                    "error": {"code": -32000, "message": "missing or wrong Mcp-Session-Id"}}, 400, ""
        if method == "tools/list":
            return {"jsonrpc": "2.0", "id": message_id, "result": {"tools": self.tools}}, 200, ""
        if method == "tools/call":
            params = body.get("params") or {}
            if params.get("name") != "echo":
                return {"jsonrpc": "2.0", "id": message_id,
                        "error": {"code": -32602, "message": "unknown tool " + str(params.get("name"))}}, 200, ""
            text = str((params.get("arguments") or {}).get("text", ""))
            return ({"jsonrpc": "2.0", "id": message_id,
                     "result": {"content": [{"type": "text", "text": text + " qua " + self.mode}]}}, 200, "")
        return {"jsonrpc": "2.0", "id": message_id,
                "error": {"code": -32601, "message": "method not supported"}}, 200, ""

    def stop(self) -> None:
        self.server.shutdown()
        self.server.server_close()
