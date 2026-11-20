import { createFileRoute } from "@tanstack/react-router";

import { WhatsAppTemplatePage } from "@/features/settings/pages/whatsapp-template";

export const Route = createFileRoute("/d/settings/integrations/whatsapp_/$channel_/templates/$template")({
  component: WhatsAppTemplateRoute,
});

function WhatsAppTemplateRoute() {
  const { channel, template } = Route.useParams();

  return <WhatsAppTemplatePage key={`${channel}-${template}`} channel={channel} template={template} />;
}
