/*
 * One webhook endpoint: where it posts and what it hears, whether it is
 * on, how it answered over the last day, its signing secret, and the
 * events sent to it lately, each of which can be opened and sent again.
 * A test sends a sample event now. Removing the endpoint asks first,
 * since its log goes with it. People who may only read see all of it
 * but change nothing and never see the secret. Its switches change only
 * when Save is pressed in the bar at the bottom.
 */
import { RiArrowLeftLine, RiWebhookLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import { Switch } from "@/components/base/switch/switch";
import { toast } from "@/components/base/toast/toast";
import {
  deleteWebhookEndpoint,
  revealWebhookSecret,
  rotateWebhookSecret,
  updateWebhookEndpoint,
  type WebhookEndpoint,
  type WebhookEndpointInput,
  type WebhookSecret,
  webhookEndpointQuery,
  webhookEventTypesQuery,
  webhookStatsQuery,
  webhooksKey,
} from "@/lib/api/webhooks";

import { ago } from "../attributes";
import { Action, Danger } from "../controls";
import { PageSkeleton, Row, Section, SettingsPage } from "../layout";
import { SaveBar } from "../save-bar";
import { useAccount } from "../use-account";
import { useCanManageWebhooks } from "../use-can-manage-webhooks";
import { DeliveriesSection } from "../webhook-deliveries";
import { type PickMode, WebhookEventPicker } from "../webhook-event-picker";
import { coveredTypes, endpointState, everyEvent, subscriptionFor, subscriptionLine, successRate } from "../webhooks";
import { SecretField } from "./webhooks";

/*
 * rotationOverlap is how long a replaced secret keeps signing: a day,
 * time enough to roll the new one out.
 */
const rotationOverlap = 24 * 60 * 60;

function BackToWebhooks() {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "webhooks" } })}
      >
        All endpoints
      </Button>
    </div>
  );
}

function Problem({ error }: { error: Error | null }) {
  if (!error) {
    return null;
  }

  return (
    <p role="alert" className="px-3 text-body-2-regular text-text-error-primary">
      {error.message}
    </p>
  );
}

export function WebhookEndpointSettings({ endpoint }: { endpoint: string }) {
  const workspace = useAccount().workspace.id;
  const { data, isError } = useQuery(webhookEndpointQuery(workspace, endpoint));

  if (isError) {
    return (
      <SettingsPage title="Endpoint not found" description="It was removed, or it belongs to another workspace.">
        <BackToWebhooks />
      </SettingsPage>
    );
  }

  if (!data) {
    return <PageSkeleton />;
  }

  return <EndpointPage endpoint={data} />;
}

type EndpointSwitches = Pick<WebhookEndpointInput, "include_notes" | "enabled">;

/*
 * useEndpointDraft holds the switches flipped but not saved yet. A switch
 * flipped back drops out. Save sends them in one call, then reads the
 * endpoint again; Discard puts them back.
 */
function useEndpointDraft(workspace: string, endpoint: WebhookEndpoint) {
  const queryClient = useQueryClient();
  const [pending, setPending] = useState<EndpointSwitches>({});
  const save = useMutation({
    mutationFn: () => updateWebhookEndpoint(workspace, endpoint.id, pending),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: webhooksKey(workspace) });
      setPending({});
      toast.success("Endpoint saved");
    },
  });

  return {
    includeNotes: pending.include_notes ?? endpoint.include_notes,
    enabled: pending.enabled ?? endpoint.enabled,
    dirty: Object.keys(pending).length > 0,
    save,
    flip: (key: keyof EndpointSwitches, on: boolean) =>
      setPending(({ [key]: _, ...rest }) => (on === endpoint[key] ? rest : { ...rest, [key]: on })),
    discard: () => setPending({}),
  };
}

function EndpointPage({ endpoint }: { endpoint: WebhookEndpoint }) {
  const workspace = useAccount().workspace.id;
  const manage = useCanManageWebhooks(workspace);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const refresh = () => queryClient.invalidateQueries({ queryKey: webhooksKey(workspace) });
  const [editing, setEditing] = useState(false);
  const [removing, setRemoving] = useState(false);
  const draft = useEndpointDraft(workspace, endpoint);
  const state = endpointState(endpoint);
  useBlocker({
    shouldBlockFn: () => draft.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => draft.dirty,
  });

  return (
    <SettingsPage
      title={endpoint.url}
      description={endpoint.description || undefined}
      footer={
        <SaveBar
          dirty={draft.dirty}
          saving={draft.save.isPending}
          error={draft.save.error?.message}
          onSave={() => draft.save.mutate()}
          onDiscard={draft.discard}
        />
      }
      actions={
        <Chip variant="caption" color={state.color}>
          {state.label}
        </Chip>
      }
    >
      <BackToWebhooks />
      <Section title="Endpoint">
        <Row label="Events" description={subscriptionLine(endpoint.event_types)}>
          {manage ? <Action onPress={() => setEditing(true)}>Change</Action> : null}
        </Row>
        <Row label="Internal notes" description="Also send events about notes only your team sees.">
          <Switch
            aria-label="Internal notes"
            size="sm"
            isSelected={draft.includeNotes}
            isDisabled={!manage || draft.save.isPending}
            onChange={(on) => draft.flip("include_notes", on)}
          />
        </Row>
        <Row
          label="Sending"
          description={
            draft.enabled
              ? endpoint.health === "failing" && endpoint.failing_since
                ? `Failing since ${ago(endpoint.failing_since)}. It turns off after three days.`
                : "Events are posted as they happen."
              : endpoint.disabled_reason === "auto_failure"
                ? "Turned off after failing for days. Turn it on once it answers again."
                : "Off. Nothing is sent and waiting events were canceled."
          }
        >
          <Switch
            aria-label="Sending"
            size="sm"
            isSelected={draft.enabled}
            isDisabled={!manage || draft.save.isPending}
            onChange={(on) => draft.flip("enabled", on)}
          />
        </Row>
      </Section>
      <StatsSection workspace={workspace} endpoint={endpoint.id} />
      {manage ? <SecretSection workspace={workspace} endpoint={endpoint} /> : null}
      <DeliveriesSection workspace={workspace} endpoint={endpoint.id} manage={manage} />
      {manage ? (
        <Section title="Danger zone">
          <Row label="Remove endpoint" description="Nothing more is sent to it, and its delivery log is deleted.">
            <Danger onPress={() => setRemoving(true)}>Remove endpoint</Danger>
          </Row>
        </Section>
      ) : null}
      {editing ? (
        <EventsDialog
          workspace={workspace}
          endpoint={endpoint}
          onClose={() => setEditing(false)}
          onSaved={() => {
            setEditing(false);
            void refresh();
          }}
        />
      ) : null}
      <AlertDialog
        isOpen={removing}
        onOpenChange={setRemoving}
        title="Remove this endpoint?"
        description={`Nothing more is sent to ${endpoint.url}, and its delivery log is deleted. This cannot be undone.`}
        confirmLabel="Remove"
        pendingLabel="Removing…"
        onConfirm={async () => {
          await deleteWebhookEndpoint(workspace, endpoint.id);
          await navigate({ to: "/d/settings/$section", params: { section: "webhooks" } });
          await refresh();
        }}
      />
    </SettingsPage>
  );
}

function StatsSection({ workspace, endpoint }: { workspace: string; endpoint: string }) {
  const { data: stats } = useQuery(webhookStatsQuery(workspace, endpoint));

  return (
    <Section title="Last 24 hours" loading={!stats} rows={2}>
      {stats ? (
        <>
          <Row label="Requests">{<span className="text-body-regular text-text-secondary">{stats.attempts}</span>}</Row>
          <Row label="Delivered">
            <span className="text-body-regular text-text-secondary">{successRate(stats)}</span>
          </Row>
          <Row label="Response time" description="Median and 95th percentile.">
            <span className="text-body-regular text-text-secondary">
              {stats.attempts === 0 ? "–" : `${stats.p50_ms} ms · ${stats.p95_ms} ms`}
            </span>
          </Row>
        </>
      ) : null}
    </Section>
  );
}

function SecretSection({ workspace, endpoint }: { workspace: string; endpoint: WebhookEndpoint }) {
  const [secret, setSecret] = useState<WebhookSecret | null>(null);
  const [rotating, setRotating] = useState(false);
  const reveal = useMutation({
    mutationFn: () => revealWebhookSecret(workspace, endpoint.id),
    onSuccess: setSecret,
  });

  return (
    <Section
      title="Signing secret"
      description="Each request carries a signature made with it. Revealing it is recorded in the audit log."
    >
      {secret ? (
        <div className="flex flex-col gap-3 py-2.5 pe-2.5">
          <SecretField label="Secret" secret={secret.secret} />
          {secret.previous_secret && secret.previous_expires_at ? (
            <SecretField
              label={`Previous secret, signing until ${new Date(secret.previous_expires_at).toLocaleString()}`}
              secret={secret.previous_secret}
            />
          ) : null}
        </div>
      ) : (
        <Row label="Secret" description="Hidden.">
          <Action loading={reveal.isPending} onPress={() => reveal.mutate()}>
            Reveal
          </Action>
        </Row>
      )}
      <Row label="Rotate" description="A new secret; the old one keeps signing for a day so you can switch.">
        <Action onPress={() => setRotating(true)}>Rotate</Action>
      </Row>
      <Problem error={reveal.error} />
      <AlertDialog
        isOpen={rotating}
        onOpenChange={setRotating}
        variant="default"
        title="Rotate the signing secret?"
        description="Requests are signed with both secrets for a day. Update your server before then."
        confirmLabel="Rotate"
        pendingLabel="Rotating…"
        onConfirm={async () => setSecret(await rotateWebhookSecret(workspace, endpoint.id, rotationOverlap))}
      />
    </Section>
  );
}

function EventsDialog({
  workspace,
  endpoint,
  onClose,
  onSaved,
}: {
  workspace: string;
  endpoint: WebhookEndpoint;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { data: catalog = [] } = useQuery(webhookEventTypesQuery(workspace));
  const [mode, setMode] = useState<PickMode>(endpoint.event_types.includes(everyEvent) ? "all" : "some");
  const [picked, setPicked] = useState<string[] | null>(null);
  const chosen = picked ?? coveredTypes(endpoint.event_types, catalog);
  const save = useMutation({
    mutationFn: () =>
      updateWebhookEndpoint(workspace, endpoint.id, {
        event_types: mode === "all" ? [everyEvent] : subscriptionFor(chosen, catalog),
      }),
    onSuccess: onSaved,
  });

  return (
    <Modal isOpen size="lg" onOpenChange={(open) => !open && onClose()}>
      <ModalHeader icon={RiWebhookLine} title="Events" description="What this endpoint hears." />
      <ModalBody className="flex flex-col gap-4">
        {save.error ? (
          <Notification status="error" title={save.error.message} dismissible={false} className="w-full" />
        ) : null}
        <WebhookEventPicker
          catalog={catalog}
          mode={mode}
          picked={chosen}
          onModeChange={setMode}
          onPickedChange={setPicked}
        />
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button
          loading={save.isPending}
          disabled={mode === "some" && chosen.length === 0}
          onClick={() => save.mutate()}
        >
          Save
        </Button>
      </ModalFooter>
    </Modal>
  );
}
