import { RiChatOffLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { Spinner } from "@/components/base/spinner/spinner";
import { ConversationView } from "@/features/inbox/conversation-view";
import { useInbox } from "@/features/inbox/conversations";
import { EmptyState, Panel } from "@/features/shell/page";
import { conversationQuery } from "@/lib/api/inbox";

export const Route = createFileRoute("/d/inbox/$view/$conversation")({
  component: ConversationRoute,
});

function ConversationRoute() {
  const { view, conversation: id } = Route.useParams();
  const { workspace } = useInbox();
  const { data: conversation, isPending } = useQuery({
    ...conversationQuery(workspace, id),
    enabled: workspace !== "",
    retry: false,
  });

  if (conversation) {
    return <ConversationView key={conversation.id} view={view} workspace={workspace} conversation={conversation} />;
  }

  return (
    <Panel label="Conversation" className="flex-1">
      {isPending ? (
        <div className="flex flex-1 items-center justify-center">
          <Spinner aria-label="Loading the conversation" />
        </div>
      ) : (
        <EmptyState
          icon={RiChatOffLine}
          title="Conversation not found"
          description="It may have been deleted, or it belongs to another workspace."
        />
      )}
    </Panel>
  );
}
