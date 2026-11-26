import { createFileRoute } from "@tanstack/react-router";

import { ConversationAttributePage } from "@/features/settings/pages/conversation-attribute-page";

export const Route = createFileRoute("/d/settings/conversation-attributes/$attribute")({
  component: ConversationAttributeRoute,
});

function ConversationAttributeRoute() {
  const { attribute } = Route.useParams();

  return <ConversationAttributePage key={attribute} attribute={attribute} />;
}
