/*
 * One address with its forwarding setup: where to forward, the steps for
 * the mail service it lives in, whether mail arrived, and the settings
 * of the address for whoever may manage it, among them the default that
 * new mail leaves from and how replies look. The settings are held in
 * the page's draft and sent with its save bar; making the address the
 * default is an action of its own and happens at once.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Choice, Toggle } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { type EmailAddress, updateEmailAddress } from "@/lib/api/email";

import { Activity } from "./activity";
import { AddressStatus } from "./address-status";
import { CopyButton } from "./copy-button";
import { Failure } from "./failure";
import { ForwardingGuide } from "./forwarding-guide";
import { ForwardingTest } from "./forwarding-test";
import { TemplateSettings } from "./template-settings";

const bulkLabels = { close: "Close", open: "Keep open", drop: "Drop" } as const;

type AddressSettings = EmailAddress["settings"];

/*
 * fallbackColor is the brand color shown before one is picked.
 */
const fallbackColor = "#171717";

/*
 * savedSettings reads an address's settings as the controls show them,
 * filling a reply style never picked with its defaults.
 */
function savedSettings(settings: AddressSettings): AddressSettings {
  return {
    ...settings,
    template: settings.template ?? "plain",
    brand_color: settings.brand_color || fallbackColor,
    footer_markdown: settings.footer_markdown ?? "",
  };
}

/*
 * useAddressDraft holds the address's settings changed but not saved
 * yet. A setting put back to its saved value drops out of the draft.
 * send writes what changed in one call; clear forgets it after a save or
 * on Discard. generation counts the clears, so controls that keep their
 * own value, such as the color well, start over from the saved one.
 */
export function useAddressDraft(workspace: string, address: EmailAddress | undefined) {
  const [pending, setPending] = useState<Partial<AddressSettings>>({});
  const [generation, setGeneration] = useState(0);
  const saved = address ? savedSettings(address.settings) : undefined;
  const dirty = Object.keys(pending).length > 0;

  return {
    settings: saved ? { ...saved, ...pending } : undefined,
    generation,
    dirty,
    set: <K extends keyof AddressSettings>(key: K, value: AddressSettings[K]) =>
      setPending(({ [key]: _, ...rest }) => (saved && saved[key] === value ? rest : { ...rest, [key]: value })),
    send: () => (address && dirty ? updateEmailAddress(workspace, address.id, { settings: pending }) : null),
    clear: () => {
      setPending({});
      setGeneration((count) => count + 1);
    },
  };
}

export type AddressDraft = ReturnType<typeof useAddressDraft>;

/*
 * AddressCard is one address with its forwarding setup.
 */
export function AddressCard({
  address,
  manage,
  readRaw,
  draft,
  title = address.address,
}: {
  address: EmailAddress;
  manage: boolean;
  readRaw: boolean;
  draft: AddressDraft;
  title?: string;
}) {
  const me = useAccount();
  const queryClient = useQueryClient();
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["workspace", me.workspace.id, "email-addresses"] });

  const makeDefault = useMutation({
    mutationFn: () => updateEmailAddress(me.workspace.id, address.id, { is_default: true }),
    onSuccess: refresh,
  });

  const confirmation = address.forwarding_confirmation;
  const settings = draft.settings ?? savedSettings(address.settings);

  return (
    <Section title={title} description={`${address.inbox.name} inbox`} action={<AddressStatus address={address} />}>
      <Row label="Forwarding address" description="Forward every email sent to your address here.">
        <span className="font-mono text-body-2-regular text-text-primary" data-testid="forwarding-address">
          {address.forwarding_address}
        </span>
        <CopyButton value={address.forwarding_address} label={`Copy ${address.forwarding_address}`} />
      </Row>
      <ForwardingGuide guides={address.setup_guides} />
      {confirmation ? (
        <Row label="Gmail confirmation code" description="Enter it in Gmail to finish turning on forwarding.">
          <span className="font-mono text-body-medium text-text-primary">{confirmation.code}</span>
          {confirmation.confirm_url ? (
            <a
              className="text-body-2-medium text-text-primary underline outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
              href={confirmation.confirm_url}
              target="_blank"
              rel="noopener noreferrer"
            >
              Open Gmail confirmation
            </a>
          ) : null}
        </Row>
      ) : null}
      <Row
        label="Sends as"
        description={
          address.sending_identity.uses_default_sender ? "Replies leave from the forwarding address." : undefined
        }
      >
        <span className="text-body-2-regular text-text-secondary">{address.sending_identity.from}</span>
      </Row>
      {manage ? (
        <>
          {address.is_default ? null : (
            <Row label="Default address" description="New conversations started here leave from the default.">
              <Button
                variant="secondary"
                size="small"
                loading={makeDefault.isPending}
                onClick={() => makeDefault.mutate()}
              >
                Make default
              </Button>
            </Row>
          )}
          <ForwardingTest address={address} />
          <Row label="Quote the previous message">
            <Toggle
              key={`quote-${settings.quote_previous}`}
              label="Quote the previous message"
              on={settings.quote_previous}
              onChange={(on) => draft.set("quote_previous", on)}
            />
          </Row>
          <Row label="Bulk mail" description="Newsletters and notifications that start no thread.">
            <Choice
              key={`bulk-${settings.bulk_mail_handling}`}
              label="Bulk mail"
              options={Object.values(bulkLabels)}
              value={bulkLabels[settings.bulk_mail_handling]}
              onChange={(label) => {
                const handling = (Object.keys(bulkLabels) as (keyof typeof bulkLabels)[]).find(
                  (key) => bulkLabels[key] === label,
                );
                if (handling) {
                  draft.set("bulk_mail_handling", handling);
                }
              }}
            />
          </Row>
          <TemplateSettings workspace={me.workspace.id} address={address} draft={draft} settings={settings} />
        </>
      ) : null}
      {readRaw ? <Activity address={address} /> : null}
      <Failure error={makeDefault.error} />
    </Section>
  );
}
