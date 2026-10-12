"""Registry lenh cua bridge LibreOffice: cung ten, loai app, tham so, co ForAgent voi CommandDispatcher (C#)
de Agent Core dung lai allowlist va skill (LibreOffice_arch.md LD-2). Them lenh = them mot dong command(...)."""
from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class Param:
    name: str
    required: bool = False
    hint: str | None = None


@dataclass
class Command:
    name: str
    kind: str | None
    handler: object
    summary: str
    params: list = field(default_factory=list)
    agent: bool = False
    gated: bool = True        # False: khong chay trong UnoGate (lenh tu goi gate / ai.ask)
    undo: bool = False        # True: gom thanh mot buoc Undo


REGISTRY: dict[str, Command] = {}


def req(name: str, hint: str | None = None) -> Param:
    return Param(name, True, hint)


def opt(name: str, hint: str | None = None) -> Param:
    return Param(name, False, hint)


def command(name, kind, handler, summary, *params, agent=False, gated=True, undo=False) -> Command:
    if name in REGISTRY:
        prev = REGISTRY[name]
        old = prev.handler
        mod = getattr(old, "__module__", None) or "?"
        qual = getattr(old, "__qualname__", None) or getattr(old, "__name__", None) or repr(old)
        raise ValueError(f"trung lenh '{name}': da dang ky boi {mod}.{qual}")
    item = Command(name, kind, handler, summary, list(params), agent, gated, undo)
    REGISTRY[name] = item
    return item


def catalog() -> list:
    return [{
        "name": c.name,
        "kind": c.kind,
        "agent": c.agent,
        "summary": c.summary,
        "params": [{"name": p.name, "required": p.required, "hint": p.hint} for p in c.params],
    } for c in REGISTRY.values()]


def load_all() -> None:
    # Import de cac module tu dang ky lenh.
    from . import general, writer, calc, impress, checks  # noqa: F401
