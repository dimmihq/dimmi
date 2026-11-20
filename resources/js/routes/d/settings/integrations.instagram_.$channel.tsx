import { createFileRoute } from "@tanstack/react-router";

import { SocialDetail } from "@/features/settings/pages/social";

export const Route = createFileRoute("/d/settings/integrations/instagram_/$channel")({
  component: Detail,
});

function Detail() {
  const { channel } = Route.useParams();

  return <SocialDetail platform="instagram" channel={channel} />;
}
