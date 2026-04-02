# UI Hardening Report

## Branch
- `codex/phase3-ui-dashboard`

## Architecture Path Verified
- Vite/React frontend running against the existing backend API proxy
- `/api/health`
- `/api/events`
- `/api/topics/spikes`
- `/api/topics/trending`
- `/api/sources`
- `/api/curiosity/index`
- Dashboard flow verified:
  - header status and index
  - globe hero
  - region drill-down
  - source lens switching
  - live feed
  - spike alerts
  - trend chart

## Scenarios Tested

### Startup and Baseline
- `npm install`
- `npm run build`
- `npm run dev -- --host 127.0.0.1 --port 4173`
- frontend loaded successfully against the live backend
- baseline desktop screenshot captured

### Globe Interaction
- default render in headful Chrome
- hover and pointer inspection behavior
- drag interaction behavior
- click-to-focus region behavior
- region switch via hotspot list
- non-WebGL fallback behavior in headless Chrome

### Source Switching
- `overview`
- `wikipedia`
- `reddit`
- `hackernews`
- `github`
- `gdelt`
- verified source state propagation into:
  - region panel
  - live feed
  - spike alerts
  - trend chart
- verified empty-state behavior for low-signal source views

### Region Drill-down
- switched selected region from North America to Europe
- verified selected region context updates across the panel stack

### Data States
- loading state
- backend unavailable before first load
- backend unavailable after initial snapshot and background refresh
- stale snapshot retention after refresh failure

### Responsiveness
- verified at widths:
  - `1280`
  - `1440`
  - `1720`
- no horizontal overflow detected in those desktop layouts

## Bugs / Issues Found
1. Source lenses with no signal could not remain selected because the app forced them back to `overview`.
2. The live feed issue was part of a broader shared-state problem affecting source-aware consistency across multiple panels.
3. The globe auto-rotated continuously, including during inspection, which made hover/inspection feel unstable.
4. The globe had no graceful fallback when WebGL failed, causing the main hero to collapse in unsupported environments.
5. The offline-first-load header state could misleadingly show a healthy detail while the app was unavailable.
6. The app emitted a favicon 404 on startup.

## Fixes Applied
1. Removed the automatic fallback-to-overview effect in `App.tsx` so low-signal tabs can stay selected and show meaningful empty states.
2. Kept source lens propagation explicit into:
   - `RegionPanel`
   - `LiveFeedPanel`
   - `SpikeAlertsPanel`
   - `TrendChartPanel`
   and added lightweight panel attributes for direct verification.
3. Updated `GlobeHero.tsx` to pause auto-rotation during inspection:
   - pointer over hero
   - point hover
   - drag interaction
   - short post-click inspection lock
4. Improved globe readability and signal encoding:
   - switched to a brighter earth texture
   - tightened ring targets to prioritized hotspots
   - strengthened selected/inspection state messaging
5. Added a `GlobeRenderBoundary` and fallback region map so WebGL failure degrades cleanly instead of blanking the page.
6. Corrected header status messaging for offline/loading states.
7. Added a favicon to remove the startup 404.

## Retest Results
- Build passed after fixes.
- Source switching stayed in sync across region panel, live feed, spike alerts, and trend chart.
- Empty-state source tabs now remain selected and show appropriate quiet-state messaging.
- Region switching updated the dashboard context consistently.
- Globe status changed from `AUTOPILOT SWEEP` to `INSPECTION HOLD` during hover/interaction.
- Offline first-load state rendered the designed standby view.
- Refresh failure after initial snapshot preserved the existing dashboard and showed the recovery warning.
- Headless non-WebGL runs now render the fallback globe surface instead of crashing the page.
- Desktop responsiveness checks passed at `1280`, `1440`, and `1720` widths without overflow.

## Remaining Limitations / Risks
- `react-globe.gl` still emits upstream Three.js deprecation warnings from its dependency chain.
- The globe bundle remains large; this is acceptable for now but still worth a later performance pass.
- WebGL initialization errors still appear in the browser console before the error boundary catches them. The UI now recovers correctly, but the underlying library still logs the failure.

## Commands Used
```bash
cd /Users/dhamodharans/Global-Curiosity-Engine/frontend
npm install
npm run build
npm run dev -- --host 127.0.0.1 --port 4173
```

```bash
curl -s http://127.0.0.1:8080/health
docker compose ps
docker compose stop api-server
docker compose up -d api-server
```

Browser automation was run with local Chrome via `puppeteer-core` against `http://127.0.0.1:4173` to verify:
- render stability
- source switching
- region switching
- globe inspection state
- offline state
- stale snapshot refresh behavior
- responsive layout widths

## Final Assessment
- The Phase 3 UI is now stable and demo-ready for the current dashboard scope.
- The main remaining concern is bundle size, not correctness or interaction stability.
