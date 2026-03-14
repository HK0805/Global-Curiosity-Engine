import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { formatScore } from "../lib/format";
import { PanelState } from "./PanelState";
import { SectionCard } from "./SectionCard";
import type { RegionInsight } from "../types";

type TrendChartPanelProps = {
  region: RegionInsight | null;
  isLoading?: boolean;
  className?: string;
};

export function TrendChartPanel({ region, isLoading = false, className = "" }: TrendChartPanelProps) {
  const hasTrend = Boolean(region?.trend.length);

  return (
    <SectionCard title="Curiosity Trend" eyebrow="Regional Momentum" tone="violet" className={`h-full ${className}`}>
      {isLoading && !region ? (
        <PanelState
          tone="loading"
          title="Drawing trend line"
          body="Composing the current curiosity pulse for the selected region."
        />
      ) : hasTrend ? (
        <>
          <div className="mb-4 flex items-center justify-between gap-3 rounded-2xl border border-white/8 bg-white/[0.03] px-4 py-3 text-sm text-slate-300">
            <span>{region?.region.label} trajectory anchored by {region?.hotspotLabel}</span>
            <span className="rounded-full border border-violet/20 bg-violet/10 px-3 py-1 text-xs uppercase tracking-[0.18em] text-violet-200">
              {region?.dominantSource}
            </span>
          </div>
          <div className="mb-4 grid grid-cols-3 gap-3">
            {region!.trend.slice(-3).map((point) => (
              <div key={point.label} className="rounded-2xl border border-white/8 bg-white/[0.03] px-3 py-3">
                <p className="text-[0.62rem] uppercase tracking-[0.18em] text-slate-500">{point.label}</p>
                <p className="mt-2 font-display text-lg font-semibold text-white">{formatScore(point.value)}</p>
              </div>
            ))}
          </div>
          <div className="h-[240px]">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={region?.trend ?? []} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                <defs>
                  <linearGradient id="trendFill" x1="0" x2="0" y1="0" y2="1">
                    <stop offset="0%" stopColor="#38bdf8" stopOpacity={0.65} />
                    <stop offset="65%" stopColor="#8b5cf6" stopOpacity={0.18} />
                    <stop offset="100%" stopColor="#8b5cf6" stopOpacity={0.02} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke="rgba(148, 163, 184, 0.12)" vertical={false} />
                <XAxis dataKey="label" stroke="rgba(191, 219, 254, 0.55)" tickLine={false} axisLine={false} />
                <YAxis stroke="rgba(191, 219, 254, 0.55)" tickLine={false} axisLine={false} tickFormatter={(value) => `${value}`} />
                <Tooltip
                  contentStyle={{
                    background: "rgba(5, 9, 21, 0.94)",
                    border: "1px solid rgba(56, 189, 248, 0.16)",
                    borderRadius: "18px",
                    color: "#e5eefc",
                  }}
                  formatter={(value: number) => [`${formatScore(value)}`, "Curiosity"]}
                />
                <Area
                  type="monotone"
                  dataKey="value"
                  stroke="#38bdf8"
                  strokeWidth={3}
                  fill="url(#trendFill)"
                  activeDot={{ r: 6, stroke: "#fff", strokeWidth: 1 }}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </>
      ) : (
        <PanelState title="Trend line unavailable" body="There is not enough recent signal to draw momentum for this region." />
      )}
    </SectionCard>
  );
}
