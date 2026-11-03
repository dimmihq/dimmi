import { createFileRoute, notFound } from "@tanstack/react-router";

import { ConversationView } from "@/features/inbox/conversation-view";
import { findConversation } from "@/features/inbox/conversations";

export const Route = createFileRoute("/d/inbox/$view/$conversation")({
  loader: ({ params }) => {
    const conversation = findConversation(params.conversation);
    if (!conversation) {
      throw notFound();
    }

    return conversation;
  },
  component: ConversationRoute,
});

function ConversationRoute() {
  const { view } = Route.useParams();
  const conversation = Route.useLoaderData();

  return <ConversationView key={conversation.id} view={view} conversation={conversation} />;
}
