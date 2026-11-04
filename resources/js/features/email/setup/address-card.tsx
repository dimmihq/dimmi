/*
 * One address with its forwarding setup: where to forward, the steps for
 * the mail service it lives in, whether mail arrived, and the settings
 * of the address for whoever may manage it.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Choice, Toggle } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { type EmailAddress, sendEmailAddressTest, updateEmailAddress } from "@/lib/api/email";

import { Activity } from "./activity";
import { CopyButton } from "./copy-button";
import { Failure } from "./failure";

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
  const [guide, setGuide] = useState(address.setup_guides[0]?.name ?? "");
  const steps = address.setup_guides.find((g) => g.name === guide)?.steps ?? [];
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["workspace", me.workspace.id, "email-addresses"] });

  const test = useMutation({ mutationFn: () => sendEmailAddressTest(me.workspace.id, address.id) });
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
      <Row label="Set up forwarding" description="Steps for the mail service your address lives in.">
        <Choice
          label="Mail service"
          options={address.setup_guides.map((g) => g.name)}
          value={guide}
          onChange={setGuide}
        />
      </Row>
      <ol className="flex list-decimal flex-col gap-1 border-b border-separator-border py-3 ps-5 pe-3 text-body-2-regular text-text-secondary">
        {steps.map((step) => (
          <li key={step}>{step}</li>
        ))}
      </ol>
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
          <Row
            label="Forwarding test"
            description={
              test.data
                ? `Sent to ${test.data.sent_to}. It shows as receiving once it comes back.`
                : "Send a test email to your address to check the forward."
            }
          >
            <Button variant="secondary" size="small" disabled={test.isPending} onClick={() => test.mutate()}>
              Send test email
            </Button>
          </Row>
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
      <Failure error={test.error ?? change.error} />
    </Section>
  );
}
