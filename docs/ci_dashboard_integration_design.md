# Design: Closing the CI → Dashboard Loop

Status: **Design only** (not yet implemented). Companion to
`diagrams/architecture.md` and `docs/module_design.md`.

## 1. Goal

Today two halves work independently:

- The **dashboard** (`internal/web`) shows results of runs triggered
  locally via `POST /api/runs/trigger`.
- **GitHub Actions** (`.github/workflows/ci.yml`) runs the same test
  suites on every push, but its results never reach the dashboard.

The goal is to **close the loop**: when code is pushed, the run executed
by GitHub Actions appears in the dashboard automatically, with its
status, error logs, and the commit/branch it came from — delivering the
"unified view of software quality" the product promises.

Non-goals for this iteration: real-time streaming of in-progress runs,
coverage trends, and multi-repository aggregation.

## 2. Current state (recap)

```
push ──▶ GitHub Actions ──▶ go test (pass/fail gate)      [results discarded]
local operator ──▶ POST /api/runs/trigger ──▶ orchestration.Run ──▶ store ──▶ dashboard
```

Relevant existing pieces this design reuses:

- `orchestration.RunResult` — the structured result type.
- `orchestration.Run(ctx, dir, pkgs...)` — runs `go test -json`, parses it.
- `store.RunStore.Save/List` — the data layer.
- `web` routes: `GET /`, `GET /api/runs`, `POST /api/runs/trigger`.

## 3. Options considered

| # | Approach | How CI delivers results | Pros | Cons | Verdict |
|---|----------|-------------------------|------|------|---------|
| 1 | **Push to an ingestion endpoint** | CI `POST`s the result JSON to a deployed dashboard | Reuses the store; small new surface; results are live | Dashboard must be reachable from CI; needs a CI credential | **Chosen** |
| 2 | Artifact + pull | CI uploads JSON as a workflow artifact; dashboard polls the GitHub API | Dashboard need not be publicly reachable | Polling, GitHub API auth, more moving parts | Deferred |
| 3 | Commit to repo / GitHub Pages | CI writes JSON to a branch or publishes a static page | No server needed | No auth, no dynamic API, noisy git history | Rejected |
| 4 | `workflow_run` webhook | GitHub calls the dashboard, which fetches results | Event-driven, no polling | Needs a public endpoint + webhook secret + GitHub API fetch | Deferred |

Option 1 fits the existing architecture with the least new machinery and
keeps the data layer as the single source of truth. Options 2/4 become
attractive later if the dashboard cannot be exposed to CI.

## 4. Target flow (Option 1)

```
push
 └─▶ GitHub Actions
       ├─ orchestrate: go test -json ./...  ──▶ result.json   (RunResult + provenance)
       │        └─ exit non-zero if any test failed  (quality gate stays in CI)
       └─ ingest: POST /api/runs  (Bearer <INGEST_TOKEN>, body = result.json)
                    └─▶ web ──▶ store.Save ──▶ dashboard shows the CI run
```

Two new steps in CI: **orchestrate** (produce the structured result) and
**ingest** (send it). The existing pass/fail gate is preserved because
the orchestrate step exits non-zero on failure.

## 5. Data model changes

Extend `orchestration.RunResult` with optional provenance so CI runs are
distinguishable from local ones and link back to their source. All
fields are `omitempty`, so existing stored JSON stays valid (backward
compatible).

```go
type RunResult struct {
    // ... existing fields ...

    Source      string `json:"source,omitempty"`       // "ci" | "local"
    Commit      string `json:"commit,omitempty"`        // full SHA
    Branch      string `json:"branch,omitempty"`        // e.g. "development"
    RunURL      string `json:"run_url,omitempty"`       // link to the Actions run
    TriggeredBy string `json:"triggered_by,omitempty"`  // actor / "local"
}
```

Dashboard changes: show a `CI` / `local` badge per run, and render
`Commit` (short SHA) linking to `RunURL`.

## 6. API contract: `POST /api/runs`

New authenticated ingestion endpoint (distinct from `/trigger`, which
*runs* tests; this one *records* an already-run result).

- **Request:** `Content-Type: application/json`, body is a `RunResult`.
- **Auth:** `Authorization: Bearer <INGEST_TOKEN>` (see §7).
- **Validation:** reject if `started_at` is zero, if counts are negative,
  or if `total != passed + failed + skipped`. Cap body size (e.g. 1 MiB)
  and truncate per-failure `output` (e.g. 8 KiB) to bound storage.
- **Responses:**
  - `201 Created` — stored (returns the canonical stored record).
  - `200 OK` — duplicate ignored (idempotent; see §8).
  - `400 Bad Request` — malformed / failed validation.
  - `401 Unauthorized` — missing/invalid token.
  - `413 Payload Too Large` — body over the cap.

```
POST /api/runs
Authorization: Bearer ci-xxxxxxxx
Content-Type: application/json

{ "started_at":"2026-07-12T10:15:00Z", "total":10, "passed":8, "failed":1,
  "skipped":1, "failures":[...], "source":"ci", "commit":"abc123...",
  "branch":"development", "run_url":"https://github.com/.../actions/runs/42" }
```

Handler sketch (reusing the store):

```go
func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
    // requireToken middleware has already run
    var res orchestration.RunResult
    if err := decodeLimited(r.Body, &res, maxBody); err != nil { badRequest(w, err); return }
    if err := validate(res); err != nil { badRequest(w, err); return }
    if s.runs.Exists(dedupKey(res)) { writeJSON(w, 200, res); return }
    if err := s.runs.Save(res); err != nil { serverError(w, err); return }
    writeJSON(w, 201, res)
}
```

## 7. Authentication for CI

Human Basic-auth credentials must **not** be reused by automation.
Introduce a dedicated ingestion token:

- Config: `INGEST_TOKEN` (env / `.env`, added to `config.Config`).
- Middleware `requireToken`: constant-time compare (`crypto/subtle`) of
  the `Bearer` value against `INGEST_TOKEN`; `401` on mismatch. If
  `INGEST_TOKEN` is empty, the endpoint is disabled (returns `404`/`403`)
  so it is never open by default.
- In GitHub, store it as the repository secret `INGEST_TOKEN` and pass it
  to the ingest step. Fork PRs do not receive secrets, so untrusted forks
  cannot post — an intentional safety property.
- Rotation: changing the secret + server env is sufficient; tokens carry
  no state.

`/trigger` keeps Basic auth (a human action); `/api/runs` uses the token
(a machine action). Separating them keeps each credential scoped to one
caller.

## 8. Idempotency & ordering

A workflow can be re-run, delivering the same result twice. Define a
**dedup key** = `commit + "|" + started_at` (fall back to `started_at`
alone for local runs). `RunStore` gains `Exists(key)` (or `Save` becomes
insert-if-absent). Re-posting returns `200` without creating a duplicate.

Ordering is unchanged: `List()` already sorts by `StartedAt` descending,
so CI and local runs interleave correctly by time.

## 9. CI workflow changes

Replace the plain `go test` step with an **orchestrate** step that emits
JSON and preserves the gate, then add a best-effort **ingest** step.

```yaml
      - name: Orchestrate tests
        run: go run ./cmd/orchestrate -out result.json   # exits non-zero on failure

      - name: Upload result (artifact, always)
        if: always()
        uses: actions/upload-artifact@v4
        with: { name: test-result, path: result.json }

      - name: Ingest into dashboard (best-effort, push only)
        if: always() && github.event_name == 'push'
        env:
          INGEST_TOKEN: ${{ secrets.INGEST_TOKEN }}
          DASHBOARD_URL: ${{ vars.DASHBOARD_URL }}
        run: |
          curl -fsS -X POST "$DASHBOARD_URL/api/runs" \
            -H "Authorization: Bearer $INGEST_TOKEN" \
            -H "Content-Type: application/json" \
            --data-binary @result.json || echo "dashboard unreachable; artifact retained"
```

Design decisions:

- **Gate stays in CI:** the orchestrate step's non-zero exit fails the
  build; ingestion never affects the gate (`|| echo ...` swallows POST
  errors). Reporting availability must not block merging.
- **Artifact as fallback:** the result is always uploaded, so it survives
  even when the dashboard is down (and supports Option 2 later).
- **Push only:** avoids double-counting PR + push and keeps fork PRs out.

A new small command `cmd/orchestrate` wraps `orchestration.Run`, stamps
provenance from `GITHUB_*` env vars, writes `-out`, and mirrors the
pass/fail exit code.

## 10. Reachability / deployment assumption

Option 1 requires GitHub Actions to reach the dashboard at
`DASHBOARD_URL`. For a small team this is one of:

- a small always-on deployment (the single Go binary + a persistent
  volume for `DATA_DIR`), or
- a **self-hosted runner** on the team network posting to an internal URL.

If neither is acceptable, switch to Option 2 (the artifact is already
produced, so only a poller is added). This is documented as the fallback
rather than built now.

## 11. Security summary

- Dedicated, rotatable token; disabled unless configured; constant-time
  compare; fork PRs cannot authenticate.
- Body-size cap and per-failure output truncation bound storage and
  mitigate abuse.
- Only quality *reporting* crosses the trust boundary; the quality
  *gate* remains inside CI.
- HTTPS assumed at `DASHBOARD_URL` so the token is not sent in clear.

## 12. Incremental implementation plan

1. Add provenance fields to `RunResult` (`omitempty`) + dashboard badges.
2. Add `RunStore.Exists` / insert-if-absent (dedup).
3. Add `INGEST_TOKEN` to config + `requireToken` middleware.
4. Add `POST /api/runs` handler (validation, size cap, idempotency).
5. Add `cmd/orchestrate` (run → stamp provenance → write JSON → exit code).
6. Update `ci.yml` (orchestrate + upload + ingest steps).
7. Tests: validation, auth (missing/wrong/disabled token), idempotent
   re-post, provenance round-trip through the store.

Steps 1–4 are self-contained and demoable without any deployment (post
with `curl` locally). Steps 5–6 close the loop end-to-end once a
`DASHBOARD_URL` exists.

## 13. Future work

- Option 2 poller / Option 4 webhook if the dashboard can't be exposed.
- Coverage and trend metrics (Metrics & Reporting module).
- Per-repository and per-branch views once multi-repo is in scope.
```
