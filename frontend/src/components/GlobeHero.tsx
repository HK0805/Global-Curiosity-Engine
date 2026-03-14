import { useEffect, useRef, useState } from "react";
import { Crosshair, Sparkles } from "lucide-react";
import Globe, { type GlobeMethods } from "react-globe.gl";
import { PanelState } from "./PanelState";
import type { RegionInsight } from "../types";
import { formatCompactNumber, formatScore, toTitleCase } from "../lib/format";

const GLOBE_TEXTURE =
  "https://unpkg.com/three-globe/example/img/earth-dark.jpg";
const BUMP_TEXTURE =
  "https://unpkg.com/three-globe/example/img/earth-topology.png";

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
  size: number;
  color: string;
  score: number;
  label: string;
  summary: string;
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

  const points: GlobePoint[] = regions.map((item) => ({
    id: item.region.id,
    lat: item.region.lat,
    lng: item.region.lng,
    size:
      0.5 +
      Math.min((item.score + item.spikePressure * 0.4) / 16, 2.8) +
      ((item.region.id === selectedRegionID ? 0.45 : 0) + (item.region.id === hoverRegionID ? 0.25 : 0)),
    color: item.region.accent,
    score: item.score,
    label: item.region.label,
    summary: item.narrative,
  }));

  const focusRegion = regions.find((item) => item.region.id === (hoverRegionID ?? selectedRegionID)) ?? regions[0];
  const selectedRank = focusRegion?.rank ?? regions.findIndex((item) => item.region.id === focusRegion?.region.id) + 1;

  useEffect(() => {
    const globe = globeRef.current;
    if (!globe) {
      return;
    }

    const controls = globe.controls() as GlobeControls;
    controls.autoRotate = true;
    controls.autoRotateSpeed = 0.45;
    controls.enablePan = false;
    controls.minDistance = 180;
    controls.maxDistance = 320;
    controls.addEventListener?.("start", () => setIsInteracting(true));
    controls.addEventListener?.("end", () => setIsInteracting(false));
  }, []);

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
      1100,
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
      <div className="surface-sheen relative overflow-hidden rounded-[32px] border border-cyan/10 bg-[#050915] shadow-glow">
        <div className="grid-noise absolute inset-0" />
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_30%_20%,rgba(56,189,248,0.16),transparent_22%),radial-gradient(circle_at_78%_28%,rgba(139,92,246,0.12),transparent_20%),radial-gradient(circle_at_50%_90%,rgba(245,158,11,0.07),transparent_28%)]" />
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
          <span className={`h-2 w-2 rounded-full ${isInteracting ? "bg-violet animate-pulse-glow" : "bg-cyan animate-pulse-glow"}`} />
          {isInteracting ? "Tracking operator input" : "Autopilot sweep"}
        </div>

        <div className="h-[460px] w-full sm:h-[520px] 2xl:h-[580px]">
          <Globe
            ref={globeRef}
            width={900}
            height={580}
            backgroundColor="rgba(0,0,0,0)"
            globeImageUrl={GLOBE_TEXTURE}
            bumpImageUrl={BUMP_TEXTURE}
            showAtmosphere
            atmosphereColor="#40c4ff"
            atmosphereAltitude={0.18}
            pointsData={points}
            pointAltitude="size"
            pointRadius={0.85}
            pointColor={(point: object) => (point as GlobePoint).color}
            pointLabel={(point: object) => {
              const item = point as GlobePoint;
              return `<div style="padding:8px 10px;border-radius:12px;background:rgba(4,7,16,.94);border:1px solid rgba(56,189,248,.25);color:#e5eefc">
                <div style="font-size:11px;letter-spacing:.18em;text-transform:uppercase;color:#7dd3fc">Region</div>
                <div style="font-size:16px;font-weight:700;margin-top:4px">${item.label}</div>
                <div style="font-size:13px;margin-top:6px;color:#bfd4f3">Curiosity Score ${formatScore(item.score)}</div>
              </div>`;
            }}
            onPointHover={(point: object | null) => setHoverRegionID(point ? (point as GlobePoint).id : null)}
            onPointClick={(point: object) => onSelectRegion((point as GlobePoint).id)}
            ringsData={points}
            ringColor={(point: object) => [((point as GlobePoint).color)]}
            ringMaxRadius={(point: object) => Math.max(2.5, (point as GlobePoint).size * 3.8)}
            ringPropagationSpeed={1.3}
            ringRepeatPeriod={1250}
          />
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
              onClick={() => onSelectRegion(item.region.id)}
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

function HudMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-white/8 bg-white/[0.04] px-3 py-3">
      <p className="text-[0.62rem] uppercase tracking-[0.2em] text-slate-500">{label}</p>
      <p className="mt-2 font-display text-lg font-semibold text-white line-clamp-2">{value}</p>
    </div>
  );
}
