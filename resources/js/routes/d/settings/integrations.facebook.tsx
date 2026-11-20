import { createFileRoute } from "@tanstack/react-router";

import { SocialSettings } from "@/features/settings/pages/social";

export const Route = createFileRoute("/d/settings/integrations/facebook")({
  component: () => <SocialSettings platform="facebook" />,
});
