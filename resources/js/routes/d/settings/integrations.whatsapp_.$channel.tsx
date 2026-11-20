import { createFileRoute } from "@tanstack/react-router";

import { WhatsAppNumberPage } from "@/features/settings/pages/whatsapp-detail";

export const Route = createFileRoute("/d/settings/integrations/whatsapp_/$channel")({
  component: WhatsAppNumberRoute,
});

function WhatsAppNumberRoute() {
  const { channel } = Route.useParams();

  return <WhatsAppNumberPage key={channel} channel={channel} />;
}
