"""Unit test cho logic wizard thiet lap (axiom.setup) - khong can LibreOffice lan Agent Core.

Phan ve giao dien (setupwizard.py) kiem tra rieng tren LibreOffice that; o day giu dung phan de sai nhat:
dieu huong buoc, xac thuc o nhap, ghep ket qua Core tra ve thanh cau tieng Viet, va cac preset/tham so
gui len Core (phai khop Api/SetupEndpoints.cs).
"""
from __future__ import annotations

import os
import sys
import types
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PYTHONPATH = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath")
if "uno" not in sys.modules:
    sys.modules["uno"] = types.ModuleType("uno")
sys.path.insert(0, PYTHONPATH)

from axiom import setup, setup_catalog  # noqa: E402

class StepTests(unittest.TestCase):
    def test_nam_buoc_dung_thu_tu(self):
        self.assertEqual(setup.STEP_IDS, ["welcome", "checks", "connect", "features", "done"])
        state = setup.SetupState()
        self.assertEqual(state.step_id, "welcome")
        self.assertTrue(state.is_first)
        self.assertFalse(state.is_last)
        self.assertEqual(state.progress, "Bước 1/5 — " + setup_catalog.STEPS[0]["title"])
        for expected in ("checks", "connect", "features", "done"):
            state.next()
            self.assertEqual(state.step_id, expected)
        self.assertTrue(state.is_last)
        self.assertFalse(state.next())      # buoc cuoi khong di tiep
        self.assertTrue(state.back())
        self.assertEqual(state.step_id, "features")

    def test_nhay_toi_buoc_theo_id(self):
        state = setup.SetupState()
        state.go("connect")
        self.assertEqual(state.step_id, "connect")
        state.go("khong-co")
        self.assertEqual(state.step_id, "connect")     # id la thi giu nguyen

class ProviderTests(unittest.TestCase):
    def test_mac_dinh_la_may_chu_cong_ty(self):
        state = setup.SetupState()
        self.assertEqual(state.provider_id, "company")
        self.assertEqual(state.endpoint, "")
        self.assertEqual(state.codec, "openai")
        self.assertFalse(state.needs_key)

    def test_chon_openai_dien_san_dia_chi_va_model_goi_y(self):
        state = setup.SetupState()
        state.choose_provider("openai")
        self.assertEqual(state.endpoint, "https://api.openai.com/v1")
        self.assertEqual(state.model, "gpt-4o-mini")
        self.assertEqual(state.codec, "openai")
        self.assertTrue(state.needs_key)
        self.assertIn("platform.openai.com", state.key_url)

    def test_anthropic_dung_codec_rieng(self):
        state = setup.SetupState()
        state.choose_provider("anthropic")
        self.assertEqual(state.codec, "anthropic")
        self.assertEqual(state.endpoint, "https://api.anthropic.com/v1")

    def test_doi_nha_cung_cap_khong_de_mat_model_nguoi_dung_da_sua(self):
        state = setup.SetupState()
        state.choose_provider("company")
        state.model = "model-cua-cong-ty"
        state.choose_provider("openai")
        self.assertEqual(state.model, "model-cua-cong-ty")     # giu nguyen, khong de model goi y

    def test_id_la_thi_dung_preset_mac_dinh(self):
        state = setup.SetupState()
        state.choose_provider("khong-co")
        self.assertEqual(state.provider_id, "company")

class ValidationTests(unittest.TestCase):
    def test_thieu_dia_chi(self):
        state = setup.SetupState()
        state.choose_provider("openai")
        state.endpoint = "  "
        self.assertIn("địa chỉ máy chủ", state.validation_error())

    def test_thieu_model(self):
        state = setup.SetupState()
        state.choose_provider("openai")
        state.model = ""
        self.assertIn("model", state.validation_error())

    def test_thieu_khoa_voi_nha_cung_cap_can_khoa(self):
        state = setup.SetupState()
        state.choose_provider("openai")
        self.assertIn("khoá API", state.validation_error())

    def test_co_khoa_da_luu_thi_khong_bat_nhap_lai(self):
        state = setup.SetupState()
        state.choose_provider("openai")
        state.has_stored_key = True
        self.assertEqual(state.validation_error(), "")

    def test_may_chu_noi_bo_khong_bat_buoc_khoa(self):
        state = setup.SetupState()
        state.endpoint = "http://may-chu-noi-bo:20128/v1"
        state.model = "noi-bo"
        self.assertEqual(state.validation_error(), "")

    def test_chan_di_tiep_khi_chua_co_dia_chi_va_model(self):
        state = setup.SetupState()
        state.go("connect")
        allowed, reason = state.can_continue()
        self.assertFalse(allowed)
        self.assertIn("địa chỉ", reason)
        state.endpoint = "http://may-chu/v1"
        allowed, reason = state.can_continue()
        self.assertFalse(allowed)
        self.assertIn("model", reason)
        state.model = "m1"
        self.assertEqual(state.can_continue(), (True, ""))

    def test_nut_chinh_buoc_ket_noi_doi_thanh_tiep_tuc_khi_thu_thanh_cong(self):
        state = setup.SetupState()
        state.go("connect")
        self.assertEqual(state.next_label, "Kiểm tra kết nối")
        state.apply_test({"ok": False, "message": "Lỗi"})
        self.assertEqual(state.next_label, "Kiểm tra kết nối")
        state.apply_test({"ok": True, "reply": "OK"})
        self.assertEqual(state.next_label, "Tiếp tục →")
        state.go("features")
        self.assertEqual(state.next_label, "Tiếp tục →")
        state.go("done")
        self.assertEqual(state.next_label, "Hoàn tất")

    def test_go_vao_o_nhap_cap_nhat_trang_thai_va_bo_ket_qua_thu_cu(self):
        state = setup.SetupState()
        state.go("connect")
        self.assertTrue(state.update_fields(" http://may-chu/v1 ", "m1", ""))
        self.assertEqual((state.endpoint, state.model), ("http://may-chu/v1", "m1"))
        self.assertTrue(state.touched)
        self.assertEqual(state.can_continue(), (True, ""))
        state.apply_test({"ok": True, "reply": "OK"})
        self.assertFalse(state.update_fields("http://may-chu/v1", "m1", ""))   # khong doi -> giu ket qua thu
        self.assertEqual(state.next_label, "Tiếp tục →")
        self.assertTrue(state.update_fields("http://may-chu/v1", "m2", ""))    # doi model -> phai thu lai
        self.assertIsNone(state.test)
        self.assertEqual(state.next_label, "Kiểm tra kết nối")

    def test_doi_khoa_api_cung_lam_het_hieu_luc_ket_qua_thu(self):
        """Khoa la mot phan cua lan thu: doi khoa thi ket qua cu (theo khoa cu) khong con dung."""
        state = setup.SetupState()
        state.go("connect")
        state.update_fields("http://may-chu/v1", "m1", "khoa-cu")
        state.apply_test({"ok": True, "reply": "OK"})
        self.assertTrue(state.update_fields("http://may-chu/v1", "m1", "khoa-moi"))
        self.assertIsNone(state.test)
        self.assertEqual(state.next_label, "Kiểm tra kết nối")

    def test_go_o_nhap_ngoai_buoc_ket_noi_khong_danh_dau_touched(self):
        """`touched` chan Core ghi de gia tri nguoi dung; chi co y nghia o buoc ket noi."""
        state = setup.SetupState()
        state.go("features")
        state.update_fields("http://may-chu/v1", "m1", "")
        self.assertFalse(state.touched)

    def test_gui_len_core_dung_ten_tham_so(self):
        state = setup.SetupState()
        state.choose_provider("anthropic")
        state.endpoint = " https://api.anthropic.com/v1 "
        state.model = " claude-x "
        state.api_key = " sk-ant "
        self.assertEqual(state.test_payload(), {
            "providerId": "anthropic", "provider": "anthropic",
            "endpoint": "https://api.anthropic.com/v1", "model": "claude-x", "apiKey": "sk-ant"})
        self.assertEqual(state.models_query(), {
            "provider": "anthropic", "endpoint": "https://api.anthropic.com/v1", "apiKey": "sk-ant"})

class ResultTests(unittest.TestCase):
    def test_ket_qua_thanh_cong(self):
        state = setup.SetupState()
        state.apply_test({"ok": True, "seconds": 0.84, "reply": "OK", "model": "fake-model"})
        self.assertEqual(state.test_kind(), "ok")
        self.assertIn("Kết nối tốt", state.test_line())
        self.assertIn("0.8 giây", state.test_line())
        self.assertIn("OK", state.test_line())
        self.assertEqual(state.model, "fake-model")     # lay ten model may chu tra ve

    def test_ket_qua_loi_co_goi_y(self):
        state = setup.SetupState()
        state.apply_test({"ok": False, "kind": "auth", "message": "Máy chủ AI từ chối khoá truy cập (401).",
                          "hint": "Có thể khoá sai.", "detail": "HTTP 401: ..."})
        self.assertEqual(state.test_kind(), "error")
        self.assertIn("từ chối khoá", state.test_line())
        self.assertIn("khoá sai", state.test_line())

    def test_danh_sach_model(self):
        state = setup.SetupState()
        state.loading_models = True
        state.apply_models(["m1", "m2"])
        self.assertEqual(state.models, ["m1", "m2"])
        self.assertFalse(state.loading_models)
        self.assertEqual(state.models_status, "")
        self.assertEqual(state.model, "m1")            # tu chon model dau tien

    def test_danh_sach_model_rong_thi_giu_goi_y(self):
        state = setup.SetupState()
        state.choose_provider("openai")
        state.model = ""
        state.apply_models([])
        self.assertIn("tự nhập", state.models_status)
        self.assertEqual(state.model, "gpt-4o-mini")

    def test_loi_lay_danh_sach_model(self):
        state = setup.SetupState()
        state.loading_models = True
        state.apply_models_error("Máy chủ AI từ chối khoá truy cập (401).")
        self.assertIn("từ chối khoá", state.models_status)
        self.assertFalse(state.loading_models)

    def test_chuan_hoa_response_cua_core(self):
        # core.call tra ve `result`: co "kind" la loi, khong co la thanh cong.
        self.assertEqual(setup.payload_to_test_result({"reply": "OK", "seconds": 0.5}),
                         {"reply": "OK", "seconds": 0.5, "ok": True})
        failure = setup.payload_to_test_result({"kind": "auth", "message": "Khoá sai.", "hint": "Kiểm tra lại."})
        self.assertEqual(failure["kind"], "auth")
        self.assertFalse(failure.get("ok"))
        self.assertEqual(setup.payload_to_test_result(None)["kind"], "internal")

    def test_chuan_hoa_danh_sach_model(self):
        models, error = setup.payload_to_models({"models": ["a", "b"], "count": 2})
        self.assertEqual((models, error), (["a", "b"], ""))
        models, error = setup.payload_to_models({"kind": "auth", "message": "Khoá sai.", "hint": "Kiểm tra lại."})
        self.assertEqual(models, [])
        self.assertIn("Khoá sai", error)
        self.assertIn("Kiểm tra lại", error)

    def test_doc_trang_thai_tu_core(self):
        state = setup.SetupState()
        state.apply_core_payload({
            "providerId": "openai",
            "current": {"endpoint": "https://api.openai.com/v1", "model": "gpt-4o-mini", "hasKey": True,
                        "memoryEnabled": False, "memoryAutoExtract": True, "visualQaEnabled": True},
            "features": [{"key": "MemoryEnabled"}, {"key": "MemoryAutoExtract"}, {"key": "VisualQaEnabled"}],
            "core": {"port": 47840, "pid": 11, "skills": 7},
        })
        self.assertEqual(state.provider_id, "openai")
        self.assertEqual(state.endpoint, "https://api.openai.com/v1")
        self.assertEqual(state.model, "gpt-4o-mini")
        self.assertTrue(state.has_stored_key)
        self.assertFalse(state.features["MemoryEnabled"])
        self.assertTrue(state.features["VisualQaEnabled"])
        self.assertEqual(state.core_info["port"], 47840)
        self.assertIn("cổng 47840", state.summary())
        self.assertIn("Model: gpt-4o-mini", state.summary())
        self.assertIn("Ghi nhớ: tắt", state.summary())

    def test_ket_qua_kiem_tra_ve_sau_khong_ghi_de_gia_tri_nguoi_dung_da_nhap(self):
        # "Kiem tra may" chay nen luc mo wizard; nguoi dung go dia chi truoc khi no xong -> giu nguyen.
        state = setup.SetupState()
        state.choose_provider("company")
        state.endpoint, state.model = "http://may-moi/v1", "model-moi"
        state.apply_core_payload({"providerId": "openai",
                                  "current": {"endpoint": "https://api.openai.com/v1", "model": "cu", "hasKey": True}})
        self.assertEqual((state.provider_id, state.endpoint, state.model), ("company", "http://may-moi/v1", "model-moi"))
        self.assertTrue(state.has_stored_key)          # thong tin trang thai van cap nhat

    def test_doan_nha_cung_cap_khi_core_khong_tra_providerId(self):
        state = setup.SetupState()
        state.apply_core_payload({"current": {"endpoint": "http://may-chu-noi-bo/v1", "model": "m", "hasKey": False}})
        self.assertEqual(state.provider_id, "company")

    def test_core_khong_chay_thi_ghi_loi_va_khong_co_thong_tin(self):
        state = setup.SetupState()
        state.apply_core_payload({"core": {"port": 1}, "current": {}})
        state.apply_core_error("Agent Core chưa chạy.")
        self.assertEqual(state.core_error, "Agent Core chưa chạy.")
        self.assertEqual(state.core_info, {})

    def test_problem_ids_cua_core(self):
        state = setup.SetupState()
        self.assertEqual(state.problem_ids([{"id": "config"}, {"id": "token"}]), ["config", "token"])

    def test_cau_huong_dan_cuoi_buoc_theo_app(self):
        self.assertIn("Writer", setup.SetupState("wps").replace_document_hint())
        self.assertIn("Calc", setup.SetupState("et").replace_document_hint())
        self.assertIn("Impress", setup.SetupState("wpp").replace_document_hint())

class CheckVerdictTests(unittest.TestCase):
    """Cau ket luan o cuoi man "Kiem tra may" (dong trang thai)."""

    def test_moi_thu_dat_thi_bao_on(self):
        self.assertEqual(setup.checks_verdict([("Agent Core đang chạy", True), ("Cầu nối", True)]),
                         "Mọi thứ đều ổn.")

    def test_co_viec_chua_xong_thi_liet_ke_dung_cac_viec_do(self):
        self.assertEqual(setup.checks_verdict([("Agent Core đang chạy", False), ("Cầu nối", True),
                                               ("Tệp cấu hình", False)]),
                         "Cần chú ý: Agent Core đang chạy; Tệp cấu hình")

    def test_chua_kiem_tra_xong_thi_khong_noi_gi(self):
        # Truoc day cho nay tra ve "Mọi thứ đều ổn." ngay khi vua mo wizard (chua kiem tra gi ca).
        self.assertEqual(setup.checks_verdict([("Agent Core đang chạy", None), ("Cầu nối", True)]), "")

    def test_loi_core_duoc_uu_tien_hon_danh_sach(self):
        self.assertEqual(setup.checks_verdict([("Agent Core đang chạy", False)], "Không chạy được Agent Core."),
                         "Không chạy được Agent Core.")

    def test_khong_co_dong_nao_thi_coi_nhu_on(self):
        self.assertEqual(setup.checks_verdict([]), "Mọi thứ đều ổn.")

class CatalogViewTests(unittest.TestCase):
    def test_metadata_check_khop_catalog(self):
        meta = setup.check_meta("core")
        self.assertTrue(meta["fixable"])
        self.assertTrue(meta["fixLabel"])
        self.assertIsNone(setup.check_meta("khong-co"))

    def test_tinh_nang_mac_dinh_theo_catalog(self):
        state = setup.SetupState()
        self.assertTrue(state.features["MemoryEnabled"])
        self.assertTrue(state.features["MemoryAutoExtract"])
        self.assertFalse(state.features["VisualQaEnabled"])

if __name__ == "__main__":
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    unittest.main(verbosity=2)
