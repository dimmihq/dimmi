/*
 * The workspace's webhook endpoints on screen: where each posts, what it
 * hears and how it is doing, each opening its own page. Adding one asks
 * for the URL and the events, picked from the catalog. Its signing
 * secret shows once right after, with a button to copy it; it can be
 * revealed again from the endpoint's page.
 */
import { RiArrowRightSLine, RiCheckLine, RiFileCopyLine, RiWebhookLine } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import { ApiError } from "@/lib/api/client";
import {
  type CreatedWebhookEndpoint,
  createWebhookEndpoint,
  webhookEndpointsQuery,
  webhookEventTypesQuery,
  webhooksKey,
} from "@/lib/api/webhooks";

import { Action } from "../controls";
import { Empty, Glyph, Item, Section, SettingsPage } from "../layout";
import { rowLink } from "../row-link";
import { useAccount } from "../use-account";
import { useCanManageWebhooks } from "../use-can-manage-webhooks";
import { useCopy } from "../use-copy";
import { type PickMode, WebhookEventPicker } from "../webhook-event-picker";
import { endpointState, everyEvent, subscriptionFor, subscriptionLine, validURL } from "../webhooks";

export function WebhooksSettings() {
  const workspace = useAccount().workspace.id;
  const queryClient = useQueryClient();
  const manage = useCanManageWebhooks(workspace);
  const { data: endpoints, error } = useQuery(webhookEndpointsQuery(workspace));
  const [adding, setAdding] = useState(false);
  const [created, setCreated] = useState<CreatedWebhookEndpoint | null>(null);

  return (
    <SettingsPage title="Webhooks" description="Signed POST requests to your endpoints when things happen.">
      <Section
        title="Endpoints"
        action={manage ? <Action onPress={() => setAdding(true)}>New endpoint</Action> : null}
        loading={!endpoints && !error}
        rows={2}
      >
        {error ? <Empty>{error.message}</Empty> : null}
        {endpoints?.length === 0 ? <Empty>No endpoint yet</Empty> : null}
        {endpoints?.map((endpoint) => {
          const state = endpointState(endpoint);

          return (
            <Item
              key={endpoint.id}
              mark={<Glyph icon={RiWebhookLine} />}
              title={
                <Link to="/d/settings/webhooks/$endpoint" params={{ endpoint: endpoint.id }} className={rowLink}>
                  {endpoint.url}
                </Link>
              }
              detail={[subscriptionLine(endpoint.event_types), endpoint.description].filter(Boolean).join(" · ")}
            >
              <Chip variant="caption" color={state.color}>
                {state.label}
              </Chip>
              <RiArrowRightSLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
            </Item>
          );
        })}
      </Section>
      {adding ? (
        <NewEndpointDialog
          workspace={workspace}
          onClose={() => setAdding(false)}
          onCreated={(made) => {
            setAdding(false);
            setCreated(made);
            void queryClient.invalidateQueries({ queryKey: webhooksKey(workspace) });
          }}
        />
      ) : null}
      {created ? <EndpointSecretDialog created={created} onClose={() => setCreated(null)} /> : null}
    </SettingsPage>
  );
}

function NewEndpointDialog({
  workspace,
  onClose,
  onCreated,
}: {
  workspace: string;
  onClose: () => void;
  onCreated: (made: CreatedWebhookEndpoint) => void;
}) {
  const { data: catalog = [] } = useQuery(webhookEventTypesQuery(workspace));
  const [url, setURL] = useState("");
  const [description, setDescription] = useState("");
  const [mode, setMode] = useState<PickMode>("some");
  const [picked, setPicked] = useState<string[]>([]);
  const [failure, setFailure] = useState<Error | null>(null);
  const [pending, setPending] = useState(false);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFailure(null);
    setPending(true);

    try {
      onCreated(
        await createWebhookEndpoint(workspace, {
          url: url.trim(),
          description: description.trim(),
          event_types: mode === "all" ? [everyEvent] : subscriptionFor(picked, catalog),
        }),
      );
    } catch (error) {
      setFailure(error instanceof Error ? error : new Error("Something went wrong. Try again."));
    } finally {
      setPending(false);
    }
  };

  const fields = failure instanceof ApiError ? failure.fields : {};
  const general = failure && !fields.url ? failure.message : null;
  const ready = validURL(url) && (mode === "all" || picked.length > 0);

  return (
    <Modal isOpen size="lg" onOpenChange={(open) => !open && onClose()}>
      <form onSubmit={submit} className="flex min-h-0 flex-col gap-4">
        <ModalHeader
          icon={RiWebhookLine}
          title="New endpoint"
          description="dimmi signs every request, so your server can check it came from here."
        />
        <ModalBody className="flex flex-col gap-5">
          {general ? (
            <Notification status="error" title={general} dismissible={false} role="alert" className="w-full" />
          ) : null}
          <Input
            label="URL"
            placeholder="https://example.com/webhooks/dimmi"
            value={url}
            onChange={setURL}
            inputDir="ltr"
            isRequired
            autoFocus
            isInvalid={Boolean(fields.url)}
            errorMessage={fields.url}
          />
          <Input
            label="Description"
            placeholder="CRM sync"
            value={description}
            onChange={setDescription}
            maxLength={255}
          />
          <div className="flex flex-col gap-1.5">
            <span className="text-body-medium text-text-secondary">Events</span>
            <WebhookEventPicker
              catalog={catalog}
              mode={mode}
              picked={picked}
              onModeChange={setMode}
              onPickedChange={setPicked}
            />
            {fields.event_types ? (
              <span role="alert" className="text-body-2-regular text-text-error-primary">
                {fields.event_types}
              </span>
            ) : null}
          </div>
        </ModalBody>
        <ModalFooter>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" disabled={pending || !ready}>
            {pending ? "Adding…" : "Add endpoint"}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

/*
 * EndpointSecretDialog shows a new endpoint's signing secret once.
 */
function EndpointSecretDialog({ created, onClose }: { created: CreatedWebhookEndpoint; onClose: () => void }) {
  return (
    <Modal isOpen isDismissable={false} onOpenChange={(open) => !open && onClose()}>
      <ModalHeader
        icon={RiWebhookLine}
        title="Copy the signing secret"
        description="Your server checks each request's signature with it. You can reveal it again later."
        showCloseButton={false}
      />
      <ModalBody className="flex flex-col gap-1.5">
        <SecretField label={created.endpoint.url} secret={created.secret} />
      </ModalBody>
      <ModalFooter>
        <Button onClick={onClose}>Done</Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * SecretField shows a secret read-only with a button to copy it.
 */
export function SecretField({ label, secret }: { label: string; secret: string }) {
  const { copied, copy } = useCopy();

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-end gap-2">
        <Input
          label={label}
          value={secret}
          isReadOnly
          inputDir="ltr"
          className="min-w-0 flex-1"
          fieldClassName="font-mono"
        />
        <IconButton
          icon={copied ? RiCheckLine : RiFileCopyLine}
          aria-label={copied ? "Copied" : "Copy secret"}
          onClick={() => void copy(secret)}
        />
      </div>
      <span aria-live="polite" className="text-body-2-regular text-text-tertiary">
        {copied ? "Copied to the clipboard." : "Keep it in your server's secrets, never in code."}
      </span>
    </div>
  );
}
