import { createFileRoute } from "@tanstack/react-router";

import { TeamSettings } from "@/features/settings/pages/team";

export const Route = createFileRoute("/d/settings/teams/$team")({
  component: TeamRoute,
});

function TeamRoute() {
  const { team } = Route.useParams();

  return <TeamSettings key={team} team={team} />;
}
