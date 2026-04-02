import { SOURCE_VIEW_OPTIONS } from "../lib/regions";
import type { RegionInsight, SourceViewKey } from "../types";

type SourceViewTabsProps = {
  region: RegionInsight | null;
  selectedSourceView: SourceViewKey;
  onChange: (next: SourceViewKey) => void;
};

export function SourceViewTabs({ region, selectedSourceView, onChange }: SourceViewTabsProps) {
  return (
    <div className="flex flex-wrap gap-2">
      {SOURCE_VIEW_OPTIONS.map((option) => {
        const hasSignal = option.key === "overview" || region?.sourceViews[option.key]?.hasSignal;
        const isActive = selectedSourceView === option.key;

        return (
          <button
            key={option.key}
            type="button"
            onClick={() => onChange(option.key)}
            aria-pressed={isActive}
            data-source-view={option.key}
            className={`rounded-full border px-3 py-2 text-xs uppercase tracking-[0.18em] transition duration-300 ${
              isActive
                ? "border-cyan/35 bg-cyan/10 text-cyan shadow-glow"
                : hasSignal
                  ? "border-white/10 bg-white/[0.03] text-slate-300 hover:border-violet/25 hover:bg-violet/10"
                  : "border-white/8 bg-white/[0.02] text-slate-500 hover:border-white/12 hover:bg-white/[0.04]"
            }`}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
