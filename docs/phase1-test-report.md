# Phase 1 Test Report

## Architecture Path Verified

Phase 1 runtime path verified:

`Sources -> Kafka raw topics -> event-normalizer -> normalized.events -> SQLite -> API`

Verified raw topics:

- `raw.wikipedia.edits`
- `raw.reddit.posts`
- `raw.hn.stories`
- `raw.github.events`
- `raw.gdelt.events`

Verified normalized topic:

- `normalized.events`

Verified SQLite table:

- `normalized_events`

Verified API endpoints:

- `GET /health`
- `GET /events`
- `GET /sources`

## Scenarios Tested

### Baseline build and startup

- `go build ./...`
- `docker compose config`
- Docker image builds for all Phase 1 app services
- Full stack startup with `docker compose up -d`
- Container state check with `docker compose ps`
- Startup log inspection for:
  - `wiki-producer`
  - `reddit-producer`
  - `hn-producer`
  - `github-producer`
  - `gdelt-poller`
  - `event-normalizer`
  - `api-server`

### Kafka and topic flow

- Verified all required topics exist
- Verified raw topics receive live messages from:
  - Wikipedia
  - Reddit
  - Hacker News
  - GitHub
- Verified `normalized.events` receives normalized output
- Verified event-normalizer consumes all raw topics

### Source-specific runtime checks

#### Wikipedia producer

- Live Wikimedia SSE ingest confirmed
- Restart test confirmed reconnect after restart
- Shutdown/restart behavior reviewed

#### Reddit producer

- Live polling confirmed
- Default user-agent path confirmed
- Bad-response path checked with a poor user-agent locally
- Duplicate suppression observed indirectly via stable IDs and downstream upsert

#### Hacker News producer

- Live polling confirmed
- Story ingest confirmed
- Non-story filtering validated by manually injecting a `comment` item and confirming it was skipped

#### GitHub producer

- Live polling confirmed with no token
- Token-present path tested with a bogus token
- `401 Unauthorized` handled safely without crashing
- Rate-limit-safe handling path reviewed in code and runtime flow

#### GDELT poller

- Startup and polling confirmed
- Upstream `429 Too Many Requests` handling tested live
- Poller remained running and logged rate limiting safely

### Normalizer checks

- Consumes all five raw topics via consumer group
- Source detection by topic verified
- Normalized JSON payloads inspected from `normalized.events`
- Malformed raw payload injected manually and skipped safely
- Source-specific field mapping verified for seeded events
- Metadata preservation verified for seeded events

### SQLite checks

- Database file creation verified
- Parent directory creation path present in code
- Schema initialization verified
- Rows inserted into `normalized_events`
- Upsert behavior verified by seeding duplicate IDs
- API and normalizer sharing the same DB file verified via bind mount

### API checks

- `/health` returns `200` with DB status
- `/events` returns latest events ordered descending
- `/events?limit=...` works
- `/sources` returns grouped counts
- Empty DB behavior verified:
  - `/events` returns `[]`
  - `/sources` returns `[]`
- Invalid limit behavior verified:
  - `limit=abc` returns `400`
  - `limit=0` returns `400`
- Large limit behavior verified:
  - request succeeds and is capped server-side
- DB path failure path verified on startup with invalid `SQLITE_PATH`

### Failure and edge-case scenarios

- Kafka/topic-init startup ordering exercised repeatedly
- Service restart behavior tested for `event-normalizer` and `wiki-producer`
- Malformed raw Kafka message injected manually
- Duplicate raw Kafka message injected manually
- Empty SQLite state tested
- Missing env/default path checks:
  - API default port path (`8080`)
  - Reddit default user-agent path
  - default Kafka broker path already used by containers
- Invalid SQLite path startup failure tested

## Bugs / Issues Found

### 1. GDELT rate-limit responses were not classified consistently

Observed behavior:

- GDELT returned plain-text rate-limit responses
- Earlier code path could surface this as JSON decode failure instead of a clear rate-limit error

Impact:

- Misleading logs during upstream throttling

### 2. Wikipedia producer emitted a noisy warning on normal restart/shutdown

Observed behavior:

- On restart, wiki producer could log a warning from a canceled context while shutting down:
  - `failed to publish wikipedia event: context canceled`

Impact:

- No functional failure, but noisy and misleading logs during normal restart behavior

## Fixes Made

### Fix 1. Hardened GDELT rate-limit detection

File:

- [/Users/dhamodharans/Global-Curiosity-Engine/services/gdelt-poller/main.go](/Users/dhamodharans/Global-Curiosity-Engine/services/gdelt-poller/main.go)

Change:

- Detect `429 Too Many Requests` explicitly
- Treat plain-text rate-limit responses containing `limit requests` as rate-limited
- Avoid falling through to JSON decode for those responses

### Fix 2. Suppressed normal shutdown noise in Wikipedia producer

File:

- [/Users/dhamodharans/Global-Curiosity-Engine/services/wiki-producer/main.go](/Users/dhamodharans/Global-Curiosity-Engine/services/wiki-producer/main.go)

Change:

- Ignore `context.Canceled` when handling buffered SSE events during shutdown
- Restart logs are now clean and do not imply a publish failure on normal stop

## Results

### End-to-end seeded flow

Seeded representative raw events into:

- `raw.wikipedia.edits`
- `raw.reddit.posts`
- `raw.hn.stories`
- `raw.github.events`
- `raw.gdelt.events`

Verified:

- Normalized events published to `normalized.events`
- Rows persisted into SQLite
- API returned those rows through `/events`
- `/sources` returned grouped counts

### Duplicate handling

Seeded the same Wikipedia raw event ID twice.

Verified:

- SQLite row count for that ID remained `1`
- Stored row was updated via upsert
- Updated metadata reflected the second payload

### Malformed payload handling

Seeded `not-json` into `raw.reddit.posts`.

Verified:

- `event-normalizer` logged a decode warning
- Process stayed up
- No crash

### HN filtering

Seeded an HN item with `type=comment`.

Verified:

- No row written to SQLite for that ID

## Remaining Limitations

- GDELT free endpoint is aggressively rate-limited. The poller now handles this safely, but live ingest depends on upstream allowing requests.
- The all-at-once `docker compose build` path was unreliable in this desktop environment via Buildx. Individual/targeted service builds were reliable and the required images were successfully built.
- `api-server` still starts before `kafka-init` because it only depends on `service_started`, but this is not functionally harmful since the API only depends on SQLite, not Kafka.
- Duplicate suppression for live pollers is bounded in-memory only. This is appropriate for Phase 1 local-demo use but not durable across restarts.

## Exact Commands Used For Verification

### Build and config

```bash
go build ./...
docker compose config
docker compose build api-server event-normalizer wiki-producer reddit-producer hn-producer github-producer gdelt-poller
docker compose build gdelt-poller
docker compose build wiki-producer
```

### Full stack startup and health

```bash
docker compose up -d
docker compose ps
docker compose exec -T kafka kafka-topics --bootstrap-server kafka:29092 --list
curl http://localhost:${API_PORT:-8080}/health
```

### Topic inspection

```bash
docker compose exec -T kafka kafka-console-consumer --bootstrap-server kafka:29092 --topic raw.wikipedia.edits --from-beginning --max-messages 1
docker compose exec -T kafka kafka-console-consumer --bootstrap-server kafka:29092 --topic raw.reddit.posts --from-beginning --max-messages 1
docker compose exec -T kafka kafka-console-consumer --bootstrap-server kafka:29092 --topic raw.hn.stories --from-beginning --max-messages 1
docker compose exec -T kafka kafka-console-consumer --bootstrap-server kafka:29092 --topic raw.github.events --from-beginning --max-messages 1
docker compose exec -T kafka kafka-console-consumer --bootstrap-server kafka:29092 --topic normalized.events --from-beginning --max-messages 1
```

### Seeded end-to-end tests

```bash
docker compose exec -T kafka kafka-console-producer --bootstrap-server kafka:29092 --topic raw.wikipedia.edits
docker compose exec -T kafka kafka-console-producer --bootstrap-server kafka:29092 --topic raw.reddit.posts
docker compose exec -T kafka kafka-console-producer --bootstrap-server kafka:29092 --topic raw.hn.stories
docker compose exec -T kafka kafka-console-producer --bootstrap-server kafka:29092 --topic raw.github.events
docker compose exec -T kafka kafka-console-producer --bootstrap-server kafka:29092 --topic raw.gdelt.events
docker compose exec -T kafka kafka-console-consumer --bootstrap-server kafka:29092 --topic normalized.events --from-beginning --max-messages 5
sqlite3 database/global-curiosity-engine.db 'select count(*) from normalized_events;'
sqlite3 database/global-curiosity-engine.db 'select id, source, title from normalized_events order by source;'
```

### API verification

```bash
curl http://localhost:${API_PORT:-8080}/health
curl http://localhost:${API_PORT:-8080}/events
curl 'http://localhost:${API_PORT:-8080}/events?limit=1'
curl 'http://localhost:${API_PORT:-8080}/events?limit=abc'
curl 'http://localhost:${API_PORT:-8080}/events?limit=0'
curl 'http://localhost:${API_PORT:-8080}/events?limit=9999'
curl http://localhost:${API_PORT:-8080}/sources
```

### Restart and failure-path checks

```bash
docker restart gce-event-normalizer
docker restart gce-wiki-producer
docker compose logs --tail=25 gdelt-poller
docker compose logs --tail=20 event-normalizer
```

### Local failure-path checks

```bash
KAFKA_BROKER=localhost:9092 GITHUB_TOKEN=bogus-token go run ./services/github-producer
SQLITE_PATH=/dev/null/test.db API_PORT=18081 go run ./api/server
```

## Conclusion

Phase 1 is verified end to end for local-demo use.

Working end-to-end:

- Live source ingest for Wikipedia, Reddit, Hacker News, and GitHub
- Safe GDELT polling and rate-limit handling
- Kafka raw-topic ingestion
- Multi-topic normalization into `normalized.events`
- SQLite persistence with schema init and upsert
- SQLite-backed HTTP API for health, latest events, and source counts

Recommended next step: Phase 2

## Second Test Pass

### Scope re-tested

This second pass re-ran the full Phase 1 regression and hardening checklist after the first-round fixes, with emphasis on:

- build and Docker startup regression
- GDELT rate-limit classification
- Wikipedia restart and shutdown log behavior
- end-to-end topic flow into normalization, SQLite, and API
- deterministic seeded duplicate and malformed-payload handling
- API edge cases and DB failure-path behavior

Architecture path re-verified:

`Sources -> Kafka raw topics -> event-normalizer -> normalized.events -> SQLite -> API`

### Scenarios tested

#### Build and startup regression

- `go build ./...`
- `docker compose config`
- targeted Docker image builds for:
  - `api-server`
  - `event-normalizer`
  - `wiki-producer`
  - `reddit-producer`
  - `hn-producer`
  - `github-producer`
  - `gdelt-poller`
- full stack startup with `docker compose up -d`
- running container verification with `docker compose ps`
- topic verification with `kafka-topics --list`

#### First-round fix verification

- GDELT upstream `429` response inspected directly
- `gdelt-poller` logs verified to classify rate limiting explicitly as `gdelt rate limited`
- `wiki-producer` restart verified not to emit a misleading publish-failure warning during normal shutdown/restart

#### End-to-end regression

- live ingest confirmed again for:
  - Wikipedia
  - Reddit
  - Hacker News
  - GitHub
- normalizer logs verified to emit normalized events
- SQLite row count verified after live ingest
- API `/health` verified against populated DB

#### Deterministic seeded regression tests

- database cleared and re-tested from empty state
- seeded representative events into:
  - `raw.wikipedia.edits`
  - `raw.github.events`
- seeded malformed payload into:
  - `raw.reddit.posts`
- seeded duplicate raw messages and verified SQLite upsert behavior
- seeded a non-story Hacker News comment item and verified it was skipped
- verified normalized events appeared in:
  - Kafka topic `normalized.events`
  - SQLite table `normalized_events`
  - API `/events` response

#### API regression

- `/health`
- `/events`
- `/events?limit=1`
- `/events?limit=abc`
- `/events?limit=0`
- `/events?limit=9999`
- `/sources`
- empty DB behavior for `/events` and `/sources`

#### Failure and edge-path checks

- restart behavior for:
  - `wiki-producer`
  - `event-normalizer`
- bad GitHub token handling using local `go run`
- invalid SQLite path handling for API startup
- malformed raw payload handling in `event-normalizer`

### Results

- `go build ./...` passed after the latest fixes.
- `docker compose config` passed.
- Full stack startup succeeded.
- All required topics still existed.
- GDELT rate limiting is now classified correctly in service logs.
- Wikipedia restart no longer logs the misleading `context canceled` publish warning during normal shutdown.
- Normalizer still consumes raw topics and produces valid normalized JSON.
- SQLite still initializes, stores data, and upserts duplicate event IDs correctly.
- API still serves correct health, event, and source-count responses.
- Empty DB behavior remained safe.
- Malformed raw payloads were still skipped without crashing the normalizer.
- GitHub token failure path remained safe and non-crashing.
- Hacker News non-story filtering remained correct.

### Additional fixes made in this second pass

- Hardened [services/gdelt-poller/main.go](/Users/dhamodharans/Global-Curiosity-Engine/services/gdelt-poller/main.go) so rate-limit responses are classified both from HTTP `429` status and from plain-text rate-limit bodies.
- Hardened [services/wiki-producer/main.go](/Users/dhamodharans/Global-Curiosity-Engine/services/wiki-producer/main.go) so normal restart/shutdown does not log a misleading buffered publish failure on `context canceled`.

### Regressions found

No new architectural regressions were found after the second-pass retest.

One service-level issue was confirmed and fixed during this pass:

- GDELT rate-limit handling still needed stronger classification for plain-text `429` responses.

After the fix and retest, that issue was resolved.

### Remaining limitations

- GDELT free upstream access is rate-limited and can prevent live-event verification during short test windows.
- Duplicate suppression in producers remains in-memory only and resets on service restart.
- Docker Buildx wrapper behavior in this environment can hang even when image builds succeed; the built images themselves were usable and runtime behavior was unaffected.

### Ready state

Phase 1 remains production-like for local demo use and is ready to proceed to Phase 2.
