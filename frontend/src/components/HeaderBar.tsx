import { Activity, ArrowUpRight, Dot, Radar, Sparkle } from "lucide-react";
import { formatClock, formatScore } from "../lib/format";

type HeaderBarProps = {
  curiosityIndex: number | null;
  selectedRegionLabel: string;
  serviceStatus: string;
  lastUpdated: string | null;
  isRefreshing: boolean;
};

export function HeaderBar({
  curiosityIndex,
  selectedRegionLabel,
  serviceStatus,
  lastUpdated,
  isRefreshing,
}: HeaderBarProps) {
  const normalizedStatus = serviceStatus.toLowerCase();
  const isLive = normalizedStatus === "ok";
  const statusLabel =
    normalizedStatus === "ok"
      ? "Live"
      : normalizedStatus === "offline"
        ? "Offline"
        : normalizedStatus === "loading"
          ? "Syncing"
          : serviceStatus;
  const statusDetail =
    normalizedStatus === "ok"
      ? isRefreshing
        ? "Syncing stream"
        : "Healthy"
      : normalizedStatus === "offline"
        ? "Backend unavailable"
        : "Awaiting feed";

  return (
    <header className="surface-sheen relative overflow-hidden rounded-[30px] border border-cyan/15 bg-panel px-5 py-5 shadow-panel backdrop-blur-xl sm:px-6">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_10%_10%,rgba(56,189,248,0.14),transparent_24%),radial-gradient(circle_at_88%_18%,rgba(139,92,246,0.11),transparent_24%)]" />
      <div className="relative flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div className="space-y-3">
          <div className="flex items-center gap-3">
            <div className="rounded-full border border-cyan/20 bg-cyan/10 p-2 text-cyan shadow-glow">
              <Radar className="h-5 w-5" />
            </div>
            <div>
              <p className="text-[0.68rem] uppercase tracking-[0.42em] text-cyan/65">Mission Control For Global Attention</p>
              <h1 className="font-display text-3xl font-bold text-white sm:text-4xl">Global Curiosity Engine</h1>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-3 text-xs uppercase tracking-[0.2em] text-slate-400">
            <span className="inline-flex items-center gap-2 rounded-full border border-emerald-400/12 bg-emerald-400/10 px-3 py-1 text-emerald-200">
              <span className={`h-2 w-2 rounded-full ${isLive ? "bg-emerald-300 animate-pulse-glow" : normalizedStatus === "offline" ? "bg-amber-300" : "bg-slate-400"}`} />
              {isLive ? "Realtime backend online" : statusLabel}
            </span>
            <span className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.04] px-3 py-1">
              <Sparkle className="h-3.5 w-3.5 text-cyan" />
              Globe-led exploration
            </span>
            {isRefreshing ? (
              <span className="inline-flex items-center gap-2 rounded-full border border-violet/20 bg-violet/10 px-3 py-1 text-violet-200">
                <span className="h-2 w-2 rounded-full bg-violet animate-pulse-glow" />
                Refreshing
              </span>
            ) : null}
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
          <StatPill
            label="Status"
            value={statusLabel}
            accent={isLive ? "text-emerald-300" : normalizedStatus === "offline" ? "text-amber-200" : "text-slate-200"}
            icon={<Dot className="h-5 w-5" />}
            detail={statusDetail}
          />
          <StatPill
            label="Curiosity Index"
            value={formatScore(curiosityIndex)}
            accent="text-cyan"
            icon={<Activity className="h-4 w-4" />}
            detail="Cross-source attention"
          />
          <StatPill
            label="Region Focus"
            value={selectedRegionLabel}
            accent="text-violet-300"
            icon={<ArrowUpRight className="h-4 w-4" />}
            detail="Current command lens"
          />
          <StatPill label="Last Updated" value={formatClock(lastUpdated)} accent="text-white/90" detail="Signal timestamp" />
        </div>
      </div>
    </header>
  );
}

type StatPillProps = {
  label: string;
  value: string;
  accent: string;
  icon?: React.ReactNode;
  detail?: string;
};

function StatPill({ label, value, accent, icon, detail }: StatPillProps) {
  return (
    <div className="rounded-2xl border border-white/8 bg-white/[0.035] px-4 py-3 transition duration-300 hover:border-cyan/20 hover:bg-white/[0.05]">
      <div className="mb-1 flex items-center gap-2 text-[0.68rem] uppercase tracking-[0.26em] text-white/40">
        {icon}
        <span>{label}</span>
      </div>
      <p className={`truncate font-display text-lg font-semibold ${accent}`}>{value}</p>
      {detail ? <p className="mt-1 text-xs text-slate-500">{detail}</p> : null}
    </div>
  );
}
