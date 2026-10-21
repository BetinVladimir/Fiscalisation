"""Fail closed on Android instrumentation summaries, including zero tests."""
import re

def validate_android_output(output):
    if any(marker in output for marker in ("FAILURES!!!", "INSTRUMENTATION_FAILED", "INSTRUMENTATION_ABORTED", "Process crashed", "shortMsg=")):
        raise RuntimeError("ANDROID_INSTRUMENTATION_FAILED")
    summaries = re.findall(r"^OK \(\s*(\d+) tests?\s*\)$", output, re.MULTILINE)
    if summaries != ["1"]:
        raise RuntimeError("ANDROID_EXPECTED_EXACTLY_ONE_TEST")
