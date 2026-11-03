import { createFileRoute } from "@tanstack/react-router";

import { NoConversation } from "@/features/inbox/inbox-page";

export const Route = createFileRoute("/d/inbox/$view/")({
  component: NoConversation,
});
