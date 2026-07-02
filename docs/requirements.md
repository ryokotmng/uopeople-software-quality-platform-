# Requirements

Working list of functional and non-functional requirements for the
Software Quality Platform. Intentionally short — items are added or
refined as design work progresses.

## Functional Requirements (FR)

| ID    | Requirement                                                         |
|-------|---------------------------------------------------------------------|
| FR-1  | Users can register with a unique username and password.             |
| FR-2  | Users can authenticate with their username and password.            |
| FR-3  | The platform can discover test suites under `tests/` automatically. |
| FR-4  | The platform can run all discovered suites in a stable order.       |
| FR-5  | The platform records the pass/fail outcome of each test run.        |
| FR-6  | Only authenticated users can trigger test runs.                     |
| FR-7  | Users can view progress reports produced by previous runs.          |

## Non-Functional Requirements (NFR)

| ID     | Category        | Requirement                                                                 |
|--------|-----------------|-----------------------------------------------------------------------------|
| NFR-1  | Security        | Passwords are stored only as hashes produced by a modern KDF.               |
| NFR-2  | Security        | Failed authentication does not reveal whether the username exists.          |
| NFR-3  | Reliability     | A failure in one test suite must not prevent others from running.           |
| NFR-4  | Maintainability | Source, tests, docs, diagrams, and reports live in separate top-level dirs. |
| NFR-5  | Traceability    | Every change reaches `main` through a reviewed commit on a feature branch. |
| NFR-6  | Portability     | The platform runs on any environment with Python 3.11 or later.             |
| NFR-7  | Usability       | Test orchestrator output is stable enough to diff between runs.             |

## Change Log

- Initial draft — FR-1..7 and NFR-1..7 seeded from the project writeup.
