/*
 * Operator beside the article: the teammate's own Operator chat, told
 * which article is open and which suggested change is shown on it. They
 * can ask why a change was suggested, ask for it to be changed, or ask
 * for something new; what Operator changes shows in the article as
 * struck-through and green lines, to accept from the header.
 */
import { RiArticleLine, RiEditLine, RiQuestionLine, RiSearchEyeLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { OperatorSidePanel } from "@/features/operator/operator-side-panel";
import { useOperatorChat } from "@/features/operator/use-operator-chat";
import {
  operatorKeys,
  operatorSettingsQuery,
  operatorThreadQuery,
  startWorkspaceThread,
  workspaceThreadQuery,
  workspaceThreadsQuery,
} from "@/lib/api/operator";

export function ArticleOperatorPanel({
  workspace,
  articleId,
  articleTitle,
  suggestion,
  onClose,
  className,
}: {
  workspace: string;
  articleId: string;
  articleTitle: string;
  /* The suggested change shown on the article, when one waits. */
  suggestion: string | null;
  onClose: () => void;
  className?: string;
}) {
  const client = useQueryClient();
  const [picked, setPicked] = useState<string | null>(null);
  const latest = useQuery({ ...workspaceThreadQuery(workspace), enabled: picked === null });
  const chosen = useQuery({ ...operatorThreadQuery(workspace, picked ?? ""), enabled: picked !== null });
  const thread = picked === null ? latest.data : chosen.data;
  const key =
    picked === null
      ? ([...operatorKeys.threads(workspace), "latest"] as const)
      : operatorKeys.thread(workspace, picked);
  const { data: past = [] } = useQuery(workspaceThreadsQuery(workspace));
  const { data: settings } = useQuery(operatorSettingsQuery(workspace));
  const chat = useOperatorChat(workspace, thread, key, {
    article_id: articleId,
    changeset_id: suggestion ?? undefined,
  });
  const fresh = useMutation({
    mutationFn: () => startWorkspaceThread(workspace),
    onSuccess: (started) => {
      client.setQueryData(operatorKeys.thread(workspace, started.id), started);
      setPicked(started.id);
    },
  });
  const reviewing = [
    {
      icon: RiQuestionLine,
      title: "Explain this suggestion",
      detail: "Why it was made and what it changes.",
      onPick: () => chat.ask("Why was this change suggested? Explain what it changes and why."),
    },
    {
      icon: RiSearchEyeLine,
      title: "Check it against other articles",
      detail: "Makes sure the new text agrees with the rest of the help center.",
      onPick: () => chat.ask("Check that this suggested change agrees with our other help articles."),
    },
  ];
  const writing = [
    {
      icon: RiSearchEyeLine,
      title: "Check this article for mistakes",
      detail: "Finds facts that disagree with other articles or went stale.",
      onPick: () => chat.ask("Check this article for mistakes and stale facts, and suggest fixes."),
    },
    {
      icon: RiEditLine,
      title: "Make it easier to read",
      detail: "Shorter sentences and plainer words, same facts.",
      onPick: () => chat.ask("Make this article easier to read without changing its facts."),
    },
  ];

  return (
    <OperatorSidePanel
      thread={thread}
      chat={chat}
      past={past}
      enabled={settings?.enabled ?? true}
      starting={fresh.isPending}
      heading="Work on this article with Operator"
      blurb={
        suggestion
          ? "Ask why this change was suggested, or ask Operator to change it. What it changes shows in the article for you to accept."
          : "Ask Operator to check or change this article. What it changes shows in the article for you to accept."
      }
      suggestions={suggestion ? reviewing : writing}
      placeholder="Ask Operator about this article"
      inputId="article-operator-composer-input"
      scope={{ icon: RiArticleLine, label: articleTitle || "This article" }}
      onPick={setPicked}
      onNew={() => fresh.mutate()}
      onClose={onClose}
      className={className}
    />
  );
}
