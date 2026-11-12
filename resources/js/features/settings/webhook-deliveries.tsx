/*
 * An endpoint's recent deliveries: each with its event, status and when
 * it went, and a dialog with every attempt, the payload, and sending it
 * again.
 */
import { RiWebhookLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import {
  redeliverWebhook,
  testWebhookEndpoint,
  type WebhookDelivery,
  webhookDeliveriesQuery,
  webhookDeliveryQuery,
  webhooksKey,
} from "@/lib/api/webhooks";

import { ago } from "./attributes";
import { Action } from "./controls";
import { Empty, Glyph, Item, Section } from "./layout";
import { answerLine, deliveryState } from "./webhooks";

export function DeliveriesSection({
  workspace,
  endpoint,
  manage,
}: {
  workspace: string;
  endpoint: string;
  manage: boolean;
}) {
  const queryClient = useQueryClient();
  const { data: page, error } = useQuery(webhookDeliveriesQuery(workspace, endpoint));
  const [open, setOpen] = useState<string | null>(null);
  const refresh = () => queryClient.invalidateQueries({ queryKey: webhooksKey(workspace) });
  const test = useMutation({ mutationFn: () => testWebhookEndpoint(workspace, endpoint), onSettled: refresh });
  const now = new Date();

  return (
    <Section
      title="Recent deliveries"
      description="Failed deliveries are tried again for a day."
      action={
        manage ? (
          <Action loading={test.isPending} onPress={() => test.mutate()}>
            Send test
          </Action>
        ) : null
      }
      loading={!page && !error}
      rows={3}
    >
      {test.data ? (
        <div className="py-2.5 pe-2.5">
          <Notification
            status={test.data.delivery.status === "succeeded" ? "success" : "error"}
            title={`Test ${test.data.delivery.status === "succeeded" ? "delivered" : "failed"}: ${answerLine({
              last_response_status: test.data.attempt.response_status,
              last_error_code: test.data.attempt.error_code,
            })}`}
            description={test.data.attempt.hint}
            dismissible={false}
            className="w-full"
          />
        </div>
      ) : null}
      {test.error ? <Empty>{test.error.message}</Empty> : null}
      {error ? <Empty>{error.message}</Empty> : null}
      {page?.items.length === 0 ? <Empty>Nothing sent yet</Empty> : null}
      {page?.items.map((delivery) => (
        <DeliveryItem key={delivery.id} delivery={delivery} now={now} onOpen={() => setOpen(delivery.id)} />
      ))}
      {open ? (
        <DeliveryDialog
          workspace={workspace}
          delivery={open}
          manage={manage}
          onClose={() => setOpen(null)}
          onResent={refresh}
        />
      ) : null}
    </Section>
  );
}

function DeliveryItem({ delivery, now, onOpen }: { delivery: WebhookDelivery; now: Date; onOpen: () => void }) {
  const state = deliveryState(delivery);
  const attempts = delivery.attempt_count === 1 ? "1 attempt" : `${delivery.attempt_count} attempts`;

  return (
    <Item
      mark={<Glyph icon={RiWebhookLine} />}
      title={<span className="font-mono">{delivery.event_type}</span>}
      detail={[answerLine(delivery), attempts, ago(delivery.created_at, now)].join(" · ")}
    >
      <Chip variant="caption" color={state.color}>
        {state.label}
      </Chip>
      <Action size="xs" onPress={onOpen}>
        View
      </Action>
    </Item>
  );
}

function DeliveryDialog({
  workspace,
  delivery,
  manage,
  onClose,
  onResent,
}: {
  workspace: string;
  delivery: string;
  manage: boolean;
  onClose: () => void;
  onResent: () => void;
}) {
  const queryClient = useQueryClient();
  const { data } = useQuery(webhookDeliveryQuery(workspace, delivery));
  const resend = useMutation({
    mutationFn: () => redeliverWebhook(workspace, delivery),
    onSuccess: (fresh) => {
      queryClient.setQueryData(webhookDeliveryQuery(workspace, delivery).queryKey, fresh);
      onResent();
    },
  });

  return (
    <Modal isOpen size="xl" onOpenChange={(open) => !open && onClose()}>
      <ModalHeader
        icon={RiWebhookLine}
        title={data?.event_type ?? "Delivery"}
        description={data ? `${deliveryState(data).label} · ${answerLine(data)}` : undefined}
      />
      <ModalBody className="flex flex-col gap-5">
        {resend.error ? (
          <Notification status="error" title={resend.error.message} dismissible={false} className="w-full" />
        ) : null}
        <div className="flex flex-col gap-2">
          <span className="text-body-medium text-text-secondary">Attempts</span>
          {data?.attempts.length === 0 ? (
            <span className="text-body-2-regular text-text-tertiary">Not sent yet.</span>
          ) : null}
          {data?.attempts.map((a) => (
            <div key={a.id} className="flex flex-col gap-0.5 border-b border-separator-border pb-2 last:border-b-0">
              <span className="text-body-regular text-text-primary">
                {`#${a.number} · ${answerLine({ last_response_status: a.response_status, last_error_code: a.error_code })}`}
                {a.duration_ms === null ? "" : ` · ${a.duration_ms} ms`}
              </span>
              <span className="text-body-2-regular text-text-tertiary">
                {`${new Date(a.requested_at).toLocaleString()} · ${a.trigger}`}
              </span>
              {a.hint ? <span className="text-body-2-regular text-text-secondary">{a.hint}</span> : null}
              {a.error_message ? (
                <span className="text-body-2-regular text-text-secondary">{a.error_message}</span>
              ) : null}
              {a.response_body ? (
                <pre className="max-h-32 overflow-auto rounded-lg bg-background-secondary-default p-2 font-mono text-caption-1-regular text-text-secondary">
                  {a.response_body}
                </pre>
              ) : null}
            </div>
          ))}
        </div>
        <div className="flex flex-col gap-2">
          <span className="text-body-medium text-text-secondary">Payload</span>
          <pre
            dir="ltr"
            className="max-h-80 overflow-auto rounded-lg bg-background-secondary-default p-3 font-mono text-caption-1-regular text-text-secondary"
          >
            {data ? JSON.stringify(data.payload, null, 2) : ""}
          </pre>
        </div>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Close
        </Button>
        {manage ? (
          <Button loading={resend.isPending} disabled={!data} onClick={() => resend.mutate()}>
            Send again
          </Button>
        ) : null}
      </ModalFooter>
    </Modal>
  );
}
