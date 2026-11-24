/*
 * Operator beside the procedure editor: the shared Operator panel on the
 * teammate's workspace threads, with suggestions for the open procedure:
 * review its recent runs, review it for gaps, or suggest tests. Nothing
 * said here reaches customers, and any proposed changeset lands as a
 * reviewable card right in the thread.
 */
import { RiListOrdered2, RiPulseLine, RiSearchEyeLine, RiTestTubeLine } from "@remixicon/react";
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

export function ProcedureOperatorPanel({
  workspace,
  procedureId,
  procedureName,
  onClose,
  className,
}: {
  workspace: string;
  procedureId: string;
  procedureName: string;
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
  const about = `procedure "${procedureName || "this procedure"}" (${procedureId})`;

  return (
    <OperatorSidePanel
      thread={thread}
      chat={chat}
      past={past}
      enabled={settings?.enabled ?? true}
      starting={fresh.isPending}
      heading="Work on this procedure with Operator"
      blurb="Operator reads this procedure, inspects recent runs and proposes edits or test cases for you to apply."
      suggestions={[
        {
          icon: RiPulseLine,
          title: "Review recent runs",
          detail: "Reads the runs that went wrong and proposes fixes with regression tests.",
          onPick: () =>
            chat.ask(
              `Review the recent runs of the ${about}: call list_procedure_anomalies, read the traces of the runs ` +
                "that went wrong, and propose fixes to its working copy with regression tests made from the failing " +
                "conversations.",
            ),
        },
        {
          icon: RiSearchEyeLine,
          title: "Review this procedure for gaps",
          detail: "Finds unclear steps and missing edge cases.",
          onPick: () => chat.ask(`Review ${about} and suggest any unclear steps, missing edge cases, or improvements.`),
        },
        {
          icon: RiTestTubeLine,
          title: "Suggest test cases",
          detail: "Proposes saved tests for the main branches and edge cases.",
          onPick: () =>
            chat.ask(`Read ${about} and propose saved test cases covering its main branches and edge cases.`),
        },
      ]}
      placeholder="Ask Operator about this procedure"
      inputId="procedure-operator-composer-input"
      scope={{ icon: RiListOrdered2, label: procedureName || "This procedure" }}
      onPick={setPicked}
      onNew={() => fresh.mutate()}
      onClose={onClose}
      className={className}
    />
  );
}
