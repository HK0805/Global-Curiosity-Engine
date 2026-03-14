import type { ReactNode } from "react";
import { AlertCircle, LoaderCircle, Orbit } from "lucide-react";

type PanelStateProps = {
  title: string;
  body: string;
  tone?: "loading" | "empty" | "error";
  icon?: ReactNode;
  compact?: boolean;
};

export function PanelState({ title, body, tone = "empty", icon, compact = false }: PanelStateProps) {
  const palette =
    tone === "error"
      ? "border-amber/20 bg-amber/10 text-amber-100 shadow-spike"
      : tone === "loading"
        ? "border-cyan/20 bg-cyan/10 text-cyan-100 shadow-glow"
        : "border-white/10 bg-white/[0.03] text-slate-300";

  const resolvedIcon =
    icon ??
    (tone === "error" ? (
      <AlertCircle className="h-5 w-5" />
    ) : tone === "loading" ? (
      <LoaderCircle className="h-5 w-5 animate-spin" />
    ) : (
      <Orbit className="h-5 w-5" />
    ));

  return (
    <div
      className={`rounded-2xl border px-4 ${compact ? "py-4" : "py-7"} ${palette}`}
    >
      <div className="mb-3 flex items-center gap-3">
        <div className="rounded-full border border-current/15 bg-black/10 p-2">{resolvedIcon}</div>
        <div>
          <p className="font-display text-base font-semibold text-white">{title}</p>
          <p className="mt-1 text-sm leading-6 opacity-80">{body}</p>
        </div>
      </div>
    </div>
  );
}

type SkeletonBlockProps = {
  className?: string;
};

export function SkeletonBlock({ className = "" }: SkeletonBlockProps) {
  return (
    <div
      className={`overflow-hidden rounded-2xl bg-white/[0.05] ${className}`}
    >
      <div className="h-full w-full animate-pulse bg-[linear-gradient(90deg,transparent,rgba(255,255,255,0.08),transparent)]" />
    </div>
  );
}
