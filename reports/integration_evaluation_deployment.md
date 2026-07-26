# Part A: System Integration & Feature Completion

The system was integrated as a Go-based Software Quality Improvement Platform that combines the user interface, backend services, and reporting logic into a single functioning application. The backend is implemented with Go's standard `net/http` package so the project stays lightweight, portable, and easy to deploy, consistent with software engineering principles that emphasize modularity, traceability, and maintainability (Summers, 2020). This also keeps the capstone scope practical by showing modules communicate via HTTP endpoints without an added framework.

The integrated system rests on three connected capabilities: authentication, report generation, and HTTP routing. The authentication module receives a username and password and checks them against a local user store; passwords are never compared as plain text but as salted PBKDF2-HMAC-SHA256 hashes, and a failed login returns the same message whether the username was unknown or the password was wrong. The reporting module collects test outcomes and calculates the total, passed, and failed counts and the pass rate. The HTTP layer exposes both through `/login`, which serves an HTML form to browsers (`GET /login`) and accepts a form or JSON POST, and `/report`, which serializes the report function's output as JSON. Both entry points route to the same core function regardless of caller, so the logic is never duplicated between the human-facing and machine-facing paths. This shows clear inter-module communication: the handler layer depends on the core logic layer, while the core logic stays independent of presentation, whether that presentation is an HTML page or a JSON document (Summers, 2020).

Figure 1
Go source code showing the authentication function and the `/login` HTTP handler

Figure 2
Browser screenshots of the login page showing a successful login and a failed login

Figure 3
Go source code showing the report generation function and the `/report` HTTP handler that calls it

Figure 4
Output screenshot showing JSON report results from `/report`

Figure 5
Go source code showing the dashboard handler reading from the run-result data layer

A first example of inter-module communication is the login handler calling the authentication function, then rendering an HTML page or a JSON body depending on how the request arrived (Figure 1). A second is the report endpoint calling the report-generation function and serializing its output with no extra logic in the handler (Figure 3). A third sits behind the dashboard (Figure 5): the test-orchestration service writes run results into a small file-based data layer, and the dashboard handler reads from that same layer to render run history, in place of a conventional database and swappable for one later. The main implementation challenge was balancing clarity and scope: a production-grade authentication service with external identity providers would have added unnecessary complexity, so a local, salted-hash user store was used instead. A second challenge was serving a login page and an API from one endpoint without duplicating logic; this was resolved by branching on `Content-Type` while sharing one authentication function underneath both paths.

# Part B: System Evaluation & Metrics Reporting

The system was evaluated with quantitative and qualitative indicators. Quantitative metrics provide numerical evidence of performance, while qualitative metrics describe user experience and clarity (LaunchNotes, 2023). The quantitative metrics chosen were latency and throughput, and the qualitative metric was interface clarity and usability — appropriate because the system is small enough to measure directly yet meaningful enough to demonstrate real performance evaluation (Wells, 2025; GeeksforGeeks, 2025).

Latency and throughput were measured with a small command-line load-testing tool built for this project, which sends real HTTP requests to a running instance rather than calling handlers in-process. It runs two phases: a latency phase timing sequential requests to an endpoint and reporting min, mean, p50, and p95 response times, and a throughput phase running several concurrent workers for a fixed duration and reporting completed requests and req/s. Because both phases go over an actual HTTP connection, results reflect the full request path, not an isolated function call.

Measured against a local instance, `/report` had a median (p50) latency of about 52 microseconds and sustained roughly 17,169 requests per second under concurrent load. `/login` had a median latency of about 19.3 milliseconds and sustained roughly 313 requests per second. This gap is intentional: `/login` performs 210,000 rounds of PBKDF2 hashing on every request to resist offline password-guessing attacks, while `/report` only summarizes data already in memory. The two metrics move together for one reason — `/login`'s ~371-times higher latency directly produces its ~55-times lower throughput — illustrating a security/performance tradeoff rather than an inefficiency.

Figure 6
Terminal output from the load-testing tool showing `/login` and `/report` latency

Figure 7
Terminal output from the load-testing tool showing `/login` and `/report` throughput

Figure 8
Screenshot and notes from a usability review of the login page and dashboard

Figure 9
Bar chart comparing latency and throughput across the two endpoints, on a log scale

The qualitative assessment covered the login page's clarity, the dashboard's readability, and the JSON report's interpretability. The login page presents one labeled form with a single action and an unambiguous success or error banner, so a first-time user needs no extra explanation. The dashboard surfaces the latest run's pass/fail status, counts, and any failing-test output directly, and the JSON report's field names are self-describing. A capstone interface does not need commercial polish; it only needs to be clear enough to demonstrate purpose and behavior.

Results were summarized in a structured table listing each metric, its method, the observed value, and an interpretation, supported by a bar chart (Figure 9) comparing both endpoints across latency and throughput on a shared log scale. The chart makes the tradeoff visible: the endpoint with the shorter latency bar has the taller throughput bar, and vice versa, consistent with evaluating systems on functional, technical, and user-facing criteria together rather than code correctness alone (Wells, 2025; GeeksforGeeks, 2025). As a secondary check, the same handlers were measured in-process with Go's built-in benchmarking tool, isolating compute cost from network overhead; those numbers agreed on which endpoint is more expensive, increasing confidence the gap reflects PBKDF2 cost rather than measurement noise. Overall, the load-test evidence and usability review together provide enough support for the project goals (Summers, 2020).

# Part C: Deployment & Configuration Planning Overview

The most appropriate deployment method for this system is a Docker-based container environment, supported by GitHub Actions for build and test automation. Docker packages the Go application into a repeatable runtime environment, reducing environment drift and improving reliability. A multi-stage Dockerfile compiles a single static Go binary into a minimal Alpine image running as a non-root user, producing a ~31 MB image — practical for a capstone project since it is easy to reproduce and document (Sumo Logic, n.d.), and aligned with configuration management principles since the environment can be versioned and monitored consistently (Bennett, 2024).

Configuration management contributes to reliability by ensuring the same software version, environment variables, and build steps are used across development, testing, and deployment, reducing errors from mismatched dependencies and supporting predictable, documented delivery (Bennett, 2024; Sumo Logic, n.d.). Configuration details are stored in a `.env` file, with a committed `.env.example` documenting every variable, and secrets are kept out of source control by git-ignoring `.env` itself. The listen port can be set through `PORT`, which most container and PaaS platforms inject automatically, or `ADDR` for a full address; `PORT` takes precedence when both are present, so the same image runs unmodified locally or inside an orchestrator.

Setup requirements include an operating system such as Linux, macOS, or Windows; Go 1.24 or later, since authentication depends on the `crypto/pbkdf2` package introduced in that release (the module targets Go 1.26); Git; and Docker if container deployment is used. Environment variables include `PORT` or `ADDR`, `DATA_DIR` for stored accounts and test-run results, `REPO_DIR` for the repository the orchestrator runs against, `ADMIN_USERNAME` and `ADMIN_PASSWORD` for the seeded account, and a reserved `DATABASE_URL` for a future shared PostgreSQL store. Installation is straightforward: install Go, clone the repository, build with `go build ./...`, and test with `go test ./...`; since `go.mod` is already initialized, no `go mod init` step is needed. For Docker, the steps are `docker build -t software-quality-platform .` and `docker run --rm -p 8080:8080 -e ADMIN_USERNAME=... -e ADMIN_PASSWORD=... software-quality-platform`. GitHub Actions automates these on every push and pull request: one job builds, vets, and tests the application, and a second job, gated on the first succeeding, builds the Docker image to confirm the container itself stays buildable.

Figure 10
Dockerfile showing the multi-stage build

Figure 11
Terminal output showing `go build` and `go test` execution

Figure 12
GitHub Actions workflow run showing the test job and the Docker build job passing

Docker was selected because it simplifies portability and makes the demonstration more reliable than the alternatives: on-premise deployment would require more manual setup, and a cloud-only deployment would add unnecessary complexity for this assignment. Docker bridges development and deployment while reflecting a realistic professional workflow, and GitHub Actions automates build, test, and image validation on every push. That relationship between configuration management and deployment matters because it ensures the project can be reproduced consistently and monitored across milestones (Bennett, 2024).
