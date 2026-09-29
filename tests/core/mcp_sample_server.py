"""MCP server mau (stdio, chi thu vien chuan) cho test e2e cua Agent Core (New_arch.md muc 8.7, 12).

    python tests/core/mcp_sample_server.py

Tool: add {a, b} -> tong; shout {text} -> chu hoa; fail {} -> isError. Moi thong diep JSON-RPC mot dong.
"""
from __future__ import annotations

import json
import sys

TOOLS = [
    {"name": "add", "description": "Cong hai so",
     "inputSchema": {"type": "object", "properties": {"a": {"type": "number"}, "b": {"type": "number"}}, "required": ["a", "b"]}},
    {"name": "shout", "description": "Viet hoa mot doan chu",
     "inputSchema": {"type": "object", "properties": {"text": {"type": "string"}}, "required": ["text"]}},
    {"name": "fail", "description": "Luon bao loi (test isError)", "inputSchema": {"type": "object", "properties": {}}},
]


def reply(message_id, result=None, error=None) -> None:
    payload = {"jsonrpc": "2.0", "id": message_id}
    if error is not None:
        payload["error"] = error
    else:
        payload["result"] = result
    sys.stdout.write(json.dumps(payload, ensure_ascii=False) + "\n")
    sys.stdout.flush()


def call(name: str, arguments: dict) -> dict:
    if name == "add":
        return {"content": [{"type": "text", "text": str(arguments.get("a", 0) + arguments.get("b", 0))}]}
    if name == "shout":
        return {"content": [{"type": "text", "text": str(arguments.get("text", "")).upper()}]}
    if name == "fail":
        return {"content": [{"type": "text", "text": "cong cu nay luon loi"}], "isError": True}
    return {"content": [{"type": "text", "text": "unknown tool " + name}], "isError": True}


def main() -> int:
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stdin.reconfigure(encoding="utf-8")
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        message = json.loads(line)
        method = message.get("method")
        message_id = message.get("id")
        if message_id is None:
            continue  # notification (notifications/initialized)
        if method == "initialize":
            reply(message_id, {"protocolVersion": message.get("params", {}).get("protocolVersion", "2025-06-18"),
                               "capabilities": {"tools": {}}, "serverInfo": {"name": "mau", "version": "1.0"}})
        elif method == "tools/list":
            reply(message_id, {"tools": TOOLS})
        elif method == "tools/call":
            params = message.get("params", {})
            reply(message_id, call(params.get("name", ""), params.get("arguments") or {}))
        else:
            reply(message_id, error={"code": -32601, "message": "method not found: " + str(method)})
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
