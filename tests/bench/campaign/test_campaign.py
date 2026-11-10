from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import record as record_module  # noqa: E402
import records  # noqa: E402
import report as report_module  # noqa: E402
import stats  # noqa: E402


def make(pair_id: str, task: str, role: str, variant: str, success: str = "S3", billable: int = 1000,
         tool_calls: int = 5, rounds: int = 3, failure: str = "") -> records.RunRecord:
    return records.RunRecord(
        run_id=f"{pair_id}-{role}", pair_id=pair_id, task_id=task, variant=variant, role=role,
        success=success, billable=billable, tool_calls=tool_calls, rounds=rounds, failure=failure,
    )


def pair(pair_id: str, success_control: str, success_treatment: str, **kwargs) -> records.Pair:
    control = make(pair_id, kwargs.get("task", "W03"), "control", "B0", success_control,
                   kwargs.get("billable_control", 1000))
    treatment = make(pair_id, kwargs.get("task", "W03"), "treatment", "B2", success_treatment,
                     kwargs.get("billable_treatment", 800))
    return records.Pair(pair_id, control.task_id, control, treatment)


class CampaignTests(unittest.TestCase):
    def test_mcnemar_exact_known_example(self):
        items = [pair("p1", "S3", "S3"), pair("p2", "S3", "S3"), pair("p3", "S3", "S3"),
                 pair("p4", "S3", "S3"), pair("p5", "S3", "S3"), pair("p6", "S3", "S3"),
                 pair("p7", "S3", "S3"), pair("p8", "S0", "S0"),
                 pair("p9", "S3", "S0"), pair("p10", "S0", "S3"),
                 pair("p11", "S0", "S3"), pair("p12", "S0", "S3")]
        self.assertEqual(stats.mcnemar_exact(items), {
            "both_pass": 7, "both_fail": 1, "control_only": 1,
            "treatment_only": 3, "discordant": 4, "p_value": 0.625,
        })

    def test_pair_records_rejects_wrong_shape(self):
        duplicated = [make("p1", "W03", "control", "B0"), make("p1", "W03", "control", "B0")]
        with self.assertRaisesRegex(ValueError, "control"):
            records.pair_records(duplicated)

        bad_control = [make("p1", "W03", "control", "B1"), make("p1", "W03", "treatment", "B2")]
        with self.assertRaisesRegex(ValueError, "B0"):
            records.pair_records(bad_control)

    def test_median_reduction_only_counts_successful_pairs(self):
        items = [pair("p1", "S3", "S3", billable_control=1000, billable_treatment=800),
                 pair("p2", "S3", "S3", billable_control=1000, billable_treatment=500),
                 pair("p3", "S3", "S0", billable_control=1000, billable_treatment=100)]
        result = stats.median_reduction(items, "billable")
        self.assertEqual(result["n"], 2)
        self.assertAlmostEqual(result["median"], 0.35)  # median(20%, 50%)

    def test_cost_per_success_includes_failed_runs_in_numerator(self):
        rows = [make("p1", "W03", "treatment", "B2", "S3", billable=10),
                make("p2", "W03", "treatment", "B2", "S0", billable=30)]
        self.assertEqual(stats.cost_per_success(rows), 40.0)
        self.assertIsNone(stats.cost_per_success([rows[1]]))

    def test_bootstrap_is_deterministic_and_bounded(self):
        items = [pair(f"p{i}", "S3", "S3", billable_control=1000, billable_treatment=1000 - i * 50)
                 for i in range(1, 8)]
        first = stats.bootstrap_reduction_ci(items, "billable", iterations=200, seed=7)
        second = stats.bootstrap_reduction_ci(items, "billable", iterations=200, seed=7)
        self.assertEqual(first, second)
        self.assertLessEqual(first[0], first[1])

    def test_decision_hints_hold_on_small_or_wide_samples(self):
        one = stats.variant_summary([pair("p1", "S3", "S3")], bootstrap=50)
        self.assertIn("HOLD", stats.decision_hint(one))
        wide = stats.variant_summary(
            [pair(f"p{i}", "S3", "S3", billable_control=1000, billable_treatment=1000 - i * 300)
             for i in range(1, 6)], bootstrap=50)
        # Dù median giảm mạnh, CI rộng (0%…60%+) phải là HOLD, không phải KEEP.
        self.assertIn("HOLD", stats.decision_hint(wide))

    def test_report_renders_variant_and_decision(self):
        rows = []
        for index in range(1, 5):
            rows.append(make(f"p{index}", "W03", "control", "B0", "S3", billable=1000))
            rows.append(make(f"p{index}", "W03", "treatment", "B2", "S3", billable=700))
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "runs.jsonl"
            records.write_jsonl(path, rows)
            text = report_module.render(str(path), "S3", bootstrap=100)
        self.assertIn("| B2 |", text)
        self.assertIn("KEEP", text)
        self.assertIn("W03", text)
        self.assertIn("McNemar", text)

    def test_record_cli_appends(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "runs.jsonl"
            self.assertEqual(record_module.main([
                "--out", str(path), "--run-id", "r1", "--pair", "p1", "--task", "W03",
                "--variant", "B0", "--role", "control", "--success", "S3"]), 0)
            self.assertEqual(record_module.main([
                "--out", str(path), "--run-id", "r2", "--pair", "p1", "--task", "W03",
                "--variant", "B2", "--role", "treatment", "--success", "S3"]), 0)
            loaded = records.load_jsonl(path)
            self.assertEqual(len(loaded), 2)
            self.assertEqual({item.role for item in loaded}, {"control", "treatment"})
            self.assertEqual(json.loads(path.read_text(encoding="utf-8").splitlines()[0])["task_id"], "W03")


if __name__ == "__main__":
    unittest.main()
