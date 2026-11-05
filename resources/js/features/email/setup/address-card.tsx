/*
 * One address with its forwarding setup: where to forward, the steps for
 * the mail service it lives in, whether mail arrived, and the settings
 * of the address for whoever may manage it.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { Chip } from "@/components/base/badges/chip";
import { Choice, Toggle } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { type EmailAddress, updateEmailAddress } from "@/lib/api/email";

import { Activity } from "./activity";
import { CopyButton } from "./copy-button";
import { Failure } from "./failure";
import { ForwardingGuide } from "./forwarding-guide";
import { ForwardingTest } from "./forwarding-test";

/*
 * Status says whether mail reached the address yet.
 */
function Status({ address }: { address: EmailAddress }) {
  return address.inbound_verified_at ? (
    <Chip variant="caption" color="lime">
      Receiving
    </Chip>
  ) : (
    <Chip variant="caption" color="neutral">
      Waiting for first email
    </Chip>
  );
}

const bulkLabels = { close: "Close", open: "Keep open", drop: "Drop" } as const;

/*
 * AddressCard is one address with its forwarding setup.
 */
export function AddressCard({
  address,
  manage,
  readRaw,
}: {
  address: EmailAddress;
  manage: boolean;
  readRaw: boolean;
}) {
  const me = useAccount();
  const queryClient = useQueryClient();
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["workspace", me.workspace.id, "email-addresses"] });

  const change = useMutation({
    mutationFn: (changes: Parameters<typeof updateEmailAddress>[2]) =>
      updateEmailAddress(me.workspace.id, address.id, changes),
    onSuccess: refresh,
  });

  const confirmation = address.forwarding_confirmation;

  return (
    <Section title={address.address} description={`${address.inbox.name} inbox`} action={<Status address={address} />}>
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
          <ForwardingTest address={address} />
          <Row label="Quote the previous message">
            <Toggle
              label="Quote the previous message"
              on={address.settings.quote_previous}
              onChange={(on) => change.mutate({ settings: { quote_previous: on } })}
            />
          </Row>
          <Row label="Bulk mail" description="Newsletters and notifications that start no thread.">
            <Choice
              label="Bulk mail"
              options={Object.values(bulkLabels)}
              value={bulkLabels[address.settings.bulk_mail_handling]}
              onChange={(label) => {
                const handling = (Object.keys(bulkLabels) as (keyof typeof bulkLabels)[]).find(
                  (key) => bulkLabels[key] === label,
                );
                if (handling) {
                  change.mutate({ settings: { bulk_mail_handling: handling } });
                }
              }}
            />
          </Row>
        </>
      ) : null}
      {readRaw ? <Activity address={address} /> : null}
      <Failure error={change.error} />
    </Section>
  );
}
