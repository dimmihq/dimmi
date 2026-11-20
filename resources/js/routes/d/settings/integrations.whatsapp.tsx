import { createFileRoute } from "@tanstack/react-router";

import { WhatsAppSettings } from "@/features/settings/pages/whatsapp";

export const Route = createFileRoute("/d/settings/integrations/whatsapp")({
  component: WhatsAppSettings,
});
