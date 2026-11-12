/*
 * An incident's public update log: short posts customers see on the
 * messenger's status card and the help center's status page, newest
 * first. The composer posts one, and can also send it to everyone the
 * incident links, on their own channel. A post that moved the incident
 * names the state it moved to.
 */
import { RiMegaphoneLine, RiSendPlaneLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { Reveal } from "@/features/shell/reveal";
import { postPublicUpdate, publicUpdatesQuery, trackerKeys } from "@/lib/api/ticket-trackers";
import { type Ticket, type TicketState, ticketKeys } from "@/lib/api/tickets";
import { formatDateTime, formatNumber } from "@/lib/i18n/format";

const limit = 2000;

export function IncidentLog({
  workspace,
  ticket,
  states,
  linked,
}: {
  workspace: string;
  ticket: Ticket;
  states: TicketState[];
  linked: number;
}) {
  const { t } = useTranslation("trackers");
  const client = useQueryClient();
  const { data: posts = [], isPending } = useQuery(publicUpdatesQuery(workspace, ticket.id));
  const [body, setBody] = useState("");
  const [send, setSend] = useState(false);
  const [fresh, setFresh] = useState<string | null>(null);
  const byId = new Map(states.map((s) => [s.id, s]));
  const titleId = useId();
  const post = useMutation({
    mutationFn: () => postPublicUpdate(workspace, ticket.id, { body: body.trim(), send_to_linked: send }),
    onSuccess: (posted) => {
      setFresh(posted.id);
      setBody("");
      setSend(false);
      toast.success(send ? t("log.postedAndSent", { count: linked }) : t("log.posted"));
      void client.invalidateQueries({ queryKey: trackerKeys.log(workspace, ticket.id) });
      void client.invalidateQueries({ queryKey: trackerKeys.updates(workspace, ticket.id) });
      void client.invalidateQueries({ queryKey: ticketKeys.one(workspace, ticket.id) });
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <section aria-labelledby={titleId} className="flex flex-col gap-3">
      <div className="flex flex-col gap-0.5 px-3">
        <h2 id={titleId} className="text-body-2-medium text-text-secondary">
          {t("log.title")}
        </h2>
        <p className="text-body-2-regular text-text-tertiary">{t("log.description")}</p>
      </div>
      <form
        className="flex flex-col gap-3 rounded-2xl bg-background-primary-default p-3 shadow-xs ring-1 ring-border-button-default ring-inset"
        onSubmit={(event) => {
          event.preventDefault();
          post.mutate();
        }}
      >
        <Textarea
          aria-label={t("log.composer")}
          placeholder={t("log.placeholder")}
          value={body}
          onChange={setBody}
          maxLength={limit}
          rows={3}
        />
        <div className="flex flex-wrap items-center justify-between gap-3">
          <Switch size="sm" isSelected={send} onChange={setSend} isDisabled={linked === 0}>
            {t("log.sendToLinked", { count: linked, formatted: formatNumber(linked) })}
          </Switch>
          <Button
            type="submit"
            size="small"
            leadingIcon={RiSendPlaneLine}
            loading={post.isPending}
            disabled={body.trim() === ""}
          >
            {send ? t("log.postAndSend") : t("log.post")}
          </Button>
        </div>
      </form>
      {!isPending && posts.length === 0 ? (
        <p className="px-3 text-body-2-regular text-text-tertiary">{t("log.empty")}</p>
      ) : null}
      <ol className="relative flex flex-col ps-3">
        {posts.map((p, i) => {
          const state = p.state_id ? byId.get(p.state_id) : undefined;
          return (
            <li key={p.id} className="relative flex gap-3 pb-4">
              {i < posts.length - 1 ? (
                <span aria-hidden className="absolute start-[11px] top-7 bottom-0 w-px bg-separator-border" />
              ) : null}
              <span
                aria-hidden
                className="z-10 flex size-6 shrink-0 items-center justify-center rounded-full bg-background-tertiary-default"
              >
                <RiMegaphoneLine className="size-3.5 text-foreground-icon-secondary" />
              </span>
              <Reveal animate={p.id === fresh} className="min-w-0 flex-1">
                <div className="flex min-w-0 flex-col gap-1">
                  <p className="flex flex-wrap items-center gap-2 text-caption-1-regular text-text-tertiary">
                    <time dateTime={p.created_at}>{formatDateTime(p.created_at)}</time>
                    {state ? (
                      <Chip variant="caption" color="blue">
                        {state.name}
                      </Chip>
                    ) : null}
                    {p.update_id ? (
                      <Chip variant="caption" color="lime">
                        {t("log.sent")}
                      </Chip>
                    ) : null}
                    {p.source !== "dimmi" ? <span className="capitalize">{p.source}</span> : null}
                  </p>
                  <p className="whitespace-pre-line text-body-regular text-text-primary">{p.body}</p>
                </div>
              </Reveal>
            </li>
          );
        })}
      </ol>
    </section>
  );
}
