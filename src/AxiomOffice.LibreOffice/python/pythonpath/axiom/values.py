"""Doc tham so lenh - port tu CommandDispatcher.Params.cs de LibreOffice nhan dung cac dang ma model hay gui:
so dang chuoi, {"item": ...} kieu mang XML, mang bi boc thua, values la chuoi JSON (xem CHANGELOG)."""
from __future__ import annotations

import json


class ParamError(ValueError):
    """Tham so sai: bao ten tham so + gia tri nhan duoc de model tu sua (giong ArgumentException ben C#)."""


WRAPPER_KEYS = ("item", "items", "row", "rows", "values")


def describe(value) -> str:
    if isinstance(value, dict):
        return "an object with keys [%s]" % ", ".join(list(value.keys())[:5])
    if isinstance(value, str):
        return "'%s'" % value
    return "null" if value is None else repr(value)


def has(params: dict, name: str) -> bool:
    return isinstance(params, dict) and params.get(name) is not None


def unwrap_scalar(value):
    """{"item": 6} / [6] -> 6 (toi da 8 lop)."""
    for _ in range(8):
        if isinstance(value, dict) and len(value) == 1:
            key = next(iter(value))
            if key not in ("item", "items", "value"):
                return value
            value = value[key]
            continue
        if isinstance(value, list) and len(value) == 1:
            value = value[0]
            continue
        return value
    return value


def string(params: dict, name: str, default=None):
    value = params.get(name) if isinstance(params, dict) else None
    if value is None:
        return default
    return value if isinstance(value, str) else str(unwrap_scalar(value))


def integer(params: dict, name: str, default: int) -> int:
    value = params.get(name) if isinstance(params, dict) else None
    if value is None:
        return default
    value = unwrap_scalar(value)
    try:
        if isinstance(value, bool):
            raise TypeError
        if isinstance(value, str):
            return int(round(float(value.strip())))
        return int(round(float(value)))
    except (TypeError, ValueError):
        raise ParamError("'%s' must be a whole number, got %s" % (name, describe(value)))


def number(params: dict, name: str, default: float) -> float:
    value = params.get(name) if isinstance(params, dict) else None
    if value is None:
        return default
    value = unwrap_scalar(value)
    try:
        return float(value.strip() if isinstance(value, str) else value)
    except (TypeError, ValueError):
        raise ParamError("'%s' must be a number, got %s" % (name, describe(value)))


def boolean(params: dict, name: str, default: bool) -> bool:
    value = params.get(name) if isinstance(params, dict) else None
    if value is None:
        return default
    value = unwrap_scalar(value)
    if isinstance(value, bool):
        return value
    if isinstance(value, (int, float)):
        return value != 0
    text = str(value).strip().lower()
    if text in ("true", "1", "yes", "on"):
        return True
    if text in ("false", "0", "no", "off"):
        return False
    raise ParamError("'%s' must be true or false, got %s" % (name, describe(value)))


def _decode_arrays(value):
    """Moi lop {"item": x} la MOT cap mang: x la danh sach phan tu, hoac phan tu duy nhat khi khong la mang."""
    if isinstance(value, dict) and len(value) == 1:
        key = next(iter(value))
        if key in WRAPPER_KEYS:
            inner = value[key]
            return [_decode_arrays(i) for i in inner] if isinstance(inner, list) else [_decode_arrays(inner)]
    if isinstance(value, list):
        return [_decode_arrays(i) for i in value]
    return value


def _matrix_help(name: str, got: str) -> str:
    return ("'%s' must be a JSON 2D array (a list of rows), e.g. [[\"Họ tên\",\"Điểm\"],[\"An\",9.5],[\"Bình\",8]] - got %s"
            % (name, got))


def matrix(params: dict, name: str, required: bool):
    """values 2 chieu: gia tri o la str / so / bool / None."""
    raw = params.get(name) if isinstance(params, dict) else None
    if raw is None:
        if required:
            raise ParamError(_matrix_help(name, "missing"))
        return None
    if isinstance(raw, str) and raw.lstrip()[:1] in ("[", "{"):
        try:
            raw = json.loads(raw)
        except ValueError:
            pass
    data = _decode_arrays(raw)
    if not isinstance(data, list):
        raise ParamError(_matrix_help(name, describe(data)))
    while len(data) == 1 and isinstance(data[0], list) and data[0] and all(isinstance(i, list) for i in data[0]):
        data = data[0]
    if not data:
        if required:
            raise ParamError(_matrix_help(name, "an empty array"))
        return []
    all_rows = all(isinstance(i, list) for i in data)
    all_scalars = all(not isinstance(i, (list, dict)) for i in data)
    if not all_rows and not all_scalars:
        raise ParamError(_matrix_help(name, "a mix of rows and single values"))
    rows = []
    for r, item in enumerate(data):
        source = item if all_rows else [item]
        while len(source) == 1 and isinstance(source[0], list):
            source = source[0]
        row = []
        for c, cell in enumerate(source):
            while isinstance(cell, list) and len(cell) == 1:
                cell = cell[0]
            if isinstance(cell, (list, dict)):
                raise ParamError(_matrix_help(name, "a nested array/object at row %d, column %d" % (r + 1, c + 1)))
            row.append(cell)
        rows.append(row)
    return rows


def color(value) -> int | None:
    """'#RRGGBB' / 'RRGGBB' -> so RGB cua LibreOffice (0xRRGGBB, khac BGR cua Office)."""
    if not value:
        return None
    text = str(value).strip().lstrip("#")
    if len(text) != 6:
        return None
    try:
        return int(text, 16)
    except ValueError:
        return None


POINT_TO_HMM = 35.2778  # 1pt = 0.352778mm = 35.2778 (1/100 mm)
