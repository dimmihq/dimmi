/*
 * Operator beside the workflow builder: the shared Operator panel on the
 * teammate's workspace threads, with suggestions for the open workflow.
 * The panel says the workflow is open, with the suggestion waiting on
 * it, so every question carries the workflow in full and a change
 * Operator makes is added to that suggestion. The suggestion shows in
 * the builder as soon as Operator makes it (see workflow-review.ts).
 */
import { RiFlowChart, RiPulseLine, RiRouteLine, RiSearchEyeLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { OperatorSidePanel } from "@/features/operator/operator-side-panel";
import { useOperatorChat } from "@/features/operator/use-operator-chat";
import { useViewing } from "@/features/operator/viewing";
import {
  operatorKeys,
  operatorSettingsQuery,
  operatorThreadQuery,
  startWorkspaceThread,
  workspaceThreadQuery,
  workspaceThreadsQuery,
} from "@/lib/api/operator";
import { workflowQuery } from "@/lib/api/workflows";

import { useWorkflowRequest } from "./workflow-review";

export function WorkflowOperatorPanel({
  workspace,
  workflow,
  onClose,
  className,
}: {
  workspace: string;
  workflow: string;
  onClose?: () => void;
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
  const { data: shown } = useQuery(workflowQuery(workspace, workflow));
  const { request } = useWorkflowRequest(workspace, workflow, shown?.spec);
  useViewing("workflow", workflow, request?.changeset.id);
  const chat = useOperatorChat(workspace, thread, key);
  const fresh = useMutation({
    mutationFn: () => startWorkspaceThread(workspace),
    onSuccess: (started) => {
      client.setQueryData(operatorKeys.thread(workspace, started.id), started);
      setPicked(started.id);
    },
  });
  const name = shown?.name || "this workflow";

  return (
    <OperatorSidePanel
      thread={thread}
      chat={chat}
      past={past}
      enabled={settings?.enabled ?? true}
      starting={fresh.isPending}
      heading="Work on this workflow with Operator"
      blurb="Operator reads this workflow and its runs, explains what it did, and suggests changes you accept in the builder."
      suggestions={[
        {
          icon: RiPulseLine,
          title: "Explain a recent run",
          detail: "Says why it ran, or did not, on a conversation.",
          onPick: () =>
            chat.ask(
              `Explain what the workflow "${name}" did on its most recent conversations, and why any step went ` +
                "a way we might not expect.",
            ),
        },
        {
          icon: RiSearchEyeLine,
          title: "Review it for gaps",
          detail: "Finds paths that end nowhere and cases it misses.",
          onPick: () =>
            chat.ask(
              `Review the workflow "${name}" for paths that end without an owner, cases it misses and warnings, ` +
                "and suggest fixes.",
            ),
        },
        {
          icon: RiRouteLine,
          title: "Change a step",
          detail: "Describe it, and Operator edits the steps.",
          onPick: () => chat.ask(`What would you change in the workflow "${name}" first, and why?`),
        },
      ]}
      placeholder="Ask Operator about this workflow"
      inputId="workflow-operator-composer-input"
      scope={{ icon: RiFlowChart, label: shown?.name || "This workflow" }}
      onPick={setPicked}
      onNew={() => fresh.mutate()}
      onClose={onClose ?? (() => undefined)}
      className={className}
    />
  );
}
