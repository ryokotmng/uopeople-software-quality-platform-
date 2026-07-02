"""Test orchestration module.

Discovers test suites under `tests/` and runs them in a predictable
order so CI output stays stable across runs. Kept intentionally thin —
this is glue over `unittest`, not a competing framework.
"""

from __future__ import annotations

import sys
import unittest
from pathlib import Path


TESTS_ROOT = Path(__file__).resolve().parent


def build_suite(pattern: str = "test_*.py") -> unittest.TestSuite:
    loader = unittest.TestLoader()
    return loader.discover(start_dir=str(TESTS_ROOT), pattern=pattern)


def run(pattern: str = "test_*.py", verbosity: int = 2) -> bool:
    suite = build_suite(pattern)
    runner = unittest.TextTestRunner(verbosity=verbosity)
    result = runner.run(suite)
    return result.wasSuccessful()


if __name__ == "__main__":
    sys.exit(0 if run() else 1)
