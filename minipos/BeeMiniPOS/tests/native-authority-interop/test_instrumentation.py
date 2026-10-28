import unittest
from instrumentation import validate_android_output

class InstrumentationEvidenceTest(unittest.TestCase):
    def test_actual_single_test_summary(self):
        validate_android_output("INSTRUMENTATION_STATUS_CODE: 1\n.\nTime: 0.2\n\nOK (1 test)\n\nINSTRUMENTATION_CODE: -1\n")
    def test_missing_zero_multiple_and_duplicate_tests_are_not_success(self):
        for text in ["", "INSTRUMENTATION_CODE: -1", "OK (0 tests)", "OK (2 tests)", "OK (1 test)\nOK (1 test)", "prefix OK (1 test)"]:
            with self.subTest(text=text), self.assertRaises(RuntimeError):
                validate_android_output(text)
    def test_failure_cannot_be_masked_by_success_summary(self):
        for marker in ["FAILURES!!!", "INSTRUMENTATION_FAILED", "INSTRUMENTATION_ABORTED", "Process crashed", "shortMsg=crash"]:
            with self.subTest(marker=marker), self.assertRaises(RuntimeError):
                validate_android_output("OK (1 test)\n"+marker)

if __name__ == "__main__": unittest.main()
