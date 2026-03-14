import { startTransition } from "react";
import { buildDashboardState } from "./regions";
import type {
  CuriosityIndex,
  DashboardState,
  HealthResponse,
  NormalizedEvent,
  SourceCount,
  TopicScore,
  TopicSpike,
} from "../types";

const API_BASE = "/api";

export async function fetchDashboardState(): Promise<DashboardState> {
  const [health, events, spikes, trending, sources, curiosityIndex] = await Promise.all([
    requestJSON<HealthResponse>("/health"),
    requestJSON<NormalizedEvent[]>("/events?limit=12"),
    requestJSON<TopicSpike[]>("/topics/spikes?limit=8"),
    requestJSON<TopicScore[]>("/topics/trending?limit=12"),
    requestJSON<SourceCount[]>("/sources"),
    requestJSON<CuriosityIndex>("/curiosity/index"),
  ]);

  return buildDashboardState({
    curiosityIndex,
    events,
    spikes,
    trending,
    sources,
    serviceStatus: health.status,
  });
}

export function pollDashboardState(onData: (state: DashboardState) => void, onError: (message: string) => void) {
  let active = true;

  const load = async () => {
    try {
      const state = await fetchDashboardState();
      if (!active) {
        return;
      }
      startTransition(() => onData(state));
    } catch (error) {
      if (!active) {
        return;
      }
      onError(error instanceof Error ? error.message : "Unknown dashboard error");
    }
  };

  void load();
  const intervalID = window.setInterval(() => {
    void load();
  }, 30000);

  return () => {
    active = false;
    window.clearInterval(intervalID);
  };
}

async function requestJSON<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`);
  if (!response.ok) {
    throw new Error(`${path} failed with status ${response.status}`);
  }
  return (await response.json()) as T;
}
