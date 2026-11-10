import { createFileRoute } from "@tanstack/react-router";

import { SeriesBuilder } from "@/features/series/series-builder";

export const Route = createFileRoute("/d/series/$series")({
  component: SeriesRoute,
});

function SeriesRoute() {
  const { series } = Route.useParams();

  return <SeriesBuilder key={series} id={series} />;
}
