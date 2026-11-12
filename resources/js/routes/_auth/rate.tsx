import { createFileRoute } from "@tanstack/react-router";

import { RatePage } from "@/features/ratings/rate-page";

interface RateSearch {
  t?: string;
  score?: number;
}

export const Route = createFileRoute("/_auth/rate")({
  validateSearch: (search: Record<string, unknown>): RateSearch => {
    const score = Number(search.score);

    return {
      t: typeof search.t === "string" ? search.t : undefined,
      score: Number.isInteger(score) && score >= 1 && score <= 5 ? score : undefined,
    };
  },
  component: RateRoute,
});

function RateRoute() {
  const { t, score } = Route.useSearch();

  return <RatePage token={t ?? ""} picked={score} />;
}
