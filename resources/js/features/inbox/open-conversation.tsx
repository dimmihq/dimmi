/*
 * A conversation opened in the inbox by its address: its view once it
 * loads, a skeleton while it loads or follows a merge, and a way back
 * when it does not load or is not the teammate's to see.
 */
import { RiChatOffLine, RiWifiOffLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { Button } from "@/components/base/buttons/button";
import { EmptyState, Panel } from "@/features/shell/page";
import { ApiError } from "@/lib/api/client";
import { conversationQuery } from "@/lib/api/inbox";

import { ConversationView } from "./conversation-view";
import { useInbox } from "./conversations";
import { ConversationSkeleton } from "./skeletons";
import { useFollowMerge } from "./use-follow-merge";

/*
 * OpenConversation shows the conversation the address names.
 */
export function OpenConversation({ view, id }: { view: string; id: string }) {
  const { workspace } = useInbox();
  const {
    data: conversation,
    isPending,
    error,
    refetch,
    isFetching,
  } = useQuery({
    ...conversationQuery(workspace, id),
    enabled: workspace !== "",
    retry: false,
  });

  const following = useFollowMerge(view, error);
  const gone = error instanceof ApiError && (error.status === 404 || error.status === 403);

  if (conversation && !gone) {
    return <ConversationView key={conversation.id} view={view} workspace={workspace} conversation={conversation} />;
  }

  return (
    <Panel label="Conversation" className="flex-1">
      {isPending || following ? (
        <ConversationSkeleton />
      ) : error && !gone ? (
        <EmptyState
          icon={RiWifiOffLine}
          title="This conversation did not load"
          description="Check your connection and try again."
        >
          <Button size="small" variant="secondary" loading={isFetching} onClick={() => void refetch()}>
            Try again
          </Button>
        </EmptyState>
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
