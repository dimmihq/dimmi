/*
 * A spike of new conversations about one thing, suggested to everyone
 * online as an incident: "12 conversations about failed sign-ins in 10
 * minutes. Declare an incident?". It sits at the top of the list as a
 * card, never a banner. Declare opens the incident with every
 * conversation linked; Dismiss mutes the group for an hour. The card
 * appears the moment the detector broadcasts it.
 */
import { RiAlarmWarningLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { Reveal } from "@/features/shell/reveal";
import {
  decideIncidentSuggestion,
  type IncidentSuggestion,
  incidentSuggestionKeys,
  incidentSuggestionsQuery,
} from "@/lib/api/incident-suggestions";
import { channels } from "@/lib/realtime/keys";
import { useChannel } from "@/lib/realtime/use-channel";
import { cx } from "@/utils/cx";

/*
 * minutesOf is how many minutes the spike spans so far, at least one.
 */
function minutesOf(s: IncidentSuggestion): number {
  const spanned = Math.ceil((Date.now() - new Date(s.created_at).getTime()) / 60_000);

  return Math.max(1, Math.min(s.window_minutes, spanned));
}

function Suggestion({ workspace, suggestion }: { workspace: string; suggestion: IncidentSuggestion }) {
  const { t } = useTranslation("trackers");
  const client = useQueryClient();
  const navigate = useNavigate();
  const decide = useMutation({
    mutationFn: (status: "accepted" | "dismissed") => decideIncidentSuggestion(workspace, suggestion.id, status),
    onSuccess: (decided) => {
      void client.invalidateQueries({ queryKey: incidentSuggestionKeys.all(workspace) });
      if (decided.status === "accepted" && decided.ticket_id) {
        toast.success(t("suggestions.declared", { title: decided.title }));
        void navigate({ to: "/d/tickets/$ticket", params: { ticket: decided.ticket_id } });
      } else {
        toast.success(t("suggestions.dismissed"));
      }
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <li className="flex flex-col gap-2.5 rounded-xl border border-border-button-default bg-background-primary-default p-3 shadow-xs">
      <div className="flex items-start gap-2.5">
        <span className="grid size-7 shrink-0 place-items-center rounded-lg bg-background-tertiary-error">
          <RiAlarmWarningLine aria-hidden className="size-4 text-foreground-icon-error" />
        </span>
        <p className="min-w-0 text-body-2-regular text-text-secondary">
          <span className="text-body-2-medium text-text-primary">
            {t("suggestions.headline", {
              count: suggestion.conversation_count,
              topic: suggestion.title.toLowerCase(),
              minutes: minutesOf(suggestion),
            })}
          </span>{" "}
          {t("suggestions.question")}
        </p>
      </div>
      <div className="flex justify-end gap-1.5">
        <Button
          size="small"
          variant="secondary"
          loading={decide.isPending && decide.variables === "dismissed"}
          disabled={decide.isPending}
          onClick={() => decide.mutate("dismissed")}
        >
          {t("suggestions.dismiss")}
        </Button>
        <Button
          size="small"
          loading={decide.isPending && decide.variables === "accepted"}
          disabled={decide.isPending}
          onClick={() => decide.mutate("accepted")}
        >
          {t("suggestions.declare")}
        </Button>
      </div>
    </li>
  );
}

/*
 * IncidentSuggestions lists the open suggestions, newest first, and
 * refreshes the moment the detector finds a spike.
 */
export function IncidentSuggestions({ workspace, className }: { workspace: string; className?: string }) {
  const { t } = useTranslation("trackers");
  const client = useQueryClient();
  const { data: suggestions = [] } = useQuery(incidentSuggestionsQuery(workspace));
  useChannel(workspace ? channels.workspace(workspace) : null, {
    onEnvelope: (envelope) => {
      if (envelope.event === "incident.suggested") {
        void client.invalidateQueries({ queryKey: incidentSuggestionKeys.all(workspace) });
      }
    },
  });
  if (suggestions.length === 0) {
    return null;
  }

  return (
    <Reveal className={cx("shrink-0 px-[7px] pb-2", className)}>
      <ul aria-label={t("suggestions.label")} className="flex flex-col gap-1.5">
        {suggestions.slice(0, 2).map((s) => (
          <Suggestion key={s.id} workspace={workspace} suggestion={s} />
        ))}
      </ul>
    </Reveal>
  );
}
