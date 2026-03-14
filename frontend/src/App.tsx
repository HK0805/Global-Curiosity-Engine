import { lazy, Suspense, useEffect, useState } from "react";
import { AlertCircle } from "lucide-react";
import { fetchDashboardState } from "./lib/api";
import { REGION_DEFINITIONS } from "./lib/regions";
import { HeaderBar } from "./components/HeaderBar";
import { RegionPanel } from "./components/RegionPanel";
import { LiveFeedPanel } from "./components/LiveFeedPanel";
import { SpikeAlertsPanel } from "./components/SpikeAlertsPanel";
import { TrendChartPanel } from "./components/TrendChartPanel";
import { SkeletonBlock } from "./components/PanelState";
import type { DashboardState } from "./types";

const GlobeHero = lazy(async () => import("./components/GlobeHero").then((module) => ({ default: module.GlobeHero })));

export default function App() {
  const [dashboard, setDashboard] = useState<DashboardState | null>(null);
  const [selectedRegionID, setSelectedRegionID] = useState<string | null>(REGION_DEFINITIONS[0]?.id ?? null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [isInitialLoading, setIsInitialLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);

  useEffect(() => {
    let canceled = false;

    const load = async (isBackground: boolean) => {
      if (!isBackground) {
        setIsInitialLoading(true);
      } else {
        setIsRefreshing(true);
      }

      try {
        const state = await fetchDashboardState();
        if (canceled) {
          return;
        }
        setDashboard(state);
        setSelectedRegionID((current) => current ?? state.regions[0]?.region.id ?? null);
        setErrorMessage(null);
      } catch (error) {
        if (canceled) {
          return;
        }
        setErrorMessage(error instanceof Error ? error.message : "Failed to load dashboard");
      } finally {
        if (!canceled) {
          setIsInitialLoading(false);
          setIsRefreshing(false);
        }
      }
    };

    void load(false);

    const intervalID = window.setInterval(() => {
      void load(true);
    }, 30000);

    return () => {
      canceled = true;
      window.clearInterval(intervalID);
    };
  }, []);

  const selectedRegion =
    dashboard?.regions.find((item) => item.region.id === selectedRegionID) ?? dashboard?.regions[0] ?? null;
  const isOfflineWithoutSnapshot = Boolean(errorMessage && !dashboard);

  return (
    <main className="relative min-h-screen overflow-hidden bg-radar px-4 py-4 text-slate-100 sm:px-6 sm:py-5 lg:px-8">
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute left-[-12%] top-[-8%] h-[420px] w-[420px] rounded-full bg-cyan/10 blur-[140px]" />
        <div className="absolute bottom-[-10%] right-[-8%] h-[360px] w-[360px] rounded-full bg-violet/10 blur-[140px]" />
      </div>
      <div className="relative mx-auto flex max-w-[1660px] flex-col gap-5">
        <HeaderBar
          curiosityIndex={dashboard?.curiosityIndex?.curiosity_index ?? null}
          selectedRegionLabel={selectedRegion?.region.label ?? "Global"}
          serviceStatus={dashboard?.serviceStatus ?? "loading"}
          lastUpdated={dashboard?.lastUpdated ?? null}
          isRefreshing={isRefreshing}
        />

        {errorMessage ? (
          <div className="rounded-2xl border border-amber/20 bg-amber/10 px-4 py-3 text-sm text-amber-100 shadow-spike">
            <div className="flex items-center gap-3">
              <AlertCircle className="h-4 w-4" />
              <span>{dashboard ? `Using the last stable snapshot while refresh recovers. ${errorMessage}` : errorMessage}</span>
            </div>
          </div>
        ) : null}

        {isOfflineWithoutSnapshot ? (
          <div className="rounded-[30px] border border-amber/20 bg-panel px-6 py-8 shadow-panel">
            <div className="max-w-2xl">
              <p className="text-[0.68rem] uppercase tracking-[0.3em] text-amber-200/70">Connection Required</p>
              <h2 className="mt-3 font-display text-3xl font-semibold text-white">Mission control is ready. The live backend feed is not.</h2>
              <p className="mt-4 text-base leading-7 text-slate-300">
                Start the API and the dashboard will promote from standby into the live intelligence surface automatically.
              </p>
            </div>
          </div>
        ) : null}

        <section className="grid gap-5 2xl:grid-cols-[minmax(0,1.48fr)_minmax(380px,430px)]">
          <Suspense
            fallback={
              <div className="surface-sheen relative flex min-h-[500px] items-center justify-center rounded-[32px] border border-cyan/10 bg-[#050915] shadow-glow">
                <div className="grid-noise absolute inset-0" />
                <div className="rounded-2xl border border-white/10 bg-white/[0.03] px-5 py-4 text-sm tracking-[0.18em] text-cyan/75">
                  CALIBRATING GLOBE SURFACE
                </div>
              </div>
            }
          >
            <GlobeHero
              regions={dashboard?.regions ?? []}
              selectedRegionID={selectedRegionID}
              onSelectRegion={setSelectedRegionID}
              isLoading={isInitialLoading}
            />
          </Suspense>
          <RegionPanel region={selectedRegion} isLoading={isInitialLoading} />
        </section>

        <section className="grid gap-5 lg:grid-cols-2 2xl:grid-cols-[1.02fr_0.94fr_1.1fr]">
          {isInitialLoading && !dashboard ? (
            <>
              <LoadingPanel />
              <LoadingPanel />
              <LoadingPanel className="lg:col-span-2 2xl:col-span-1" />
            </>
          ) : (
            <>
              <LiveFeedPanel region={selectedRegion} isLoading={isInitialLoading} />
              <SpikeAlertsPanel region={selectedRegion} isLoading={isInitialLoading} />
              <TrendChartPanel region={selectedRegion} isLoading={isInitialLoading} className="lg:col-span-2 2xl:col-span-1" />
            </>
          )}
        </section>
      </div>
    </main>
  );
}

function LoadingPanel({ className = "" }: { className?: string }) {
  return (
    <div className={`rounded-[28px] border border-white/8 bg-panel p-5 shadow-panel sm:p-6 ${className}`}>
      <SkeletonBlock className="mb-4 h-5 w-36" />
      <div className="space-y-3">
        <SkeletonBlock className="h-16 w-full" />
        <SkeletonBlock className="h-16 w-full" />
        <SkeletonBlock className="h-28 w-full" />
      </div>
    </div>
  );
}
