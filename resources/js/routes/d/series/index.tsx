import { createFileRoute } from "@tanstack/react-router";

import { SeriesList } from "@/features/series/series-list";

export const Route = createFileRoute("/d/series/")({
  component: SeriesList,
});
