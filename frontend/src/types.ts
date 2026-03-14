export type HealthResponse = {
  status: string;
  service: string;
};

export type SourceCount = {
  source: string;
  count: number;
};

export type NormalizedEvent = {
  id: string;
  source: string;
  title: string;
  url?: string;
  timestamp: string;
  metadata?: Record<string, unknown>;
};

export type SourceBreakdown = {
  source: string;
  mentions: number;
  weight?: number;
};

export type TopicScore = {
  id: string;
  topic: string;
  score: number;
  distinct_sources: number;
  total_mentions: number;
  timestamp: string;
  metadata?: {
    weighted_sum?: number;
    source_breakdown?: SourceBreakdown[];
    formula_version?: string;
  };
};

export type TopicSpike = {
  id: string;
  topic: string;
  spike_type: "initial_surge" | "score_jump";
  previous_score: number;
  current_score: number;
  increase_ratio: number;
  increase_absolute: number;
  distinct_sources: number;
  total_mentions: number;
  timestamp: string;
  metadata?: Record<string, unknown>;
};

export type CuriosityIndex = {
  curiosity_index: number;
  topics_considered: number;
  generated_at: string;
};

export type RegionDefinition = {
  id: string;
  label: string;
  lat: number;
  lng: number;
  summary: string;
  accent: string;
  keywords?: string[];
  sourceBias?: Partial<Record<string, number>>;
};

export type RegionTopic = {
  topic: string;
  score: number;
  mentions: number;
  sources: number;
  sourceLead?: string;
  signalLabel?: string;
};

export type RegionSpike = {
  id: string;
  topic: string;
  spikeType: string;
  currentScore: number;
  timestamp: string;
  sourceLead?: string;
};

export type RegionEvent = {
  id: string;
  title: string;
  source: string;
  timestamp: string;
  url?: string;
};

export type RegionContribution = {
  source: string;
  value: number;
  share: number;
};

export type TrendPoint = {
  label: string;
  value: number;
};

export type RegionInsight = {
  region: RegionDefinition;
  score: number;
  activity: number;
  rank?: number;
  hotspotLabel: string;
  narrative: string;
  dominantSource: string;
  topicDiversity: number;
  spikePressure: number;
  topTopics: RegionTopic[];
  spikes: RegionSpike[];
  feed: RegionEvent[];
  contributions: RegionContribution[];
  trend: TrendPoint[];
};

export type DashboardState = {
  curiosityIndex: CuriosityIndex | null;
  events: NormalizedEvent[];
  spikes: TopicSpike[];
  trending: TopicScore[];
  sources: SourceCount[];
  regions: RegionInsight[];
  lastUpdated: string | null;
  serviceStatus: string;
};
