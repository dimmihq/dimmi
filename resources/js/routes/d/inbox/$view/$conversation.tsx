import { RiChatOffLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";

import { ConversationView } from "@/features/inbox/conversation-view";
import { useInbox } from "@/features/inbox/conversations";
import { ConversationSkeleton } from "@/features/inbox/skeletons";
import { EmptyState, Panel } from "@/features/shell/page";
import { ApiError } from "@/lib/api/client";
import { conversationQuery } from "@/lib/api/inbox";

export const Route = createFileRoute("/d/inbox/$view/$conversation")({
  component: ConversationRoute,
});

function ConversationRoute() {
  const { view, conversation: id } = Route.useParams();
  const { workspace } = useInbox();
  const {
    data: conversation,
    isPending,
    error,
  } = useQuery({
    ...conversationQuery(workspace, id),
    enabled: workspace !== "",
    retry: false,
  });

  const gone = error instanceof ApiError && error.status === 404;

  if (conversation && !gone) {
    return <ConversationView key={conversation.id} view={view} workspace={workspace} conversation={conversation} />;
  }

  return (
    <Panel label="Conversation" className="flex-1">
      {isPending ? (
        <ConversationSkeleton />
      ) : (
        <EmptyState
          icon={RiChatOffLine}
          title="You do not have access to this conversation"
          description="It may have been deleted, or it may not be yours to see."
        >
          <Link to="/d/inbox/$view" params={{ view: "mine" }} className="text-body-medium text-text-primary underline">
            Go to my inbox
          </Link>
        </EmptyState>
      )}
    </Panel>
  );
}
