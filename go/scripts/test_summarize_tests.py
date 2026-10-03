import json
import unittest

from summarize_tests import summarize


def events(*entries):
    return [json.dumps(entry) for entry in entries]


class TestSummarizeTests(unittest.TestCase):
    def test_counts_include_subtests_and_optional_database_skips(self):
        report = summarize(events(
            {"Action": "start", "Package": "example"},
            {"Action": "run", "Package": "example", "Test": "TestParent"},
            {"Action": "pass", "Package": "example", "Test": "TestParent/child"},
            {"Action": "pass", "Package": "example", "Test": "TestParent"},
            {"Action": "skip", "Package": "example", "Test": "TestDatabase"},
            {"Action": "pass", "Package": "example"},
            {"Action": "skip", "Package": "no_tests"},
        ))
        self.assertEqual(report["test_nodes"], {"pass": 2, "fail": 0, "skip": 1})
        self.assertEqual(report["packages"], {"pass": 1, "fail": 0, "skip": 1})
        self.assertEqual(report["skipped_tests"], ["example/TestDatabase"])
        self.assertEqual(report["unfinished_tests"], [])
        self.assertEqual(report["unfinished_packages"], [])

    def test_failed_build_is_reported_without_test_events(self):
        report = summarize(events(
            {"Action": "start", "Package": "broken"},
            {"Action": "fail", "Package": "broken"},
        ))
        self.assertEqual(report["failures"], ["broken"])
        self.assertEqual(report["unfinished_packages"], [])

    def test_interrupted_test_and_package_remain_unfinished(self):
        report = summarize(events(
            {"Action": "start", "Package": "example"},
            {"Action": "run", "Package": "example", "Test": "TestBlocked"},
        ))
        self.assertEqual(report["unfinished_tests"], ["example/TestBlocked"])
        self.assertEqual(report["unfinished_packages"], ["example"])

    def test_bad_json_is_rejected(self):
        with self.assertRaises(ValueError):
            summarize(["not test json"])

    def test_empty_log_has_no_success(self):
        report = summarize([])
        self.assertEqual(report["events"], 0)
        self.assertEqual(report["packages"]["pass"], 0)


if __name__ == "__main__":
    unittest.main()
