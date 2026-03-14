# Phase 2 Backend Test Report

Date: 2026-03-14
Branch: `phase2-curiosity-engine`

## Architecture Path Verified

The backend path validated in this pass was:

`sources -> Kafka raw topics -> event-normalizer -> normalized.events -> topic-extractor -> topics.extracted -> topic-aggregator -> topics.metrics -> curiosity-scorer -> topics.scored -> spike-detector -> topics.spikes -> SQLite -> API`

## Scenarios Tested

### Build and startup

- `go build ./...`
- `go test -run TestGDELTThrottleMessage -v ./services/gdelt-poller`
- `docker compose config`
- targeted `docker compose build` / `docker build` attempts for backend services
- `docker compose up -d` for Kafka/Zookeeper and backend services
- `docker compose ps`
- startup log inspection for:
  - `wiki-producer`
  - `reddit-producer`
  - `hn-producer`
  - `github-producer`
  - `gdelt-poller`
  - `event-normalizer`
  - `topic-extractor`
  - `topic-aggregator`
  - `curiosity-scorer`
  - `spike-detector`
  - `api-server`

### Kafka and topic flow

- Verified required topics exist:
  - `raw.wikipedia.edits`
  - `raw.reddit.posts`
  - `raw.hn.stories`
  - `raw.github.events`
  - `raw.gdelt.events`
  - `normalized.events`
  - `topics.extracted`
  - `topics.metrics`
  - `topics.scored`
  - `topics.spikes`
- Verified live source traffic reached Kafka for Wikipedia, Reddit, Hacker News, and GitHub.
- Verified GDELT handled upstream throttling safely.
- Verified processors consumed and produced across Phase 1 and Phase 2 topics.

### Deterministic seeded tests

- Seeded malformed messages into:
  - `raw.github.events`
  - `normalized.events`
  - `topics.extracted`
  - `topics.metrics`
  - `topics.scored`
- Seeded deterministic Phase 2 messages to validate:
  - extracted topics -> topic metrics
  - topic metrics -> topic scores
  - topic scores -> topic spikes
- Verified duplicate handling and upsert behavior with repeated topic events in the same minute bucket.
- Verified `initial_surge` and `score_jump` spike cases.
- Verified cooldown suppression.

### SQLite and API

- Verified SQLite DB file creation and schema initialization.
- Verified rows exist in:
  - `normalized_events`
  - `topic_metrics`
  - `topic_scores`
  - `topic_spikes`
- Verified API endpoints:
  - `GET /health`
  - `GET /events`
  - `GET /sources`
  - `GET /topics/trending`
  - `GET /topics/spikes`
  - `GET /curiosity/index`
- Verified invalid limit params return `400`.
- Verified invalid SQLite path fails cleanly.
- Verified empty-table reads are safe where practical.

### Failure and edge cases

- malformed payload injected at each major pipeline stage
- duplicate messages
- invalid API limit params
- processor restarts
- Kafka consumer-group offset resets for deterministic replay control
- GitHub bad-token path
- GDELT upstream throttling
- SQLite busy/path handling

## Results

- Backend code compiles.
- Kafka topic creation is correct.
- Live ingest is working for Wikipedia, Reddit, Hacker News, and GitHub.
- GDELT rate limiting is handled without crashing.
- Normalization, topic extraction, aggregation, scoring, spike detection, SQLite persistence, and API reads all work.
- SQLite-backed Phase 2 API endpoints return sensible JSON.

Deterministic validated outcomes included:

- `topic_metrics`
  - `github:phase6alpha:2026-03-14T10:30:00Z` with `mention_count=2`
  - `reddit:phase6alpha:2026-03-14T10:30:00Z` with `mention_count=1`
- `topic_scores`
  - `phase6alpha` rising to `3.22`
  - `phase6surge` score `9.6`
- `topic_spikes`
  - `phase6surge` emitted `initial_surge`
  - `jumptopic` emitted `score_jump`
- API
  - `/topics/trending` returned highest current scores
  - `/topics/spikes` returned recent spikes
  - `/curiosity/index` returned a non-zero derived index

## Fixes Made During This Pass

### 1. SQLite schema initialization

Problem:
- the SQLite driver in this environment did not reliably execute the full multi-statement schema in one `Exec`, so only the first table could be created.

Fix:
- [database/sqlite.go](/Users/dhamodharans/Global-Curiosity-Engine/database/sqlite.go)
  - schema initialization now splits and executes statements one by one

### 2. SQLite write contention

Problem:
- concurrent processor writes occasionally failed with `SQLITE_BUSY`.

Fix:
- [database/sqlite.go](/Users/dhamodharans/Global-Curiosity-Engine/database/sqlite.go)
  - enabled `WAL`
  - set `busy_timeout`
  - set `synchronous=NORMAL`

### 3. Kafka topic-init startup ordering

Problem:
- `kafka-init` could remain running because of command quoting, which blocked services waiting on `service_completed_successfully`.

Fix:
- [docker-compose.yml](/Users/dhamodharans/Global-Curiosity-Engine/docker-compose.yml)
  - removed the extra shell-command wrapper quotes from `kafka-init`

### 4. GDELT throttle classification

Problem:
- some upstream GDELT throttle responses were still surfacing as JSON decode errors instead of clear rate-limit warnings.

Fix:
- [services/gdelt-poller/main.go](/Users/dhamodharans/Global-Curiosity-Engine/services/gdelt-poller/main.go)
  - classify throttle responses before JSON decoding
  - treat `429`, `Too Many Requests`, `Please limit requests`, and `rate limit` text as upstream throttling
- [services/gdelt-poller/main_test.go](/Users/dhamodharans/Global-Curiosity-Engine/services/gdelt-poller/main_test.go)
  - added a focused regression test for throttle detection

## Remaining Limitations

- GDELT free access is rate-limited and can block live-ingest verification windows.
- Duplicate suppression in producers and processors is in-memory only and resets on restart.
- Docker image rebuild/recreate was flaky in this environment because the Docker build wrapper occasionally hung after producing images. Updated code paths were therefore verified both through Docker and direct `go run` execution against the same Kafka/SQLite stack.
- Full deterministic raw-topic-to-spike replay in one clean shot is sensitive to existing Kafka consumer-group offsets and background live traffic. Stage-by-stage deterministic validation passed.

## Recommended Next Step

Phase 3 UI.
