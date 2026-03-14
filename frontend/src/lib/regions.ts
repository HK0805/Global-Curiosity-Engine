import type {
  CuriosityIndex,
  DashboardState,
  NormalizedEvent,
  RegionContribution,
  RegionDefinition,
  RegionEvent,
  RegionInsight,
  RegionSpike,
  RegionTopic,
  SourceBreakdown,
  SourceCount,
  TopicScore,
  TopicSpike,
  TrendPoint,
} from "../types";

const DEFAULT_SOURCE_WEIGHT = 1;
const TREND_BUCKETS = 7;
const TREND_WINDOW_MINUTES = 42;

export const REGION_DEFINITIONS: RegionDefinition[] = [
  {
    id: "north-america",
    label: "North America",
    lat: 40,
    lng: -100,
    summary: "Platform engineering, consumer tech, and open-source velocity.",
    accent: "#38bdf8",
    keywords: ["ai", "cloud", "openai", "startup", "developer", "software", "model", "github", "platform"],
    sourceBias: { github: 1.28, reddit: 1.12, hackernews: 1.14, wikipedia: 0.92, gdelt: 0.82 },
  },
  {
    id: "latin-america",
    label: "Latin America",
    lat: -15,
    lng: -60,
    summary: "Civic momentum, creator platforms, and emerging startup narratives.",
    accent: "#22d3ee",
    keywords: ["community", "civic", "creator", "media", "internet", "mobile", "consumer", "city"],
    sourceBias: { reddit: 1.18, gdelt: 1.04, wikipedia: 1.02, github: 0.78, hackernews: 0.84 },
  },
  {
    id: "europe",
    label: "Europe",
    lat: 52,
    lng: 15,
    summary: "Policy, science, and cross-border technology conversation.",
    accent: "#8b5cf6",
    keywords: ["policy", "research", "science", "regulation", "energy", "privacy", "infrastructure", "union"],
    sourceBias: { gdelt: 1.22, wikipedia: 1.08, hackernews: 0.92, github: 0.86, reddit: 0.82 },
  },
  {
    id: "africa",
    label: "Africa",
    lat: 6,
    lng: 20,
    summary: "Infrastructure, energy, mobility, and leapfrog innovation signals.",
    accent: "#14b8a6",
    keywords: ["energy", "mobility", "payments", "agriculture", "connectivity", "infrastructure", "network"],
    sourceBias: { gdelt: 1.2, wikipedia: 1.06, reddit: 0.88, github: 0.82, hackernews: 0.8 },
  },
  {
    id: "middle-east",
    label: "Middle East",
    lat: 28,
    lng: 45,
    summary: "Capital flows, geopolitical shifts, and strategic technology investment.",
    accent: "#f97316",
    keywords: ["capital", "security", "trade", "supply", "investment", "strategy", "oil", "defense"],
    sourceBias: { gdelt: 1.26, wikipedia: 1.02, github: 0.78, reddit: 0.8, hackernews: 0.84 },
  },
  {
    id: "south-asia",
    label: "South Asia",
    lat: 21,
    lng: 78,
    summary: "Developer scale, consumer internet, and accelerated digital adoption.",
    accent: "#60a5fa",
    keywords: ["developer", "mobile", "payments", "scale", "internet", "cloud", "growth", "saas"],
    sourceBias: { github: 1.08, reddit: 1.04, gdelt: 0.96, hackernews: 0.9, wikipedia: 0.9 },
  },
  {
    id: "east-asia",
    label: "East Asia",
    lat: 33,
    lng: 118,
    summary: "Advanced manufacturing, AI race dynamics, and media intensity.",
    accent: "#a855f7",
    keywords: ["manufacturing", "chips", "robotics", "ai", "supply", "hardware", "semiconductor", "factory"],
    sourceBias: { gdelt: 1.08, github: 1.02, wikipedia: 1, hackernews: 0.92, reddit: 0.82 },
  },
  {
    id: "oceania",
    label: "Oceania",
    lat: -25,
    lng: 135,
    summary: "Research, cloud infrastructure, and regional resilience trends.",
    accent: "#f59e0b",
    keywords: ["research", "climate", "cloud", "resilience", "ocean", "observatory", "science", "network"],
    sourceBias: { wikipedia: 1.08, gdelt: 1.02, github: 0.94, reddit: 0.9, hackernews: 0.86 },
  },
];

type RegionAssignment = {
  regionIndex: number;
  score: number;
};

type TopicCandidate = {
  topic: TopicScore;
  assignment: RegionAssignment;
};

type SpikeCandidate = {
  spike: TopicSpike;
  assignment: RegionAssignment;
};

type EventCandidate = {
  event: NormalizedEvent;
  assignment: RegionAssignment;
};

export function buildDashboardState(input: {
  curiosityIndex: CuriosityIndex | null;
  events: NormalizedEvent[];
  spikes: TopicSpike[];
  trending: TopicScore[];
  sources: SourceCount[];
  serviceStatus: string;
}): DashboardState {
  const topicCandidates = input.trending.map((topic) => ({
    topic,
    assignment: assignTopicToRegion(topic),
  }));
  const spikeCandidates = input.spikes.map((spike) => ({
    spike,
    assignment: assignSpikeToRegion(spike),
  }));
  const eventCandidates = input.events.map((event) => ({
    event,
    assignment: assignEventToRegion(event),
  }));

  const regions = REGION_DEFINITIONS.map((region, index) =>
    buildRegionInsight(region, index, topicCandidates, spikeCandidates, eventCandidates, input.sources, input.curiosityIndex),
  )
    .sort((left, right) => right.score - left.score)
    .map((region, index) => ({ ...region, rank: index + 1 }));

  return {
    curiosityIndex: input.curiosityIndex,
    events: input.events,
    spikes: input.spikes,
    trending: input.trending,
    sources: input.sources,
    regions,
    lastUpdated: new Date().toISOString(),
    serviceStatus: input.serviceStatus,
  };
}

function buildRegionInsight(
  region: RegionDefinition,
  regionIndex: number,
  topicCandidates: TopicCandidate[],
  spikeCandidates: SpikeCandidate[],
  eventCandidates: EventCandidate[],
  globalSources: SourceCount[],
  curiosityIndex: CuriosityIndex | null,
): RegionInsight {
  const regionTopics = topicCandidates
    .filter((candidate) => candidate.assignment.regionIndex === regionIndex)
    .sort((left, right) => regionTopicStrength(right) - regionTopicStrength(left))
    .slice(0, 5);

  const regionSpikes = spikeCandidates
    .filter((candidate) => candidate.assignment.regionIndex === regionIndex)
    .sort((left, right) => right.spike.current_score - left.spike.current_score)
    .slice(0, 4);

  const regionFeed = eventCandidates
    .filter((candidate) => candidate.assignment.regionIndex === regionIndex)
    .sort((left, right) => toMillis(right.event.timestamp) - toMillis(left.event.timestamp))
    .slice(0, 6);

  const topTopics = regionTopics.map<RegionTopic>((candidate) => {
    const sourceBreakdown = candidate.topic.metadata?.source_breakdown ?? [];
    const dominantSource = sourceBreakdown[0]?.source ?? "mixed";
    return {
      topic: candidate.topic.topic,
      score: Number(regionTopicStrength(candidate).toFixed(2)),
      mentions: candidate.topic.total_mentions,
      sources: candidate.topic.distinct_sources,
      sourceLead: dominantSource,
      signalLabel: describeTopicSignal(candidate.topic, regionSpikes.some((item) => item.spike.topic === candidate.topic.topic)),
    };
  });

  const spikes = regionSpikes.map<RegionSpike>((candidate) => {
    const matchingTopic = regionTopics.find((topic) => topic.topic.topic === candidate.spike.topic);
    return {
      id: candidate.spike.id,
      topic: candidate.spike.topic,
      spikeType: candidate.spike.spike_type,
      currentScore: Number((candidate.spike.current_score * candidate.assignment.score).toFixed(2)),
      timestamp: candidate.spike.timestamp,
      sourceLead: matchingTopic?.topic.metadata?.source_breakdown?.[0]?.source ?? "mixed",
    };
  });

  const feed = regionFeed.map<RegionEvent>((candidate) => ({
    id: candidate.event.id,
    title: candidate.event.title,
    source: candidate.event.source,
    timestamp: candidate.event.timestamp,
    url: candidate.event.url,
  }));

  const contributions = buildContributions(region, regionTopics, regionFeed, globalSources);
  const spikePressure = Number(
    regionSpikes.reduce((sum, candidate) => sum + candidate.spike.current_score * candidate.assignment.score, 0).toFixed(2),
  );
  const activity = Math.round(
    topTopics.reduce((sum, topic) => sum + topic.mentions, 0) +
      spikes.length * 6 +
      feed.length * 2 +
      contributions.reduce((sum, item) => sum + item.value, 0) * 0.35,
  );
  const score = Number(
    (
      topTopics.reduce((sum, topic) => sum + topic.score, 0) * 0.9 +
      spikePressure * 0.55 +
      feed.length * 0.7 +
      (curiosityIndex?.curiosity_index ?? 0) * 0.04
    ).toFixed(2),
  );

  const hotspotLabel = topTopics[0]?.topic ?? deriveFallbackHotspot(regionFeed, regionSpikes, region.label);
  const dominantSource = contributions[0]?.source ?? "mixed";
  const topicDiversity = new Set(topTopics.map((topic) => topic.topic)).size;

  return {
    region,
    score,
    activity,
    hotspotLabel,
    narrative: buildNarrative(region, topTopics, spikes, dominantSource, feed),
    dominantSource,
    topicDiversity,
    spikePressure,
    topTopics,
    spikes,
    feed,
    contributions,
    trend: buildTrend(regionTopics, regionSpikes, regionFeed, curiosityIndex),
  };
}

function buildContributions(
  region: RegionDefinition,
  topics: TopicCandidate[],
  events: EventCandidate[],
  globalSources: SourceCount[],
): RegionContribution[] {
  const totals = new Map<string, number>();

  topics.forEach((candidate) => {
    const breakdown = candidate.topic.metadata?.source_breakdown ?? [];
    if (breakdown.length === 0) {
      totals.set("mixed", (totals.get("mixed") ?? 0) + regionTopicStrength(candidate) * 0.3);
      return;
    }

    breakdown.forEach((item) => {
      const source = item.source;
      const baseWeight = region.sourceBias?.[source] ?? DEFAULT_SOURCE_WEIGHT;
      totals.set(source, (totals.get(source) ?? 0) + item.mentions * baseWeight);
    });
  });

  events.forEach((candidate) => {
    const source = candidate.event.source;
    const baseWeight = region.sourceBias?.[source] ?? DEFAULT_SOURCE_WEIGHT;
    totals.set(source, (totals.get(source) ?? 0) + candidate.assignment.score * baseWeight * 1.4);
  });

  globalSources.forEach((item) => {
    const baseWeight = region.sourceBias?.[item.source] ?? DEFAULT_SOURCE_WEIGHT;
    totals.set(item.source, (totals.get(item.source) ?? 0) + item.count * 0.05 * baseWeight);
  });

  const raw = Array.from(totals.entries())
    .map(([source, value]) => ({ source, value: Number(value.toFixed(2)) }))
    .sort((left, right) => right.value - left.value)
    .slice(0, 4);

  const totalValue = raw.reduce((sum, item) => sum + item.value, 0);
  return raw.map((item) => ({
    ...item,
    share: totalValue > 0 ? Number(((item.value / totalValue) * 100).toFixed(1)) : 0,
  }));
}

function buildTrend(
  topics: TopicCandidate[],
  spikes: SpikeCandidate[],
  events: EventCandidate[],
  curiosityIndex: CuriosityIndex | null,
): TrendPoint[] {
  const now = Date.now();
  const bucketWidthMs = (TREND_WINDOW_MINUTES / TREND_BUCKETS) * 60 * 1000;
  const buckets = Array.from({ length: TREND_BUCKETS }, (_, index) => ({
    label: `${(TREND_BUCKETS - index - 1) * 6}m`,
    value: 0,
  }));

  topics.forEach((candidate) => {
    accumulateBucket(buckets, now, candidate.topic.timestamp, bucketWidthMs, regionTopicStrength(candidate));
  });

  spikes.forEach((candidate) => {
    accumulateBucket(buckets, now, candidate.spike.timestamp, bucketWidthMs, candidate.spike.current_score * 0.45);
  });

  events.forEach((candidate) => {
    accumulateBucket(buckets, now, candidate.event.timestamp, bucketWidthMs, 1.35 * candidate.assignment.score);
  });

  return buckets.map((bucket, index) => ({
    label: index === TREND_BUCKETS - 1 ? "now" : bucket.label,
    value: Number(Math.max(0.8, bucket.value + (curiosityIndex?.curiosity_index ?? 0) * 0.01).toFixed(2)),
  }));
}

function accumulateBucket(
  buckets: TrendPoint[],
  now: number,
  timestamp: string,
  bucketWidthMs: number,
  weight: number,
) {
  const ageMs = Math.max(0, now - toMillis(timestamp));
  const rawIndex = Math.floor(ageMs / bucketWidthMs);
  const bucketIndex = Math.min(buckets.length - 1, Math.max(0, buckets.length - 1 - rawIndex));
  buckets[bucketIndex].value += weight;
}

function assignTopicToRegion(topic: TopicScore): RegionAssignment {
  const sourceBreakdown = topic.metadata?.source_breakdown ?? [];
  const sourceText = sourceBreakdown.map((item) => item.source).join(" ");
  const seedText = `${topic.topic} ${sourceText}`.trim();
  return assignToRegion(seedText, sourceBreakdown);
}

function assignSpikeToRegion(spike: TopicSpike): RegionAssignment {
  return assignToRegion(spike.topic, []);
}

function assignEventToRegion(event: NormalizedEvent): RegionAssignment {
  const sourceBreakdown: SourceBreakdown[] = [{ source: event.source, mentions: 1 }];
  return assignToRegion(`${event.title} ${event.source}`, sourceBreakdown);
}

function assignToRegion(seedText: string, sourceBreakdown: SourceBreakdown[]): RegionAssignment {
  let bestIndex = 0;
  let bestScore = -1;

  REGION_DEFINITIONS.forEach((region, index) => {
    const keywordScore = scoreKeywords(seedText, region.keywords ?? []);
    const sourceScore = scoreSources(sourceBreakdown, region);
    const fallback = deterministicBias(`${seedText}:${region.id}`) * 0.2;
    const total = keywordScore * 1.75 + sourceScore + fallback;

    if (total > bestScore) {
      bestScore = total;
      bestIndex = index;
    }
  });

  return {
    regionIndex: bestIndex,
    score: Number(Math.max(0.65, bestScore).toFixed(2)),
  };
}

function scoreKeywords(seedText: string, keywords: string[]): number {
  const tokens = tokenize(seedText);
  if (tokens.length === 0 || keywords.length === 0) {
    return 0;
  }

  let score = 0;
  keywords.forEach((keyword) => {
    if (tokens.includes(keyword)) {
      score += 1.25;
    } else if (tokens.some((token) => token.includes(keyword) || keyword.includes(token))) {
      score += 0.55;
    }
  });
  return score;
}

function scoreSources(sourceBreakdown: SourceBreakdown[], region: RegionDefinition): number {
  if (sourceBreakdown.length === 0) {
    return 0.35;
  }

  return sourceBreakdown.reduce((sum, item) => {
    const bias = region.sourceBias?.[item.source] ?? DEFAULT_SOURCE_WEIGHT;
    return sum + item.mentions * bias;
  }, 0);
}

function regionTopicStrength(candidate: TopicCandidate): number {
  return candidate.topic.score * candidate.assignment.score;
}

function buildNarrative(
  region: RegionDefinition,
  topics: RegionTopic[],
  spikes: RegionSpike[],
  dominantSource: string,
  feed: RegionEvent[],
): string {
  const leadTopic = topics[0]?.topic;
  const secondaryTopic = topics[1]?.topic;
  const spikeLabel = spikes[0]?.topic;

  if (leadTopic && spikeLabel) {
    return `${capitalize(leadTopic)} is leading the regional conversation while ${capitalize(
      spikeLabel,
    )} is showing the sharpest acceleration, with ${dominantSource} driving the loudest signal.`;
  }

  if (leadTopic && secondaryTopic) {
    return `${capitalize(leadTopic)} and ${capitalize(
      secondaryTopic,
    )} are shaping this theater, with ${dominantSource} contributing the most visible pressure right now.`;
  }

  if (leadTopic) {
    return `${capitalize(leadTopic)} is the clearest signal in ${region.label}, reinforced by ${dominantSource} and the latest live events.`;
  }

  if (feed[0]) {
    return `Live intake is still sparse, but ${dominantSource} is currently the strongest contributor to ${region.label}'s signal map.`;
  }

  return region.summary;
}

function deriveFallbackHotspot(events: EventCandidate[], spikes: SpikeCandidate[], fallback: string): string {
  if (spikes[0]?.spike.topic) {
    return spikes[0].spike.topic;
  }
  if (events[0]?.event.title) {
    return events[0].event.title.split(/\s+/)[0] ?? fallback;
  }
  return fallback;
}

function describeTopicSignal(topic: TopicScore, hasSpike: boolean): string {
  if (hasSpike) {
    return "spiking";
  }
  if (topic.distinct_sources >= 3) {
    return "cross-source";
  }
  if (topic.total_mentions >= 8) {
    return "high-volume";
  }
  return "building";
}

function tokenize(value: string): string[] {
  return value
    .toLowerCase()
    .split(/[^a-z0-9]+/)
    .map((token) => token.trim())
    .filter(Boolean);
}

function deterministicBias(seed: string): number {
  let hash = 0;
  for (let index = 0; index < seed.length; index += 1) {
    hash = (hash * 33 + seed.charCodeAt(index)) % 2147483647;
  }
  return (hash % 1000) / 1000;
}

function toMillis(timestamp: string): number {
  const parsed = new Date(timestamp).getTime();
  return Number.isNaN(parsed) ? Date.now() : parsed;
}

function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
