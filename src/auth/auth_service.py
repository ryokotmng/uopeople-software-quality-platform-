"""Authentication service (draft).

Early sketch of the authentication surface. The interface is intentionally
minimal; password hashing, session storage, and token issuance will be
filled in as the design stabilizes.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional


@dataclass
class User:
    username: str
    password_hash: str


@dataclass
class AuthService:
    users: dict[str, User] = field(default_factory=dict)

    def register(self, username: str, password: str) -> User:
        if username in self.users:
            raise ValueError(f"user already exists: {username}")
        user = User(username=username, password_hash=self._hash(password))
        self.users[username] = user
        return user

    def authenticate(self, username: str, password: str) -> Optional[User]:
        user = self.users.get(username)
        if user is None:
            return None
        if user.password_hash != self._hash(password):
            return None
        return user

    @staticmethod
    def _hash(password: str) -> str:
        # Placeholder — replace with a real KDF (argon2id / bcrypt) before use.
        return f"sha-placeholder::{password}"
