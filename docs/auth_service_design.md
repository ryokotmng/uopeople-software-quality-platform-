# Authentication Service — Design Draft

## Purpose
Provide user registration and credential-based authentication for the
Software Quality Platform. This draft covers only the local username /
password flow; SSO and MFA are out of scope for the initial iteration.

## Responsibilities
- Register new users with a unique username.
- Verify credentials on login.
- Store password hashes rather than raw passwords.
- Expose a stable, testable interface for other services.

## Non-Goals (for the draft)
- Persistent storage (currently in-memory).
- Real password hashing (placeholder KDF in the draft).
- Session/token issuance — will be layered on later.

## Open Questions
- Which KDF: argon2id vs. bcrypt?
- Where do sessions live: signed cookie vs. server-side store?
- Rate limiting and lockout policy on failed attempts.
