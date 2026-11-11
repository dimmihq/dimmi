import { createFileRoute } from "@tanstack/react-router";

import { SeriesBuilder } from "@/features/series/series-builder";

export const Route = createFileRoute("/d/series/$series")({
  validateSearch: (search: Record<string, unknown>): { step?: string } =>
    typeof search.step === "string" ? { step: search.step } : {},
  component: SeriesRoute,
});

function SeriesRoute() {
  const { series } = Route.useParams();
  const { step } = Route.useSearch();

  return <SeriesBuilder key={series} id={series} step={step} />;
}
