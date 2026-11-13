/*
 * Message everyone a tracker links, in three steps. Write the
 * message once and see it as one customer will, in their own
 * conversation, switching between customers to check it reads right for
 * each. Then check who it reaches: dimmi counts them and names the ones
 * it holds back and why (they wrote since, or a teammate already told
 * them), and nothing is sent until the teammate confirms the count.
 * Last, follow each delivery as it goes out, with the reason beside any
 * that failed. Resolving them all is a choice made while writing.
 */
import { RiArrowLeftLine, RiCheckLine, RiErrorWarningLine, RiSendPlaneLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { StepMotion } from "@/features/knowledge/step-motion";
import { useAccount } from "@/features/settings/use-account";
import { sendTicketUpdate, type TrackerConversation, trackerKeys, type UpdatePlan } from "@/lib/api/ticket-trackers";
import { type Ticket, ticketKeys } from "@/lib/api/tickets";
import { formatNumber } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { DeliveryResults } from "./message-everyone-results";
import { customerName, heldReasonLabel } from "./tracker-helpers";

type Step = "write" | "check" | "results";

const order: Step[] = ["write", "check", "results"];

function initials(name: string): string {
  return name
    .split(/[\s@.]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");
}

/*
 * Steps shows where the teammate is: write, check, sent.
 */
function Steps({ step }: { step: Step }) {
  const { t } = useTranslation("trackers");
  const at = order.indexOf(step);

  return (
    <ol className="flex items-center gap-2 px-6 pb-2" aria-label={t("message.steps")}>
      {order.map((s, i) => (
        <li key={s} aria-current={s === step ? "step" : undefined} className="flex items-center gap-2">
          <span
            className={cx(
              "flex size-5 items-center justify-center rounded-full text-caption-1-medium transition-colors duration-300",
              i < at
                ? "bg-accent-500 text-text-white"
                : i === at
                  ? "text-text-primary ring-2 ring-accent-500 ring-inset"
                  : "bg-background-tertiary-default text-text-tertiary",
            )}
          >
            {i < at ? <RiCheckLine aria-hidden className="size-3.5" /> : i + 1}
          </span>
          <span className={cx("text-caption-1-medium", i === at ? "text-text-primary" : "text-text-tertiary")}>
            {t(`message.step.${s}`)}
          </span>
          {i < order.length - 1 ? <span aria-hidden className="h-px w-6 bg-separator-border" /> : null}
        </li>
      ))}
    </ol>
  );
}

/*
 * Preview shows the message the way one customer will find it: at the
 * end of their own conversation, from the teammate sending it.
 */
function Preview({ body, row, author }: { body: string; row: TrackerConversation | undefined; author: string }) {
  const { t } = useTranslation("trackers");
  if (!row) {
    return null;
  }

  return (
    <div className="flex flex-col gap-3 rounded-2xl bg-background-secondary-default p-4">
      <div className="flex items-center gap-2">
        <Avatar size="xs" initials={initials(customerName(row))} />
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-body-2-medium text-text-primary">
            {customerName(row)}
            {row.company_name ? <span className="text-text-tertiary"> · {row.company_name}</span> : null}
          </span>
          <span className="truncate text-caption-1-regular text-text-tertiary">
            #{row.conversation_number} {row.subject ?? ""}
          </span>
        </span>
      </div>
      <div className="flex flex-col items-end gap-1">
        <div className="max-w-[85%] rounded-2xl rounded-ee-md bg-background-primary-default px-3.5 py-2.5 text-body-2-regular whitespace-pre-line text-text-primary shadow-xs ring-1 ring-border-button-default ring-inset">
          {body.trim() || <span className="text-text-placeholder">{t("message.previewEmpty")}</span>}
        </div>
        <span className="text-caption-1-regular text-text-tertiary">{t("message.previewFrom", { name: author })}</span>
      </div>
    </div>
  );
}

/*
 * MessageEveryone is the whole flow in one dialog; sentUpdate opens a
 * message already sent on its deliveries.
 */
export function MessageEveryone({
  workspace,
  ticket,
  rows,
  sentUpdate,
  onClose,
}: {
  workspace: string;
  ticket: Ticket;
  rows: TrackerConversation[];
  sentUpdate?: string;
  onClose: () => void;
}) {
  const { t } = useTranslation("trackers");
  const client = useQueryClient();
  const account = useAccount();
  const [step, setStep] = useState<Step>(sentUpdate ? "results" : "write");
  const [forward, setForward] = useState(true);
  const [body, setBody] = useState("");
  const [resolve, setResolve] = useState(false);
  const [previewId, setPreviewId] = useState<string | null>(rows[0]?.conversation_id ?? null);
  const [plan, setPlan] = useState<UpdatePlan | null>(null);
  const [updateId, setUpdateId] = useState<string | null>(sentUpdate ?? null);
  const go = (next: Step) => {
    setForward(order.indexOf(next) > order.indexOf(step));
    setStep(next);
  };
  const byConversation = new Map(rows.map((r) => [r.conversation_id, r]));
  const previewRow = previewId ? byConversation.get(previewId) : rows[0];
  const check = useMutation({
    mutationFn: () => sendTicketUpdate(workspace, ticket.id, { body, resolve, dry_run: true }),
    onSuccess: (next) => {
      setPlan(next);
      go("check");
    },
    onError: (error) => toast.error(error.message),
  });
  const send = useMutation({
    mutationFn: () => sendTicketUpdate(workspace, ticket.id, { body, resolve }),
    onSuccess: (sent) => {
      setUpdateId(sent.update?.id ?? null);
      void client.invalidateQueries({ queryKey: trackerKeys.updates(workspace, ticket.id) });
      void client.invalidateQueries({ queryKey: ticketKeys.one(workspace, ticket.id) });
      go("results");
    },
    onError: (error) => toast.error(error.message),
  });
  const reach = plan ? plan.recipients - plan.held.length : 0;

  return (
    <Modal
      size="lg"
      className="h-[min(720px,calc(100dvh-3rem))]"
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
    >
      <ModalHeader title={t("message.title")} description={`${ticket.reference} ${ticket.title}`} />
      <Steps step={step} />
      <ModalBody className="flex min-h-0 flex-1 flex-col overflow-y-auto pb-4">
        <StepMotion view={step} forward={forward} className="flex flex-col gap-4">
          {step === "write" ? (
            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-4">
                <Textarea
                  label={t("message.body")}
                  placeholder={t("message.bodyPlaceholder")}
                  value={body}
                  onChange={setBody}
                  rows={9}
                  autoFocus
                />
                <Switch size="sm" isSelected={resolve} onChange={setResolve}>
                  {t("message.resolve")}
                </Switch>
                <p className="-mt-2 text-caption-1-regular text-text-tertiary">{t("message.resolveHint")}</p>
              </div>
              <div className="flex flex-col gap-3">
                <Select
                  label={t("message.previewAs")}
                  className="w-full"
                  selectedKey={previewRow?.conversation_id ?? null}
                  onSelectionChange={(key) => setPreviewId(key === null ? null : String(key))}
                >
                  {rows.map((r) => (
                    <SelectItem key={r.conversation_id} id={r.conversation_id} textValue={customerName(r)}>
                      {customerName(r)}
                      {r.company_name ? ` · ${r.company_name}` : ""}
                    </SelectItem>
                  ))}
                </Select>
                <Preview body={body} row={previewRow} author={account.name} />
              </div>
            </div>
          ) : null}
          {step === "check" && plan ? <Check plan={plan} resolve={resolve} rows={byConversation} /> : null}
          {step === "results" ? <DeliveryResults workspace={workspace} ticket={ticket.id} updateId={updateId} /> : null}
        </StepMotion>
      </ModalBody>
      <ModalFooter>
        {step === "write" ? (
          <>
            <Button variant="secondary" onClick={onClose}>
              {t("common.cancel")}
            </Button>
            <Button
              loading={check.isPending}
              disabled={body.trim() === "" || rows.length === 0}
              onClick={() => check.mutate()}
            >
              {t("message.checkRecipients")}
            </Button>
          </>
        ) : null}
        {step === "check" ? (
          <>
            <Button variant="secondary" leadingIcon={RiArrowLeftLine} onClick={() => go("write")}>
              {t("common.back")}
            </Button>
            <Button
              leadingIcon={RiSendPlaneLine}
              loading={send.isPending}
              disabled={reach === 0}
              onClick={() => send.mutate()}
            >
              {t("message.sendTo", { count: reach })}
            </Button>
          </>
        ) : null}
        {step === "results" ? <Button onClick={onClose}>{t("common.done")}</Button> : null}
      </ModalFooter>
    </Modal>
  );
}

/*
 * Check is the confirmation: how many get the message, whether their
 * conversations resolve, and who is held back with the reason.
 */
function Check({
  plan,
  resolve,
  rows,
}: {
  plan: UpdatePlan;
  resolve: boolean;
  rows: Map<string, TrackerConversation>;
}) {
  const { t } = useTranslation("trackers");
  const reach = plan.recipients - plan.held.length;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1 rounded-2xl bg-background-secondary-default p-4">
        <p className="text-title-3-medium text-text-primary tabular-nums">
          {t("message.reach", { count: reach, formatted: formatNumber(reach) })}
        </p>
        <p className="text-body-2-regular text-text-secondary">
          {resolve ? t("message.reachResolve") : t("message.reachKeep")}
        </p>
      </div>
      {plan.held.length > 0 ? (
        <section className="flex flex-col gap-2">
          <h3 className="flex items-center gap-1.5 text-body-2-medium text-text-secondary">
            <RiErrorWarningLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
            {t("message.held", { count: plan.held.length })}
          </h3>
          <ul className="flex flex-col rounded-2xl bg-background-secondary-default px-4">
            {plan.held.map((h) => {
              const row = rows.get(h.conversation_id);
              return (
                <li
                  key={h.conversation_id}
                  className="flex items-center justify-between gap-3 border-separator-border border-t py-2.5 first:border-t-0"
                >
                  <span className="flex min-w-0 flex-col">
                    <span className="truncate text-body-2-medium text-text-primary">
                      {row ? customerName(row) : h.conversation_id}
                    </span>
                    {row ? (
                      <span className="truncate text-caption-1-regular text-text-tertiary">
                        #{row.conversation_number} {row.company_name ?? ""}
                      </span>
                    ) : null}
                  </span>
                  <span className="shrink-0 text-body-2-regular text-text-secondary">{heldReasonLabel(h.reason)}</span>
                </li>
              );
            })}
          </ul>
        </section>
      ) : (
        <p className="text-body-2-regular text-text-tertiary">{t("message.noneHeld")}</p>
      )}
    </div>
  );
}
