"""Smoke tests for the AuthService draft."""

from __future__ import annotations

import unittest

from src.auth.auth_service import AuthService


class AuthServiceTests(unittest.TestCase):
    def test_register_then_authenticate_succeeds(self) -> None:
        svc = AuthService()
        svc.register("alice", "correct-horse")
        self.assertIsNotNone(svc.authenticate("alice", "correct-horse"))

    def test_authenticate_with_wrong_password_returns_none(self) -> None:
        svc = AuthService()
        svc.register("alice", "correct-horse")
        self.assertIsNone(svc.authenticate("alice", "battery-staple"))

    def test_register_duplicate_username_raises(self) -> None:
        svc = AuthService()
        svc.register("alice", "correct-horse")
        with self.assertRaises(ValueError):
            svc.register("alice", "another")


if __name__ == "__main__":
    unittest.main()
