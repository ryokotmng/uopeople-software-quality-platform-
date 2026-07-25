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

## Initial implementation
- **Hashing:** PBKDF2-HMAC-SHA256 with a per-user random salt and a high
  iteration count, using the standard-library `crypto/pbkdf2` package
  (NFR-1). Hashes are compared in constant time (`crypto/subtle`).
- **Enumeration resistance:** an unknown username fails with the same
  error and comparable timing as a wrong password, so the response never
  reveals whether an account exists (NFR-2).
- **Persistence:** users are stored as JSON on disk (atomic
  write-then-rename). The interface is unchanged if this is later moved
  to a database.

## Non-Goals (for the draft)
- Session/token issuance — the initial UI uses HTTP Basic auth for the
  one protected endpoint; signed sessions will be layered on later.
- SSO and MFA.
- Rate limiting and account lockout on repeated failed attempts.

## Open Questions
- Where do sessions live once added: signed cookie vs. server-side store?
- Lockout / rate-limiting policy on failed attempts.
- Migrating the user store to the shared PostgreSQL metrics database.
