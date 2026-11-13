/*
 * An incident: an outage customers are affected by, opened in the inbox
 * like any ticket, its details beside it. The thread names its severity
 * and the areas it touches. The stepper shows where
 * it stands, Investigating, Identified, Monitoring, Resolved, each with
 * the words customers see; moving it asks first, and can post to the
 * public log on the way. Resolving goes on to "Message everyone" with a
 * note drafted from the log. Below: who it reaches, the public log with
 * its composer, the messages sent, and the linked conversations.
 */
import { RiCheckLine, RiEditLine, RiSendPlaneLine } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { motion, useReducedMotion } from "motion/react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import {
  postPublicUpdate,
  publicUpdatesQuery,
  trackerConversationsQuery,
  trackerKeys,
} from "@/lib/api/ticket-trackers";
import {
  type Ticket,
  type TicketField,
  type TicketState,
  ticketKeys,
  ticketQuery,
  ticketTypesQuery,
} from "@/lib/api/tickets";
import { formatNumber } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { IncidentLog } from "./incident-log";
import { MessageEveryone } from "./message-everyone";
import { SentMessages } from "./message-everyone-results";
import { TypeIcon } from "./state-pill";
import { TicketDetails, TicketTitle } from "./ticket-thread";
import { TrackerConversations } from "./tracker-conversations";
import { draftFromLog, useChangeTicket, useTypeStates } from "./tracker-helpers";
import { ImpactTotals, IssueChips, Reach, Title } from "./tracker-parts";
import { useTicketSave } from "./use-ticket-save";

const severityTint: Record<string, ChipColor> = { minor: "yellow", major: "orange", critical: "rose" };

type Messaging = { kind: "closed" } | { kind: "new"; draft: string } | { kind: "sent"; update: string };

/*
 * Stepper shows the incident's states in order; the current one carries
 * a marker that slides to it, and every other is a button that moves the
 * incident there after confirming.
 */
function Stepper({
  states,
  current,
  onPick,
}: {
  states: TicketState[];
  current: string;
  onPick: (state: TicketState) => void;
}) {
  const { t } = useTranslation("trackers");
  const at = states.findIndex((s) => s.id === current);

  return (
    <ol aria-label={t("incident.stages")} className="grid grid-cols-2 gap-2 md:grid-cols-4">
      {states.map((s, i) => (
        <Stage key={s.id} state={s} number={i + 1} now={i === at} past={i < at} onPick={() => onPick(s)} />
      ))}
    </ol>
  );
}

function Stage({
  state,
  number,
  now,
  past,
  onPick,
}: {
  state: TicketState;
  number: number;
  now: boolean;
  past: boolean;
  onPick: () => void;
}) {
  const still = useReducedMotion() ?? false;

  return (
    <li className="relative">
      {now ? (
        <motion.span
          layoutId="incident-stage"
          transition={still ? { duration: 0 } : { type: "spring", stiffness: 420, damping: 36 }}
          className="absolute inset-0 rounded-2xl bg-background-primary-default shadow-xs ring-1 ring-accent-500 ring-inset"
        />
      ) : null}
      <button
        type="button"
        aria-current={now ? "step" : undefined}
        disabled={now}
        onClick={onPick}
        className={cx(
          "relative flex h-full w-full flex-col items-start gap-1 rounded-2xl p-3 text-start outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring",
          now
            ? "cursor-default"
            : "bg-background-secondary-default ring-1 ring-border-button-default ring-inset hover:bg-background-secondary-hover",
        )}
      >
        <span className="flex items-center gap-2">
          <span
            className={cx(
              "flex size-5 items-center justify-center rounded-full text-caption-1-medium transition-colors duration-300",
              past || now ? "bg-accent-500 text-text-white" : "bg-background-tertiary-default text-text-tertiary",
            )}
          >
            {past ? <RiCheckLine aria-hidden className="size-3.5" /> : String(number)}
          </span>
          <span className={cx("text-body-2-medium", now ? "text-text-primary" : "text-text-secondary")}>
            {state.name}
          </span>
        </span>
        <span className="text-caption-1-regular text-text-tertiary">“{state.customer_label}”</span>
      </button>
    </li>
  );
}

/*
 * MoveDialog confirms moving the incident, with an optional post to the
 * public log that can also reach everyone linked. Moving to resolved
 * goes on to message everyone instead.
 */
function MoveDialog({
  workspace,
  ticket,
  state,
  linked,
  onMoved,
  onClose,
}: {
  workspace: string;
  ticket: Ticket;
  state: TicketState;
  linked: number;
  onMoved: (state: TicketState) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation("trackers");
  const client = useQueryClient();
  const change = useChangeTicket(workspace, ticket);
  const resolving = state.group === "resolved";
  const [body, setBody] = useState("");
  const [send, setSend] = useState(false);
  const [busy, setBusy] = useState(false);
  const move = async () => {
    setBusy(true);
    try {
      if (body.trim()) {
        await postPublicUpdate(workspace, ticket.id, {
          body: body.trim(),
          state_id: state.id,
          send_to_linked: send && !resolving,
        });
      } else {
        await change.mutateAsync({ state_id: state.id });
      }
      void client.invalidateQueries({ queryKey: ticketKeys.one(workspace, ticket.id) });
      void client.invalidateQueries({ queryKey: trackerKeys.log(workspace, ticket.id) });
      void client.invalidateQueries({ queryKey: trackerKeys.updates(workspace, ticket.id) });
      onMoved(state);
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={t("incident.moveTitle", { state: state.name })}
        description={t("incident.customersWillSee", { label: state.customer_label })}
      />
      <ModalBody className="flex flex-col gap-4 pb-4">
        <Textarea
          label={t("incident.moveNote")}
          placeholder={t("incident.moveNotePlaceholder")}
          value={body}
          onChange={setBody}
          rows={3}
        />
        {resolving ? (
          <p className="text-body-2-regular text-text-secondary">{t("incident.resolveNext")}</p>
        ) : (
          <Switch size="sm" isSelected={send} onChange={setSend} isDisabled={body.trim() === "" || linked === 0}>
            {t("log.sendToLinked", { count: linked, formatted: formatNumber(linked) })}
          </Switch>
        )}
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          {t("common.cancel")}
        </Button>
        <Button loading={busy} onClick={() => void move()}>
          {resolving ? t("incident.resolveAndMessage") : t("incident.moveTitle", { state: state.name })}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * DetailsDialog sets the severity on radio cards and the areas on
 * checkboxes, both from the options the incident type's fields offer.
 */
function DetailsDialog({
  workspace,
  ticket,
  severity,
  areas,
  onClose,
}: {
  workspace: string;
  ticket: Ticket;
  severity: TicketField | undefined;
  areas: TicketField | undefined;
  onClose: () => void;
}) {
  const { t } = useTranslation("trackers");
  const change = useChangeTicket(workspace, ticket);
  const [level, setLevel] = useState(String(ticket.fields.severity ?? ""));
  const [picked, setPicked] = useState<string[]>(
    Array.isArray(ticket.fields.areas) ? (ticket.fields.areas as string[]) : [],
  );

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader title={t("incident.detailsTitle")} description={`${ticket.reference} ${ticket.title}`} />
      <ModalBody className="flex flex-col gap-5 pb-4">
        {severity ? (
          <RadioGroup aria-label={severity.label} value={level} onChange={setLevel} className="flex flex-col gap-2">
            <span className="text-body-2-medium text-text-secondary">{severity.label}</span>
            {severity.options.map((o) => (
              <RadioCard
                key={o}
                value={o}
                title={<span className="capitalize">{o}</span>}
                description={t(`incident.severityHint.${o}`, { defaultValue: "" }) || undefined}
              />
            ))}
          </RadioGroup>
        ) : null}
        {areas ? (
          <fieldset className="flex flex-col gap-2">
            <legend className="mb-2 text-body-2-medium text-text-secondary">{areas.label}</legend>
            {areas.options.map((o) => (
              <Checkbox
                key={o}
                isSelected={picked.includes(o)}
                onChange={(on) => setPicked((now) => (on ? [...now, o] : now.filter((x) => x !== o)))}
              >
                {o}
              </Checkbox>
            ))}
          </fieldset>
        ) : null}
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          {t("common.cancel")}
        </Button>
        <Button
          loading={change.isPending}
          onClick={() =>
            change.mutate(
              {
                fields: {
                  ...(severity ? { [severity.key]: level || null } : {}),
                  ...(areas ? { [areas.key]: picked } : {}),
                },
              },
              { onSuccess: onClose },
            )
          }
        >
          {t("common.save")}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * Heading names the incident with its severity, its areas and the issue
 * behind it.
 */
function Heading({ ticket, editable, onEdit }: { ticket: Ticket; editable: boolean; onEdit: () => void }) {
  const { t } = useTranslation("trackers");
  const severity = String(ticket.fields.severity ?? "");
  const areas = Array.isArray(ticket.fields.areas) ? (ticket.fields.areas as string[]) : [];

  return (
    <div className="flex flex-col gap-3">
      {ticket.description ? (
        <p className="whitespace-pre-line text-body-regular text-text-secondary">{ticket.description}</p>
      ) : null}
      <div className="flex flex-wrap items-center gap-2">
        {severity ? (
          <Chip variant="caption" color={severityTint[severity] ?? "gray"}>
            {t("incident.severity", { level: severity.charAt(0).toUpperCase() + severity.slice(1) })}
          </Chip>
        ) : null}
        {areas.map((a) => (
          <Chip key={a} variant="caption" color="gray">
            {a}
          </Chip>
        ))}
        {editable ? (
          <LinkButton size="xs" variant="secondary" leadingIcon={RiEditLine} onClick={onEdit}>
            {severity || areas.length > 0 ? t("incident.editDetails") : t("incident.addDetails")}
          </LinkButton>
        ) : null}
        <IssueChips issues={ticket.issues ?? []} />
      </div>
    </div>
  );
}

export function IncidentPage({ ticketId }: { ticketId: string }) {
  const { t } = useTranslation("trackers");
  const workspace = useAccount().workspace.id;
  const { data: ticket } = useQuery(ticketQuery(workspace, ticketId));

  if (!ticket) {
    return (
      <Panel label={t("incident.label")} className="h-full w-full flex-1">
        <PageHeader title={<Title ticket={undefined} fallback={t("incident.label")} />} />
        <div className="mx-auto w-full max-w-[64rem] px-4 pt-6 md:px-6">
          <ImpactTotals impact={undefined} />
        </div>
      </Panel>
    );
  }

  return <Incident workspace={workspace} ticket={ticket} />;
}

function Incident({ workspace, ticket }: { workspace: string; ticket: Ticket }) {
  const { t } = useTranslation("trackers");
  const { data: linked, isPending } = useQuery(trackerConversationsQuery(workspace, ticket.id));
  const { data: posts = [] } = useQuery(publicUpdatesQuery(workspace, ticket.id));
  const { data: types = [] } = useQuery(ticketTypesQuery(workspace));
  const states = useTypeStates(workspace, ticket);
  const [moving, setMoving] = useState<TicketState | null>(null);
  const [editing, setEditing] = useState(false);
  const [messaging, setMessaging] = useState<Messaging>({ kind: "closed" });
  const rows = linked?.rows ?? [];
  const fields = types.find((ty) => ty.id === ticket.type_id)?.fields ?? [];
  const severityField = fields.find((f) => f.key === "severity");
  const areasField = fields.find((f) => f.key === "areas");
  const save = useTicketSave(workspace, ticket);

  return (
    <div className="flex h-full min-w-0 flex-1 gap-2">
      <Panel label={t("incident.label")} className="flex h-full min-w-0 flex-1 flex-col">
        <header className="flex h-12 shrink-0 items-center gap-2 ps-3 pe-2.5 pt-[7px]">
          <TypeIcon icon={ticket.type.icon} kind={ticket.kind} />
          <span className="shrink-0 text-body-2-medium text-text-tertiary">{ticket.reference}</span>
          <TicketTitle key={ticket.version} ticket={ticket} save={save} />
          <span className="ms-auto shrink-0">
            <Button
              size="small"
              variant="secondary"
              leadingIcon={RiSendPlaneLine}
              disabled={rows.length === 0}
              onClick={() => setMessaging({ kind: "new", draft: "" })}
            >
              {t("message.open")}
            </Button>
          </span>
        </header>
        <ScrollArea className="flex-1">
          <div className="mx-auto flex w-full max-w-[760px] flex-col gap-6 px-4 pt-5 pb-16">
            <Heading ticket={ticket} editable={Boolean(severityField || areasField)} onEdit={() => setEditing(true)} />
            {states.length > 0 ? <Stepper states={states} current={ticket.state_id} onPick={setMoving} /> : null}
            <IncidentLog workspace={workspace} ticket={ticket} states={states} linked={rows.length} />
            <SentMessages
              workspace={workspace}
              ticket={ticket.id}
              onOpen={(u) => setMessaging({ kind: "sent", update: u.id })}
            />
            <TrackerConversations workspace={workspace} ticket={ticket} rows={rows} loading={isPending} />
          </div>
        </ScrollArea>
      </Panel>
      <Panel label="Details" className="hidden h-full w-[340px] shrink-0 flex-col lg:flex">
        <header className="flex h-12 shrink-0 items-center ps-4 pe-2.5 pt-[7px]">
          <h2 className="text-body-medium text-text-primary">Details</h2>
        </header>
        <ScrollArea>
          <TicketDetails workspace={workspace} ticket={ticket} save={save}>
            <Reach impact={linked?.impact ?? ticket.impact} />
          </TicketDetails>
        </ScrollArea>
      </Panel>
      {moving ? (
        <MoveDialog
          workspace={workspace}
          ticket={ticket}
          state={moving}
          linked={rows.length}
          onClose={() => setMoving(null)}
          onMoved={(state) => {
            setMoving(null);
            if (state.group === "resolved" && rows.length > 0) {
              setMessaging({ kind: "new", draft: draftFromLog(ticket, posts) });
            }
          }}
        />
      ) : null}
      {editing ? (
        <DetailsDialog
          workspace={workspace}
          ticket={ticket}
          severity={severityField}
          areas={areasField}
          onClose={() => setEditing(false)}
        />
      ) : null}
      {messaging.kind !== "closed" ? (
        <MessageEveryone
          workspace={workspace}
          ticket={ticket}
          rows={rows}
          draft={messaging.kind === "new" ? messaging.draft : ""}
          sentUpdate={messaging.kind === "sent" ? messaging.update : undefined}
          onClose={() => setMessaging({ kind: "closed" })}
        />
      ) : null}
    </div>
  );
}
