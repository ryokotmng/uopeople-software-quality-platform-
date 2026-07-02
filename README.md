# Software Quality Platform

Course project repository for the UoPeople Software Engineering program.
This repository tracks the source code, design notes, diagrams, tests, and
progress reports for the Software Quality Platform prototype.

## Repository Structure

| Folder      | Purpose                                        |
|-------------|------------------------------------------------|
| `/src`      | Source code for the platform's modules         |
| `/docs`     | Technical documentation and design notes       |
| `/diagrams` | Architecture and design diagrams               |
| `/tests`    | Automated test scripts                         |
| `/reports`  | Progress summaries and final writeups          |

## Branching Model

- `main` — stable, approved version.
- `development` — active branch for ongoing changes.
- `feature/*` — short-lived branches for individual pieces of work
  (e.g. `feature/testing-module`, `feature/auth-service`).

All changes flow through `development` and are merged into `main` only
after review, so the mainline stays stable and every update is
traceable in the commit history.
