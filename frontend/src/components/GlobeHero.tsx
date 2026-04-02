import { Component, type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { Crosshair, Sparkles } from "lucide-react";
import Globe, { type GlobeMethods } from "react-globe.gl";
import { PanelState } from "./PanelState";
import type { RegionInsight } from "../types";
import { formatCompactNumber, formatScore, toTitleCase } from "../lib/format";

const GLOBE_TEXTURE =
  "https://unpkg.com/three-globe/example/img/earth-blue-marble.jpg";
const BUMP_TEXTURE =
  "https://unpkg.com/three-globe/example/img/earth-topology.png";
const INSPECTION_LOCK_MS = 5000;

type GlobeHeroProps = {
  regions: RegionInsight[];
  selectedRegionID: string | null;
  onSelectRegion: (regionID: string) => void;
  isLoading?: boolean;
};

type GlobePoint = {
  id: string;
  lat: number;
  lng: number;
  radius: number;
  altitude: number;
  hitColor: string;
  score: number;
  label: string;
  summary: string;
};

type GlowCluster = {
  id: string;
  regionID: string;
  lat: number;
  lng: number;
  regionLabel: string;
  activity: number;
  brightness: number;
  auraSize: number;
  coreSize: number;
  haloSize: number;
  falloff: number;
  color: string;
  selected: boolean;
  hovered: boolean;
};

type GlobeControls = {
  autoRotate: boolean;
  autoRotateSpeed: number;
  enablePan: boolean;
  minDistance: number;
  maxDistance: number;
  addEventListener?: (eventName: "start" | "end", listener: () => void) => void;
};

export function GlobeHero({ regions, selectedRegionID, onSelectRegion, isLoading = false }: GlobeHeroProps) {
  const globeRef = useRef<GlobeMethods | undefined>(undefined);
  const [hoverRegionID, setHoverRegionID] = useState<string | null>(null);
  const [isInteracting, setIsInteracting] = useState(false);
  const [inspectionLock, setInspectionLock] = useState(false);

  const points: GlobePoint[] = useMemo(
    () =>
      regions.map((item) => ({
        id: item.region.id,
        lat: item.region.lat,
        lng: item.region.lng,
        radius:
          1.24 +
          Math.min(item.activity / 420, 0.85) +
          (item.region.id === selectedRegionID ? 0.28 : 0) +
          (item.region.id === hoverRegionID ? 0.16 : 0),
        altitude: 0.004,
        hitColor: "rgba(255,255,255,0.015)",
        score: item.score,
        label: item.region.label,
        summary: item.narrative,
      })),
    [hoverRegionID, regions, selectedRegionID],
  );
  const glowClusters = useMemo(() => buildGlowClusters(regions, selectedRegionID, hoverRegionID), [hoverRegionID, regions, selectedRegionID]);
  const ringTargets = useMemo(() => {
    const priorityIDs = new Set<string>();
    if (selectedRegionID) {
      priorityIDs.add(selectedRegionID);
    }
    if (hoverRegionID) {
      priorityIDs.add(hoverRegionID);
    }
    if (priorityIDs.size === 0) {
      return [];
    }
    return points.filter((point) => priorityIDs.has(point.id));
  }, [hoverRegionID, points, regions, selectedRegionID]);
  const ringRepeatPeriod = selectedRegionID ? 2050 : hoverRegionID ? 1850 : 0;

  const focusRegion = regions.find((item) => item.region.id === (hoverRegionID ?? selectedRegionID)) ?? regions[0];
  const selectedRank = focusRegion?.rank ?? regions.findIndex((item) => item.region.id === focusRegion?.region.id) + 1;
  const isInspectionMode = Boolean(hoverRegionID) || inspectionLock || isInteracting;

  useEffect(() => {
    const globe = globeRef.current;
    if (!globe) {
      return;
    }

    const controls = globe.controls() as GlobeControls;
    controls.autoRotateSpeed = 0.32;
    controls.enablePan = false;
    controls.minDistance = 180;
    controls.maxDistance = 320;
    controls.addEventListener?.("start", () => setIsInteracting(true));
    controls.addEventListener?.("end", () => setIsInteracting(false));
  }, []);

  useEffect(() => {
    const globe = globeRef.current;
    if (!globe) {
      return;
    }

    const controls = globe.controls() as GlobeControls;
    controls.autoRotate = !isInspectionMode;
  }, [isInspectionMode]);

  useEffect(() => {
    if (!inspectionLock) {
      return;
    }

    const timeoutID = window.setTimeout(() => {
      setInspectionLock(false);
    }, INSPECTION_LOCK_MS);

    return () => {
      window.clearTimeout(timeoutID);
    };
  }, [inspectionLock]);

  useEffect(() => {
    const selected = regions.find((item) => item.region.id === selectedRegionID);
    if (!selected || !globeRef.current) {
      return;
    }

    globeRef.current.pointOfView(
      {
        lat: selected.region.lat,
        lng: selected.region.lng,
        altitude: 1.8,
      },
      1350,
    );
  }, [regions, selectedRegionID]);

  if (isLoading && regions.length === 0) {
    return (
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_300px]">
        <div className="surface-sheen relative min-h-[500px] rounded-[32px] border border-cyan/10 bg-[#050915] shadow-glow">
          <div className="grid-noise absolute inset-0" />
          <div className="absolute inset-x-10 bottom-10">
            <PanelState
              tone="loading"
              title="Priming global signal map"
              body="Loading region intensities, feed slices, and spike clusters for the interactive globe."
            />
          </div>
        </div>
        <div className="rounded-[28px] border border-white/8 bg-panel p-5 shadow-panel">
          <PanelState
            tone="loading"
            title="Assembling regional briefing"
            body="Selected-region metrics and source distribution will appear once the first snapshot lands."
          />
        </div>
      </div>
    );
  }

  if (regions.length === 0) {
    return (
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_300px]">
        <div className="surface-sheen relative min-h-[500px] rounded-[32px] border border-cyan/10 bg-[#050915] shadow-glow">
          <div className="grid-noise absolute inset-0" />
          <div className="absolute inset-x-10 bottom-10">
            <PanelState
              title="No region telemetry available"
              body="The globe is online, but there is not enough dashboard signal yet to render regional hotspots."
            />
          </div>
        </div>
        <div className="rounded-[28px] border border-white/8 bg-panel p-5 shadow-panel">
          <PanelState
            title="Regional briefing unavailable"
            body="Once trending topics and spikes populate, selecting a hotspot will open a richer intelligence brief here."
          />
        </div>
      </div>
    );
  }

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_300px]">
      <div
        className="surface-sheen relative overflow-hidden rounded-[32px] border border-cyan/10 bg-[#050915] shadow-glow"
        onMouseLeave={() => {
          setHoverRegionID(null);
        }}
      >
        <div className="grid-noise absolute inset-0" />
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_28%_24%,rgba(56,189,248,0.22),transparent_20%),radial-gradient(circle_at_74%_26%,rgba(139,92,246,0.14),transparent_20%),radial-gradient(circle_at_54%_84%,rgba(245,158,11,0.12),transparent_26%)]" />
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_center,transparent_30%,rgba(2,7,18,0.18)_66%,rgba(2,7,18,0.5)_100%)]" />
        <div className="absolute left-6 top-6 z-10 max-w-sm rounded-[24px] border border-white/10 bg-black/25 px-5 py-4 backdrop-blur-md">
          <div className="mb-3 flex items-center gap-2 text-[0.68rem] uppercase tracking-[0.3em] text-cyan/65">
            <Crosshair className="h-3.5 w-3.5" />
            <span>Global Hotspot Map</span>
          </div>
          <div className="flex items-end justify-between gap-4">
            <div>
              <h2 className="mt-1 font-display text-2xl font-semibold text-white">
                {focusRegion?.region.label ?? "Acquiring"}
              </h2>
              <p className="mt-2 text-sm leading-6 text-slate-300">{focusRegion?.narrative ?? focusRegion?.region.summary}</p>
            </div>
            <div className="rounded-2xl border border-cyan/20 bg-cyan/10 px-3 py-2 text-right">
              <p className="text-[0.62rem] uppercase tracking-[0.22em] text-cyan/70">Intensity</p>
              <p className="font-display text-lg font-semibold text-cyan">{formatScore(focusRegion?.score ?? 0)}</p>
            </div>
          </div>
        </div>

        <div className="absolute right-6 top-6 z-10 flex items-center gap-2 rounded-full border border-white/10 bg-black/25 px-3 py-2 text-xs uppercase tracking-[0.2em] text-slate-300 backdrop-blur-md">
          <span className={`h-2 w-2 rounded-full ${(isInspectionMode || isInteracting) ? "bg-violet animate-pulse-glow" : "bg-cyan animate-pulse-glow"}`} />
          {isInteracting ? "Tracking operator input" : isInspectionMode ? "Inspection hold" : "Autopilot sweep"}
        </div>

        <div className="absolute right-6 top-20 z-10 hidden max-w-[270px] rounded-[22px] border border-white/10 bg-black/25 px-4 py-4 text-sm text-slate-300 backdrop-blur-md lg:block">
          <p className="text-[0.65rem] uppercase tracking-[0.22em] text-cyan/60">Globe Key</p>
          <div className="mt-3 space-y-3">
            <LegendRow swatch="bg-cyan" label="Regional bloom" text="Active regions render as soft attention blooms with realistic falloff." />
            <LegendRow swatch="bg-violet" label="Heat edge" text="More intense regions widen and brighten without turning into particle effects." />
            <LegendRow swatch="bg-amber" label="Focus lock" text="Selected region and its strongest live signal." />
          </div>
        </div>

        <div className="h-[460px] w-full sm:h-[520px] 2xl:h-[580px]">
          <GlobeRenderBoundary fallback={<GlobeFallbackView regions={regions} selectedRegionID={selectedRegionID} onSelectRegion={onSelectRegion} />}>
            <Globe
              ref={globeRef}
              width={900}
              height={580}
              backgroundColor="rgba(0,0,0,0)"
              globeImageUrl={GLOBE_TEXTURE}
              bumpImageUrl={BUMP_TEXTURE}
              showAtmosphere
              atmosphereColor="#7dd3fc"
              atmosphereAltitude={0.16}
              pointsData={points}
              pointAltitude={(point: object) => (point as GlobePoint).altitude}
              pointRadius={(point: object) => (point as GlobePoint).radius}
              pointResolution={16}
              pointColor={(point: object) => (point as GlobePoint).hitColor}
              pointLabel={(point: object) => {
                const item = point as GlobePoint;
                return `<div style="padding:8px 10px;border-radius:12px;background:rgba(4,7,16,.94);border:1px solid rgba(56,189,248,.25);color:#e5eefc">
                  <div style="font-size:11px;letter-spacing:.18em;text-transform:uppercase;color:#7dd3fc">Region</div>
                  <div style="font-size:16px;font-weight:700;margin-top:4px">${item.label}</div>
                  <div style="font-size:13px;margin-top:6px;color:#bfd4f3">Curiosity Score ${formatScore(item.score)}</div>
                  <div style="font-size:12px;margin-top:4px;color:#94a3b8">${item.summary}</div>
                </div>`;
              }}
              onPointHover={(point: object | null) => {
                const nextID = point ? (point as GlobePoint).id : null;
                setHoverRegionID(nextID);
              }}
              onPointClick={(point: object) => {
                setInspectionLock(true);
                onSelectRegion((point as GlobePoint).id);
              }}
              htmlElementsData={glowClusters}
              htmlTransitionDuration={600}
              htmlLat={(cluster: object) => (cluster as GlowCluster).lat}
              htmlLng={(cluster: object) => (cluster as GlowCluster).lng}
              htmlElement={(cluster: object) => {
                const item = cluster as GlowCluster;
                const element = document.createElement("div");
                element.className = "gce-glow-cluster";
                element.style.setProperty("--cluster-color", item.color);
                element.style.setProperty("--cluster-brightness", `${item.brightness}`);
                element.style.setProperty("--cluster-aura-size", `${item.auraSize}px`);
                element.style.setProperty("--cluster-core-size", `${item.coreSize}px`);
                element.style.setProperty("--cluster-halo-size", `${item.haloSize}px`);
                element.style.setProperty("--cluster-falloff", `${item.falloff}`);
                element.style.setProperty("--cluster-label", `"${item.regionLabel}"`);
                element.dataset.selected = item.selected ? "true" : "false";
                element.dataset.hovered = item.hovered ? "true" : "false";

                const aura = document.createElement("span");
                aura.className = "gce-glow-cluster__aura";
                element.appendChild(aura);

                const halo = document.createElement("span");
                halo.className = "gce-glow-cluster__halo";
                element.appendChild(halo);

                const core = document.createElement("span");
                core.className = "gce-glow-cluster__core";
                element.appendChild(core);

                element.style.transform = "translate(-50%, -50%)";
                return element;
              }}
              ringsData={ringTargets}
              ringColor={(point: object) => {
                const item = point as GlobePoint;
                return item.id === selectedRegionID ? ["rgba(245,158,11,0.78)"] : hoverRegionID === item.id ? ["rgba(139,92,246,0.55)"] : ["rgba(56,189,248,0.0)"];
              }}
              ringMaxRadius={(point: object) => {
                const item = glowClusters.find((cluster) => cluster.regionID === (point as GlobePoint).id);
                if (!item) {
                  return 0;
                }
                return item.selected ? Math.max(6.5, item.haloSize / 8) : Math.max(4.2, item.haloSize / 10);
              }}
              ringPropagationSpeed={0.66}
              ringRepeatPeriod={ringRepeatPeriod}
            />
          </GlobeRenderBoundary>
        </div>

        {focusRegion ? (
          <div className="absolute inset-x-4 bottom-4 z-10 grid gap-4 md:inset-x-6 md:bottom-6 lg:grid-cols-[minmax(0,1fr)_260px]">
            <div className="rounded-[24px] border border-white/10 bg-black/25 px-5 py-4 backdrop-blur-md">
              <div className="mb-3 flex items-center justify-between gap-3">
                <div>
                  <p className="text-[0.68rem] uppercase tracking-[0.24em] text-cyan/60">Selected hotspot</p>
                  <p className="font-display text-xl font-semibold text-white">{focusRegion.region.label}</p>
                </div>
                <div className="rounded-full border border-violet/20 bg-violet/10 px-3 py-1 text-xs uppercase tracking-[0.18em] text-violet-200">
                  Rank {selectedRank}
                </div>
              </div>
              <div className="grid gap-3 sm:grid-cols-3">
                <HudMetric label="Topic leaders" value={toTitleCase(focusRegion.hotspotLabel)} />
                <HudMetric label="Spike watch" value={`${focusRegion.spikes.length}`} />
                <HudMetric label="Signals" value={formatCompactNumber(focusRegion.activity)} />
              </div>
            </div>
            <div className="rounded-[24px] border border-white/10 bg-black/25 px-5 py-4 backdrop-blur-md">
              <div className="mb-3 flex items-center gap-2 text-[0.68rem] uppercase tracking-[0.24em] text-cyan/60">
                <Sparkles className="h-3.5 w-3.5" />
                <span>Hot topics in view</span>
              </div>
              <div className="space-y-2">
                {focusRegion.topTopics.slice(0, 3).map((topic) => (
                  <div key={topic.topic} className="flex items-center justify-between gap-3 rounded-2xl bg-white/[0.04] px-3 py-2">
                    <div>
                      <span className="text-sm text-slate-200">{toTitleCase(topic.topic)}</span>
                      <p className="mt-1 text-[0.62rem] uppercase tracking-[0.18em] text-slate-500">
                        {topic.signalLabel} · {topic.sourceLead ?? "mixed"}
                      </p>
                    </div>
                    <span className="text-sm font-semibold text-cyan">{formatScore(topic.score)}</span>
                  </div>
                ))}
                {focusRegion.topTopics.length === 0 ? (
                  <p className="text-sm text-slate-400">No dominant topics yet in this region slice.</p>
                ) : null}
              </div>
            </div>
          </div>
        ) : null}
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-1">
        {regions.slice(0, 4).map((item, index) => {
          const isActive = item.region.id === selectedRegionID;
          const isHovered = item.region.id === hoverRegionID;
          return (
            <button
              key={item.region.id}
              type="button"
              onClick={() => {
                setInspectionLock(true);
                onSelectRegion(item.region.id);
              }}
              className={`rounded-[24px] border px-4 py-4 text-left transition duration-300 ${
                isActive
                  ? "translate-x-1 border-cyan/40 bg-cyan/10 shadow-glow"
                  : isHovered
                    ? "border-violet/25 bg-violet/10"
                    : "border-white/8 bg-white/[0.03] hover:border-violet/25 hover:bg-violet/10"
              }`}
              onMouseEnter={() => setHoverRegionID(item.region.id)}
              onMouseLeave={() => setHoverRegionID((current) => (current === item.region.id ? null : current))}
            >
              <div className="mb-2 flex items-center justify-between">
                <p className="text-[0.65rem] uppercase tracking-[0.28em] text-white/45">Hotspot {index + 1}</p>
                <span
                  className="h-2.5 w-2.5 rounded-full"
                  style={{
                    backgroundColor: item.region.accent,
                    boxShadow: `0 0 18px ${item.region.accent}`,
                  }}
                />
              </div>
              <p className="font-display text-lg font-semibold text-white">{item.region.label}</p>
              <div className="mt-2 flex items-center justify-between gap-3 text-sm">
                <span className="text-slate-300">Score {formatScore(item.score)}</span>
                <span className="rounded-full border border-white/10 bg-white/[0.04] px-2 py-1 text-xs uppercase tracking-[0.16em] text-slate-300">
                  {item.topTopics.length} topics
                </span>
              </div>
              <p className="mt-3 text-sm leading-6 text-slate-400">{item.region.summary}</p>
            </button>
          );
        })}
      </div>
    </div>
  );
}

type GlobeRenderBoundaryProps = {
  children: ReactNode;
  fallback: ReactNode;
};

type GlobeRenderBoundaryState = {
  hasError: boolean;
};

class GlobeRenderBoundary extends Component<GlobeRenderBoundaryProps, GlobeRenderBoundaryState> {
  constructor(props: GlobeRenderBoundaryProps) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch() {
    // The fallback handles degraded environments such as unavailable WebGL.
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback;
    }

    return this.props.children;
  }
}

function GlobeFallbackView({
  regions,
  selectedRegionID,
  onSelectRegion,
}: {
  regions: RegionInsight[];
  selectedRegionID: string | null;
  onSelectRegion: (regionID: string) => void;
}) {
  const selected = regions.find((item) => item.region.id === selectedRegionID) ?? regions[0];

  return (
    <div className="relative h-full overflow-hidden rounded-[28px] border border-cyan/10 bg-[radial-gradient(circle_at_50%_40%,rgba(14,116,144,0.24),transparent_28%),radial-gradient(circle_at_50%_52%,rgba(15,23,42,0.9),rgba(2,6,23,0.98))] px-6 py-6">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_center,rgba(56,189,248,0.14),transparent_22%),radial-gradient(circle_at_center,transparent_26%,rgba(139,92,246,0.06)_27%,transparent_32%),radial-gradient(circle_at_center,transparent_42%,rgba(245,158,11,0.04)_43%,transparent_50%)]" />
      <div className="relative grid h-full content-between gap-5">
        <div>
          <p className="text-[0.68rem] uppercase tracking-[0.24em] text-cyan/60">Fallback region map</p>
          <h3 className="mt-2 font-display text-2xl font-semibold text-white">WebGL unavailable, regional briefing remains online.</h3>
          <p className="mt-3 max-w-lg text-sm leading-6 text-slate-300">
            The globe could not initialize in this environment, so this fallback keeps hotspot ranking and selection usable without breaking the dashboard.
          </p>
        </div>
        <div className="grid gap-4 md:grid-cols-2">
          {regions.slice(0, 6).map((item) => {
            const isActive = item.region.id === selected?.region.id;
            return (
              <button
                key={item.region.id}
                type="button"
                onClick={() => onSelectRegion(item.region.id)}
                className={`rounded-[24px] border px-4 py-4 text-left transition duration-300 ${
                  isActive
                    ? "border-cyan/35 bg-cyan/10 shadow-glow"
                    : "border-white/10 bg-white/[0.04] hover:border-violet/25 hover:bg-violet/10"
                }`}
              >
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="text-[0.65rem] uppercase tracking-[0.24em] text-cyan/60">Hotspot rank {item.rank ?? "—"}</p>
                    <p className="mt-2 font-display text-lg font-semibold text-white">{item.region.label}</p>
                  </div>
                  <span
                    className="h-3 w-3 rounded-full"
                    style={{ backgroundColor: item.region.accent, boxShadow: `0 0 18px ${item.region.accent}` }}
                  />
                </div>
                <p className="mt-3 text-sm leading-6 text-slate-300">{item.narrative}</p>
                <div className="mt-4 flex items-center justify-between gap-3 text-sm">
                  <span className="text-slate-400">Score {formatScore(item.score)}</span>
                  <span className="rounded-full border border-white/10 bg-white/[0.04] px-2 py-1 text-[0.65rem] uppercase tracking-[0.18em] text-slate-300">
                    {toTitleCase(item.hotspotLabel)}
                  </span>
                </div>
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function HudMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-white/8 bg-white/[0.04] px-3 py-3">
      <p className="text-[0.62rem] uppercase tracking-[0.2em] text-slate-500">{label}</p>
      <p className="mt-2 font-display text-lg font-semibold text-white line-clamp-2">{value}</p>
    </div>
  );
}

function LegendRow({ swatch, label, text }: { swatch: string; label: string; text: string }) {
  return (
    <div className="flex items-start gap-3">
      <span className={`mt-1 h-2.5 w-2.5 rounded-full ${swatch}`} />
      <div>
        <p className="text-xs uppercase tracking-[0.18em] text-white/65">{label}</p>
        <p className="mt-1 text-xs leading-5 text-slate-400">{text}</p>
      </div>
    </div>
  );
}

function buildGlowClusters(
  regions: RegionInsight[],
  selectedRegionID: string | null,
  hoverRegionID: string | null,
): GlowCluster[] {
  return regions.map((region) => {
    const selected = region.region.id === selectedRegionID;
    const hovered = region.region.id === hoverRegionID;
    const emphasis = selected ? 1.3 : hovered ? 1.1 : 1;
    const normalizedActivity = Math.min(1, region.activity / 420);
    const normalizedScore = Math.min(1, region.score / 52000);
    return {
      id: `${region.region.id}:cluster`,
      regionID: region.region.id,
      regionLabel: region.region.label,
      lat: region.region.lat,
      lng: region.region.lng,
      activity: region.activity,
      brightness: Number((0.58 + normalizedScore * 0.46 + (selected ? 0.26 : hovered ? 0.1 : 0)).toFixed(2)),
      auraSize: Math.round((74 + normalizedActivity * 96 + normalizedScore * 44) * emphasis),
      coreSize: Math.round((14 + normalizedActivity * 10 + normalizedScore * 6) * emphasis),
      haloSize: Math.round((86 + normalizedActivity * 76 + normalizedScore * 34) * emphasis),
      falloff: Number((0.52 + normalizedActivity * 0.22 + normalizedScore * 0.18).toFixed(2)),
      color: selected ? "#f59e0b" : hovered ? "#7dd3fc" : region.region.accent,
      selected,
      hovered,
    } satisfies GlowCluster;
  });
}
