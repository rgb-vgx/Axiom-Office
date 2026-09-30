"""Logic cua wizard thiet lap - thuan Python, khong import UNO (test duoc bang tests/lo/test_setup.py).

`setupwizard.py` chi ve giao dien roi goi vao day. Cau chu + preset lay tu `setup_catalog.py`
(sinh tu catalog/setup.json, cung nguon voi ban Windows `SetupCatalog.cs`).
"""
from __future__ import annotations

from . import setup_catalog

STEP_IDS = [step["id"] for step in setup_catalog.STEPS]
DEFAULT_PROVIDER = "company"

def steps() -> list:
    return setup_catalog.STEPS

def provider(provider_id: str):
    """Preset nha cung cap; id la thi tra ve preset "company" (nguoi dung tu nhap dia chi)."""
    return setup_catalog.provider(provider_id) or setup_catalog.provider(DEFAULT_PROVIDER)

def check_meta(check_id: str):
    return setup_catalog.check(check_id)

class SetupState:
    """Trang thai mot lan chay wizard: buoc hien tai, gia tri da nhap, ket qua kiem tra.

    Khong giu khoa API trong `self` qua lau: `api_key` chi song trong luc nguoi dung nhap va bi xoa sau khi
    luu (`save()` tra ve gia tri da ma hoa cho nguoi goi ghi xuong cau hinh).
    """

    def __init__(self, kind: str = "wps"):
        self.kind = kind
        self.step = 0
        self.provider_id = DEFAULT_PROVIDER
        self.endpoint = ""
        self.model = ""
        self.api_key = ""
        self.has_stored_key = False      # da co khoa luu trong cau hinh -> de trong o khoa = giu nguyen
        self.test = None                 # ket qua /v1/llm/test (dict) hoac None
        self.testing = False
        self.models = []                 # danh sach model lay tu may chu
        self.models_status = ""          # cau tieng Viet khi khong lay duoc danh sach
        self.loading_models = False
        self.features = {feature["key"]: bool(feature["recommended"]) for feature in setup_catalog.FEATURES}
        self.checks = []                 # [{"id", "ok", "detail", "fixable", "fixed"}]
        self.core_error = ""             # Core khong chay / khong cai
        self.core_info = {}
        self.configured = False           # Core bao da co dia chi + model (mo lai wizard thi vao man kiem tra)
        self.touched = False              # nguoi dung da nhap/chon o buoc ket noi -> Core khong duoc ghi de
        self.done = False                # da bam "Hoan tat" (co luu cau hinh)

    # ---------------------------------------------------------------- dieu huong

    @property
    def step_id(self) -> str:
        return STEP_IDS[min(self.step, len(STEP_IDS) - 1)]

    @property
    def step_title(self) -> str:
        return steps()[min(self.step, len(steps()) - 1)]["title"]

    @property
    def step_subtitle(self) -> str:
        return steps()[min(self.step, len(steps()) - 1)]["subtitle"]

    @property
    def is_last(self) -> bool:
        return self.step >= len(steps()) - 1

    @property
    def is_first(self) -> bool:
        return self.step <= 0

    @property
    def progress(self) -> str:
        """Chi bao buoc dang o dang chu: "Buoc 3/5 - Ket noi may chu AI"."""
        return "Bước %d/%d — %s" % (self.step + 1, len(steps()), self.step_title)

    def go(self, step_id: str) -> None:
        if step_id in STEP_IDS:
            self.step = STEP_IDS.index(step_id)

    def next(self) -> bool:
        if self.is_last:
            return False
        self.step += 1
        return True

    def back(self) -> bool:
        if self.is_first:
            return False
        self.step -= 1
        return True

    @property
    def next_label(self) -> str:
        """Nhan nut chinh. Buoc ket noi: "Kiem tra ket noi" cho toi khi thu thanh cong, roi "Tiep tuc"."""
        if self.step_id == "connect":
            return "Tiếp tục →" if self.test and self.test.get("ok") else "Kiểm tra kết nối"
        return {"welcome": "Bắt đầu →", "done": "Hoàn tất"}.get(self.step_id, "Tiếp tục →")

    def update_fields(self, endpoint: str, model: str, api_key: str) -> bool:
        """Nhan gia tri o nhap khi nguoi dung dang go. Doi gia tri thi ket qua thu cu het hieu luc.

        Tra ve True neu co thay doi.
        """
        endpoint, model, api_key = endpoint.strip(), model.strip(), api_key.strip()
        if (endpoint, model, api_key) == (self.endpoint, self.model, self.api_key):
            return False
        if self.step_id == "connect":
            self.touched = True
        self.endpoint, self.model, self.api_key = endpoint, model, api_key
        self.test = None
        return True

    def can_continue(self) -> tuple:
        """(duoc phep di tiep?, ly do khi khong). Khong chan cung: buoc "Tinh nang"/"Hoan tat" luon di duoc."""
        if self.step_id == "connect":
            if not self.endpoint.strip():
                return False, "Chưa nhập địa chỉ máy chủ AI."
            if not self.model.strip():
                return False, "Chưa chọn model. Bấm \"Tải danh sách model\" rồi chọn."
        return True, ""

    # ---------------------------------------------------------------- nhap lieu

    def choose_provider(self, provider_id: str) -> None:
        """Doi nha cung cap: dien san dia chi + model goi y cua preset (khong ghi de gia tri nguoi dung da sua)."""
        preset = provider(provider_id)
        if preset is None:
            return
        self.touched = True
        self.provider_id = preset["id"]
        endpoint = preset["endpoint"] or ""
        if endpoint:
            self.endpoint = endpoint
        suggested = preset.get("suggestedModels") or []
        if suggested and not self.model.strip():
            self.model = suggested[0]
        self.test = None
        self.models = []
        self.models_status = ""

    @property
    def codec(self) -> str:
        """Provider cho Agent Core: chi "anthropic" khac, con lai la OpenAI-compatible."""
        return provider(self.provider_id).get("codec", "openai")

    @property
    def needs_key(self) -> bool:
        return bool(provider(self.provider_id).get("needsKey"))

    @property
    def key_url(self) -> str:
        return provider(self.provider_id).get("keyUrl", "")

    def validation_error(self) -> str:
        """Cau tieng Viet cho o nhap dau tien con thieu ("" neu day du)."""
        if not self.endpoint.strip():
            return "Điền địa chỉ máy chủ AI (ví dụ http://may-chu-cong-ty/v1)."
        if not self.model.strip():
            return "Chọn model (bấm \"Tải danh sách model\" để lấy danh sách từ máy chủ)."
        if self.needs_key and not self.api_key.strip() and not self.has_stored_key:
            return "Nhập khoá API (bấm \"Lấy khoá ở đâu?\" để mở trang lấy khoá)."
        return ""

    def test_payload(self) -> dict:
        """Body cho POST /v1/llm/test: gui gia tri nguoi dung vua nhap, CHUA luu."""
        return {
            "providerId": self.provider_id,
            "provider": self.codec,
            "endpoint": self.endpoint.strip(),
            "model": self.model.strip(),
            "apiKey": self.api_key.strip(),
        }

    def models_query(self) -> dict:
        return {"provider": self.codec, "endpoint": self.endpoint.strip(), "apiKey": self.api_key.strip()}

    # ---------------------------------------------------------------- ket qua

    def apply_test(self, result: dict) -> None:
        """Nhan ket qua /v1/llm/test (hoac cau loi tu UI khi Core khong chay)."""
        self.test = dict(result or {})
        self.testing = False
        if self.test.get("ok"):
            model = self.test.get("model") or self.model
            if model:
                self.model = str(model)

    def test_line(self) -> str:
        """Cau hien duoi nut "Kiem tra ket noi" ("" neu chua thu)."""
        if not self.test:
            return ""
        if self.test.get("ok"):
            seconds = self.test.get("seconds")
            reply = (self.test.get("reply") or "").strip()
            where = " (%.1f giây)" % float(seconds) if isinstance(seconds, (int, float)) else ""
            return "Kết nối tốt%s — máy chủ trả lời: %s" % (where, reply[:60] or "OK")
        return (self.test.get("message") or "Chưa kết nối được.") + (
            " " + self.test["hint"] if self.test.get("hint") else "")

    def test_kind(self) -> str:
        if not self.test:
            return ""
        return "ok" if self.test.get("ok") else "error"

    def apply_models(self, models: list) -> None:
        self.models = list(models or [])
        self.models_status = "" if self.models else "Máy chủ không trả về model nào — bạn tự nhập tên model."
        self.loading_models = False
        suggested = provider(self.provider_id).get("suggestedModels") or []
        if self.models and not self.model.strip():
            self.model = self.models[0]
        elif not self.models and suggested and not self.model.strip():
            self.model = suggested[0]

    def apply_models_error(self, message: str) -> None:
        self.loading_models = False
        self.models = []
        suggested = provider(self.provider_id).get("suggestedModels") or []
        self.models_status = message + (" Vẫn có thể tự nhập tên model." if not suggested else "")

    # ---------------------------------------------------------------- trang thai Core

    def apply_core_payload(self, result: dict) -> None:
        """Nhan `result` cua GET /v1/setup (core.call tra ve result): dien theo cau hinh dang chay + tinh nang."""
        self.core_error = ""
        self.core_info = dict(result.get("core") or {})
        current = dict(result.get("current") or {})
        self.has_stored_key = bool(current.get("hasKey"))
        self.configured = bool(current.get("configured"))
        # "Kiem tra may" chay nen ngay khi wizard mo; neu nguoi dung da go/chon truoc khi no xong thi KHONG ghi
        # de (truoc day dia chi vua go bi thay bang gia tri cu trong cau hinh).
        if not self.touched:
            if current.get("endpoint"):
                self.endpoint = str(current["endpoint"])
            if current.get("model"):
                self.model = str(current["model"])
            provider_id = str(result.get("providerId") or "").strip()
            if provider_id:
                self.provider_id = provider_id
            elif current.get("endpoint"):
                self.provider_id = setup_catalog.guess_provider_id(str(current["endpoint"]),
                                                                   str(current.get("provider") or ""))
        for feature in result.get("features") or []:
            key = feature.get("key")
            if key in self.features:
                self.features[key] = bool(current.get(_feature_key(key), self.features[key]))

    def apply_core_error(self, message: str) -> None:
        self.core_error = message
        self.core_info = {}
        self.configured = False

    def problem_ids(self, problems: list) -> list:
        """Id cac van de Core bao (de UI danh dau dong kiem tra tuong ung)."""
        return [str(item.get("id")) for item in (problems or [])]

    # ---------------------------------------------------------------- tom tat / luu

    def summary(self) -> str:
        preset = provider(self.provider_id)
        parts = ["Máy chủ: %s" % (preset["label"] if preset else self.endpoint)]
        if self.endpoint.strip():
            parts.append(self.endpoint.strip())
        parts.append("Model: %s" % (self.model.strip() or "chưa chọn"))
        parts.append("Ghi nhớ: %s" % ("bật" if self.features.get("MemoryEnabled") else "tắt"))
        if self.core_info.get("port"):
            parts.append("Agent Core: cổng %s" % self.core_info["port"])
        return " · ".join(parts)

    def replace_document_hint(self) -> str:
        """Cau huong dan o buoc cuoi (khac nhau theo app dang mo)."""
        names = {"wps": "Writer", "et": "Calc", "wpp": "Impress"}
        return ("Mở menu Axiom Office → Hỏi AI (hoặc tab Axiom Office ở sidebar) để bắt đầu trong %s."
                % names.get(self.kind, "LibreOffice"))

def payload_to_test_result(result) -> dict:
    """Ket qua POST /v1/llm/test. `core.call` tra ve `result` (khong con vo {"ok":...}).

    Endpoint tra {kind, message, hint, detail} khi khong ket noi duoc, va {seconds, model, reply} khi tot.
    """
    if not isinstance(result, dict):
        return {"kind": "internal", "message": "Agent Core trả lời không đúng định dạng.",
                "hint": "Thử lại; nếu vẫn lỗi, mở Cài đặt nâng cao để xem log."}
    if result.get("kind"):
        return dict(result)
    return dict(result, ok=True)

def payload_to_models(result) -> tuple:
    """Ket qua GET /v1/llm/models -> (danh sach model, cau loi tieng Viet)."""
    if not isinstance(result, dict):
        return [], "Agent Core trả lời không đúng định dạng."
    if result.get("models") is not None:
        return [str(name) for name in result.get("models") or []], ""
    message = result.get("message") or "Không lấy được danh sách model."
    if result.get("hint"):
        message = "%s %s" % (message, result["hint"])
    return [], str(message)

def _feature_key(config_key: str) -> str:
    """Khoa trong /v1/setup.current (kieu camelCase) tu khoa cau hinh (MemoryEnabled...)."""
    return config_key[0].lower() + config_key[1:]
