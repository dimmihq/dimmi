/*
 * Operator beside Agent Studio: the shared Operator panel on the
 * teammate's workspace threads, with suggestions for the open agent.
 * Nothing said here reaches customers.
 */
import { RiQuestionAnswerLine, RiRobot2Line, RiTestTubeLine } from "@remixicon/react";
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

export function AgentOperatorPanel({
  workspace,
  agentName,
  onClose,
  className,
}: {
  workspace: string;
  agentName: string;
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
  const chat = useOperatorChat(workspace, thread, key);
  const fresh = useMutation({
    mutationFn: () => startWorkspaceThread(workspace),
    onSuccess: (started) => {
      client.setQueryData(operatorKeys.thread(workspace, started.id), started);
      setPicked(started.id);
    },
  });

  return (
    <OperatorSidePanel
      thread={thread}
      chat={chat}
      past={past}
      enabled={settings?.enabled ?? true}
      starting={fresh.isPending}
      heading={`Work on ${agentName} with Operator`}
      blurb="Operator reads this agent's draft, its simulations and recent conversations, and proposes changes for you to apply."
      suggestions={[
        {
          icon: RiTestTubeLine,
          title: "Write simulations from handovers",
          detail: "Turns last week's handovers into saved simulations.",
          onPick: () => chat.ask(`Write simulations for the ${agentName} agent from last week's handovers.`),
        },
        {
          icon: RiQuestionAnswerLine,
          title: "Explain a failing simulation",
          detail: "Reads the run and proposes a fix to the draft.",
          onPick: () => chat.ask(`Why does the failing simulation of the ${agentName} agent fail?`),
        },
      ]}
      placeholder={`Ask Operator about ${agentName}`}
      inputId="agent-operator-composer-input"
      scope={{ icon: RiRobot2Line, label: agentName }}
      onPick={setPicked}
      onNew={() => fresh.mutate()}
      onClose={onClose}
      className={className}
    />
  );
}
