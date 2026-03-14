import { AlertTriangle } from "lucide-react";
import { formatRelativeTime, formatScore, toTitleCase } from "../lib/format";
import { PanelState } from "./PanelState";
import { SectionCard } from "./SectionCard";
import type { RegionInsight } from "../types";

type SpikeAlertsPanelProps = {
  region: RegionInsight | null;
  isLoading?: boolean;
};

export function SpikeAlertsPanel({ region, isLoading = false }: SpikeAlertsPanelProps) {
  return (
    <SectionCard title="Spike Alerts" eyebrow="Escalation Queue" tone="amber" className="h-full">
      {region ? (
        <div className="mb-4 flex items-center justify-between gap-3 rounded-2xl border border-amber/15 bg-amber/10 px-4 py-3 text-sm text-amber-100/90">
          <span>Regional spike pressure</span>
          <span className="font-display text-lg font-semibold">{formatScore(region.spikePressure)}</span>
        </div>
      ) : null}
      <div className="space-y-3">
        {isLoading && !region ? (
          <PanelState
            tone="loading"
            title="Initializing alert posture"
            body="Watching scored topics for burst conditions and major jumps."
            compact
          />
        ) : null}
        {region?.spikes.length ? (
          region.spikes.map((spike) => (
            <div key={spike.id} className="rounded-2xl border border-amber/20 bg-amber/10 px-4 py-4 shadow-spike">
              <div className="mb-2 flex items-center justify-between gap-3">
                <div className="flex items-center gap-2 text-amber-200">
                  <AlertTriangle className="h-4 w-4" />
                  <span className="text-xs uppercase tracking-[0.2em]">{spike.spikeType}</span>
                </div>
                <span className="text-xs text-amber-100/80">{formatRelativeTime(spike.timestamp)}</span>
              </div>
              <p className="font-display text-lg font-semibold text-white">{toTitleCase(spike.topic)}</p>
              <div className="mt-3 flex items-center justify-between gap-3 text-sm text-amber-100/90">
                <span>Current score {formatScore(spike.currentScore)}</span>
                <span className="rounded-full border border-amber/20 bg-black/10 px-2 py-1 text-[0.64rem] uppercase tracking-[0.18em]">
                  {spike.sourceLead ?? "mixed"}
                </span>
              </div>
            </div>
          ))
        ) : (
          <PanelState title="Alert board clear" body="No active spike alerts for the selected region." />
        )}
      </div>
    </SectionCard>
  );
}
