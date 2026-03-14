import type { ReactNode } from "react";

type SectionCardProps = {
  title: string;
  eyebrow?: string;
  children: ReactNode;
  aside?: ReactNode;
  className?: string;
  tone?: "default" | "cyan" | "amber" | "violet";
};

export function SectionCard({
  title,
  eyebrow,
  children,
  aside,
  className = "",
  tone = "default",
}: SectionCardProps) {
  const toneStyles =
    tone === "amber"
      ? "border-amber/15"
      : tone === "violet"
        ? "border-violet/15"
        : tone === "cyan"
          ? "border-cyan/15"
          : "border-white/8";

  return (
    <section
      className={`surface-sheen relative rounded-[28px] border bg-panel p-5 shadow-panel backdrop-blur-xl transition duration-500 hover:border-white/12 hover:shadow-glow sm:p-6 ${toneStyles} ${className}`}
    >
      <div className="pointer-events-none absolute inset-0 rounded-[28px] bg-[radial-gradient(circle_at_top_left,rgba(56,189,248,0.08),transparent_28%),radial-gradient(circle_at_top_right,rgba(139,92,246,0.07),transparent_24%)]" />
      <div className="relative mb-4 flex items-start justify-between gap-3">
        <div>
          {eyebrow ? (
            <p className="mb-1 text-[0.68rem] uppercase tracking-[0.32em] text-cyan/70">{eyebrow}</p>
          ) : null}
          <h2 className="font-display text-xl font-semibold text-white">{title}</h2>
        </div>
        {aside}
      </div>
      <div className="relative">{children}</div>
    </section>
  );
}
