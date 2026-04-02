import { Globe2, RadioTower, Siren, Sparkles } from "lucide-react";
import { formatRelativeTime, formatScore, toTitleCase } from "../lib/format";
import { PanelState } from "./PanelState";
import { SectionCard } from "./SectionCard";
import type { RegionInsight, RegionSourceView, SourceViewKey } from "../types";

type RegionPanelProps = {
  region: RegionInsight | null;
  view?: RegionSourceView | null;
  selectedSourceView?: SourceViewKey;
  isLoading?: boolean;
};

export function RegionPanel({ region, view, selectedSourceView = "overview", isLoading = false }: RegionPanelProps) {
  if (isLoading && !region) {
    return (
      <SectionCard title="Regional Intelligence" eyebrow="Selected Region" tone="cyan">
        <PanelState
          tone="loading"
          title="Building regional briefing"
          body="Curiosity score, topic leaders, spike posture, and source composition are being assembled."
        />
      </SectionCard>
    );
  }

  if (!region) {
    return (
      <SectionCard title="Regional Intelligence" eyebrow="Selected Region" tone="cyan">
        <PanelState
          title="No region selected"
          body="Choose a hotspot on the globe to focus the intelligence panel."
        />
      </SectionCard>
    );
  }

  const activeView = view ?? region.sourceViews.overview;
  const viewLabel = selectedSourceView === "overview" ? "All signals" : activeView.label;

  return (
    <SectionCard
      title={region.region.label}
      eyebrow="Regional Intelligence"
      aside={
        <div className="rounded-full border border-cyan/20 bg-cyan/10 px-3 py-1 text-xs uppercase tracking-[0.22em] text-cyan">
          Score {formatScore(region.score)}
        </div>
      }
      className="h-full xl:sticky xl:top-5"
      tone="cyan"
    >
      <div className="space-y-5" data-panel-source={selectedSourceView} data-panel-kind="region-panel">
        <div className="rounded-[24px] border border-white/10 bg-white/[0.03] p-5">
          <div className="mb-4 flex items-start justify-between gap-4">
            <div>
              <p className="text-[0.68rem] uppercase tracking-[0.24em] text-cyan/60">Selected theater</p>
              <h3 className="mt-2 font-display text-3xl font-semibold text-white">{region.region.label}</h3>
              <p className="mt-3 max-w-xl text-sm leading-6 text-slate-300">{activeView.narrative}</p>
            </div>
            <div className="rounded-2xl border border-violet/20 bg-violet/10 px-4 py-3 text-right">
              <p className="text-[0.62rem] uppercase tracking-[0.22em] text-violet-200/75">{viewLabel}</p>
              <p className="font-display text-2xl font-semibold text-violet-100">{formatScore(activeView.score)}</p>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2 text-[0.68rem] uppercase tracking-[0.18em] text-slate-400">
            <span className="rounded-full border border-white/10 bg-white/[0.04] px-3 py-1">
              Hotspot {toTitleCase(activeView.hotspotLabel)}
            </span>
            <span className="rounded-full border border-white/10 bg-white/[0.04] px-3 py-1">
              Dominant source {toTitleCase(activeView.dominantSource)}
            </span>
            <span className="rounded-full border border-white/10 bg-white/[0.04] px-3 py-1">
              Topic diversity {activeView.topicDiversity}
            </span>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <InsightMetric icon={<Sparkles className="h-4 w-4" />} label="Curiosity Score" value={formatScore(activeView.score)} />
          <InsightMetric icon={<RadioTower className="h-4 w-4" />} label="Activity Signals" value={`${activeView.activity}`} />
          <InsightMetric icon={<Siren className="h-4 w-4" />} label="Spike Pressure" value={formatScore(activeView.spikePressure)} />
          <InsightMetric icon={<Globe2 className="h-4 w-4" />} label="Lead Source" value={toTitleCase(activeView.dominantSource)} />
        </div>

        <div>
          <p className="mb-3 text-[0.68rem] uppercase tracking-[0.28em] text-cyan/60">Top Topics</p>
          <div className="space-y-3">
            {activeView.topTopics.length === 0 ? (
              <PanelState title={`No ${viewLabel.toLowerCase()} topic leaders yet`} body="Signals are live, but no topic cluster has separated from the pack yet." compact />
            ) : (
              activeView.topTopics.map((topic) => (
                <div key={topic.topic} className="rounded-2xl border border-white/8 bg-white/[0.03] px-4 py-4 transition duration-300 hover:border-cyan/20 hover:bg-cyan/5">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p className="font-display text-base font-semibold text-white">{toTitleCase(topic.topic)}</p>
                      <div className="mt-2 flex items-center gap-3 text-xs uppercase tracking-[0.16em] text-slate-400">
                        <span>{topic.mentions} mentions</span>
                        <span>{topic.sources} sources</span>
                        {topic.signalLabel ? <span>{topic.signalLabel}</span> : null}
                      </div>
                    </div>
                    <div className="rounded-xl border border-cyan/20 bg-cyan/10 px-3 py-2 text-right">
                      <p className="text-[0.6rem] uppercase tracking-[0.18em] text-cyan/65">Score</p>
                      <p className="text-sm font-semibold text-cyan">{formatScore(topic.score)}</p>
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        <div>
          <p className="mb-3 text-[0.68rem] uppercase tracking-[0.28em] text-cyan/60">Recent Spikes</p>
          <div className="space-y-3">
            {activeView.spikes.length === 0 ? (
              <PanelState title={`No ${viewLabel.toLowerCase()} bursts`} body="Spike watch is clear for this region in the current source lens." compact />
            ) : (
              activeView.spikes.map((spike) => (
                <div key={spike.id} className="rounded-2xl border border-amber/25 bg-amber/10 px-4 py-3 shadow-spike">
                  <div className="flex items-center justify-between gap-3">
                    <p className="font-display text-base font-semibold text-white">{toTitleCase(spike.topic)}</p>
                    <span className="text-xs uppercase tracking-[0.18em] text-amber-200">{spike.spikeType}</span>
                  </div>
                  <div className="mt-2 text-sm text-amber-100/90">
                    Score {formatScore(spike.currentScore)} · {formatRelativeTime(spike.timestamp)}
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        <div>
          <p className="mb-3 text-[0.68rem] uppercase tracking-[0.28em] text-cyan/60">Source Contribution</p>
          <div className="space-y-3">
            {activeView.contributions.map((item) => (
              <div key={item.source}>
                <div className="mb-2 flex items-center justify-between text-sm text-slate-300">
                  <span>{toTitleCase(item.source)}</span>
                  <span>{item.share}%</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-white/6">
                  <div
                    className="h-full rounded-full bg-gradient-to-r from-cyan via-violet to-amber transition-all duration-700"
                    style={{ width: `${Math.min(100, item.share)}%` }}
                  />
                </div>
                <p className="mt-1 text-xs text-slate-500">Weighted signal {formatScore(item.value)}</p>
              </div>
            ))}
            {activeView.contributions.length === 0 ? (
              <PanelState title="No contribution split yet" body="The selected source lens does not have enough signal to show a source mix." compact />
            ) : null}
          </div>
        </div>
      </div>
    </SectionCard>
  );
}

type InsightMetricProps = {
  icon: React.ReactNode;
  label: string;
  value: string;
};

function InsightMetric({ icon, label, value }: InsightMetricProps) {
  return (
    <div className="rounded-2xl border border-white/8 bg-white/[0.03] p-4 transition duration-300 hover:border-cyan/20 hover:bg-white/[0.05]">
      <div className="mb-2 flex items-center gap-2 text-cyan">{icon}</div>
      <p className="text-[0.68rem] uppercase tracking-[0.22em] text-slate-500">{label}</p>
      <p className="mt-2 font-display text-lg font-semibold text-white">{value}</p>
    </div>
  );
}
