import { createFileRoute } from "@tanstack/react-router";

import { SeriesComposePage } from "@/features/series/compose-page";

export const Route = createFileRoute("/d/series/$series_/compose/$step")({
  component: SeriesComposeRoute,
});

function SeriesComposeRoute() {
  const { series, step } = Route.useParams();

  return <SeriesComposePage key={`${series}:${step}`} id={series} step={step} />;
}
