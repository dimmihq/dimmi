import { createFileRoute } from "@tanstack/react-router";

import { SocialDetail } from "@/features/settings/pages/social";

export const Route = createFileRoute("/d/settings/integrations/facebook_/$channel")({
  component: Detail,
});

function Detail() {
  const { channel } = Route.useParams();

  return <SocialDetail platform="facebook" channel={channel} />;
}
