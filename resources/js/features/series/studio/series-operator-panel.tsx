/*
 * Operator beside the series builder: the shared Operator panel on the
 * teammate's workspace threads, with suggestions for the open series.
 * The panel says the series is open, with the suggestion waiting on it,
 * so every question carries the series in full and a change Operator
 * makes is added to that suggestion. The suggestion shows in the builder
 * as soon as Operator makes it (see series-review.ts).
 */
import { RiMailSendLine, RiRouteLine, RiSearchEyeLine, RiTimerLine } from "@remixicon/react";
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
import { seriesQuery } from "@/lib/api/series";

import { useSeriesRequest } from "./series-review";

export function SeriesOperatorPanel({
  workspace,
  series,
  onClose,
  className,
}: {
  workspace: string;
  series: string;
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
  const { data: shown } = useQuery(seriesQuery(workspace, series));
  const { request } = useSeriesRequest(workspace, series, shown?.spec);
  useViewing("series", series, request?.changeset.id);
  const chat = useOperatorChat(workspace, thread, key);
  const fresh = useMutation({
    mutationFn: () => startWorkspaceThread(workspace),
    onSuccess: (started) => {
      client.setQueryData(operatorKeys.thread(workspace, started.id), started);
      setPicked(started.id);
    },
  });
  const name = shown?.name || "this series";

  return (
    <OperatorSidePanel
      thread={thread}
      chat={chat}
      past={past}
      enabled={settings?.enabled ?? true}
      starting={fresh.isPending}
      heading="Work on this series with Operator"
      blurb="Operator reads this series, writes and rewrites its messages, and suggests changes you accept in the builder."
      suggestions={[
        {
          icon: RiSearchEyeLine,
          title: "Review it for gaps",
          detail: "Finds dead ends, missing exits and an unclear goal.",
          onPick: () =>
            chat.ask(
              `Review the series "${name}" for journeys that end too early, missing exits and an unclear goal, ` +
                "and suggest fixes.",
            ),
        },
        {
          icon: RiMailSendLine,
          title: "Tighten the messages",
          detail: "Rewrites each message shorter and clearer.",
          onPick: () => chat.ask(`Rewrite the messages of the series "${name}" so each is shorter and clearer.`),
        },
        {
          icon: RiTimerLine,
          title: "Add a follow-up",
          detail: "Waits, checks who did not act, and nudges them.",
          onPick: () =>
            chat.ask(
              `Add a follow-up to the series "${name}" for people who did not act on its first message, after a ` +
                "sensible wait.",
            ),
        },
      ]}
      placeholder="Ask Operator about this series"
      inputId="series-operator-composer-input"
      scope={{ icon: RiRouteLine, label: shown?.name || "This series" }}
      onPick={setPicked}
      onNew={() => fresh.mutate()}
      onClose={onClose ?? (() => undefined)}
      className={className}
    />
  );
}
