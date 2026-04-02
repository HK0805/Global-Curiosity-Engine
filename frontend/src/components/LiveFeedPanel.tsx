import { ExternalLink } from "lucide-react";
import { formatRelativeTime, toTitleCase } from "../lib/format";
import { PanelState } from "./PanelState";
import { SectionCard } from "./SectionCard";
import type { RegionInsight, RegionSourceView, SourceViewKey } from "../types";

type LiveFeedPanelProps = {
  region: RegionInsight | null;
  view?: RegionSourceView | null;
  selectedSourceView?: SourceViewKey;
  isLoading?: boolean;
};

export function LiveFeedPanel({ region, view, selectedSourceView = "overview", isLoading = false }: LiveFeedPanelProps) {
  const activeView = view ?? region?.sourceViews.overview ?? null;
  const viewLabel = selectedSourceView === "overview" ? "All signals" : activeView?.label ?? "Selected source";

  return (
    <SectionCard title="Live Event Feed" eyebrow="Realtime Intake" tone="cyan" className="h-full">
      {region && activeView ? (
        <div className="mb-4 rounded-2xl border border-white/8 bg-white/[0.03] px-4 py-3">
          <p className="text-[0.65rem] uppercase tracking-[0.18em] text-cyan/60">{viewLabel}</p>
          <p className="mt-2 text-sm leading-6 text-slate-300">{activeView.narrative}</p>
        </div>
      ) : null}
      <div className="space-y-3" data-panel-source={selectedSourceView} data-panel-kind="live-feed">
        {isLoading && !region ? (
          <PanelState
            tone="loading"
            title="Streaming feed online"
            body="Waiting for the first event slice to reach this panel."
            compact
          />
        ) : null}
        {activeView?.feed.length ? (
          activeView.feed.map((event) => (
            <a
              key={event.id}
              href={event.url ?? "#"}
              target={event.url ? "_blank" : undefined}
              rel={event.url ? "noreferrer" : undefined}
              className="group block rounded-2xl border border-white/8 bg-white/[0.03] px-4 py-4 transition duration-300 hover:border-cyan/30 hover:bg-cyan/5"
            >
              <div className="mb-2 flex items-center justify-between gap-3 text-xs uppercase tracking-[0.18em] text-slate-400">
                <span className="rounded-full border border-white/10 bg-white/[0.03] px-2 py-1">{toTitleCase(event.source)}</span>
                <span>{formatRelativeTime(event.timestamp)}</span>
              </div>
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="line-clamp-2 text-sm leading-6 text-slate-200">{event.title}</p>
                  <p className="mt-2 text-[0.65rem] uppercase tracking-[0.18em] text-slate-500">
                    Routed into {region?.region.label ?? "selected region"} · {viewLabel}
                  </p>
                </div>
                {event.url ? <ExternalLink className="mt-1 h-4 w-4 shrink-0 text-cyan transition group-hover:translate-x-0.5 group-hover:-translate-y-0.5" /> : null}
              </div>
            </a>
          ))
        ) : (
          <PanelState title={`No ${viewLabel.toLowerCase()} events in view`} body="Live event routing is active, but this lens is quiet at the moment." />
        )}
      </div>
    </SectionCard>
  );
}
