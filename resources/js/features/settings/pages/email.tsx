/*
 * Email settings on the server: each address the workspace receives
 * mail at, the forwarding address to point it at with a copy button, the
 * steps for the mail service it lives in, and whether mail has arrived
 * yet. A forwarding test proves the forward works; a confirmation code
 * Gmail mails here shows up for whoever finishes the setup. Changing
 * addresses follows the email:manage permission the server gives.
 */
import { RiCheckLine, RiFileCopyLine, RiMailLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { ApiError } from "@/lib/api/client";
import {
  createEmailAddress,
  type EmailAddress,
  emailAddressesQuery,
  inboundEmailsQuery,
  sendEmailAddressTest,
  updateEmailAddress,
} from "@/lib/api/email";
import { callerQuery } from "@/lib/api/workspace";

import { Choice, Toggle } from "../controls";
import { Empty, Glyph, Item, Row, Section, SettingsPage } from "../layout";
import { useAccount } from "../use-account";

/*
 * Failure shows what went wrong with the last change, or nothing.
 */
function Failure({ error }: { error: Error | null }) {
  if (!error) {
    return null;
  }

  return (
    <p role="alert" className="px-3 text-body-2-regular text-text-error-primary">
      {error instanceof ApiError ? (Object.values(error.fields)[0] ?? error.message) : error.message}
    </p>
  );
}

/*
 * CopyButton copies a value and says so for a moment.
 */
function CopyButton({ value, label }: { value: string; label: string }) {
  const [copied, setCopied] = useState(false);

  return (
    <Button
      variant="secondary"
      size="small"
      aria-label={label}
      leadingIcon={copied ? RiCheckLine : RiFileCopyLine}
      onClick={() => {
        void navigator.clipboard?.writeText(value).catch(() => undefined);
        setCopied(true);
        window.setTimeout(() => setCopied(false), 2000);
      }}
    >
      {copied ? "Copied" : "Copy"}
    </Button>
  );
}

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
function AddressCard({ address, manage, readRaw }: { address: EmailAddress; manage: boolean; readRaw: boolean }) {
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

/*
 * Activity lists the last emails that reached the address.
 */
function Activity({ address }: { address: EmailAddress }) {
  const me = useAccount();
  const { data } = useQuery(inboundEmailsQuery(me.workspace.id, address.id));

  if (!data || data.length === 0) {
    return <Empty>No email has arrived yet.</Empty>;
  }

  return (
    <>
      {data.map((mail) => (
        <Item
          key={mail.id}
          mark={<Glyph icon={RiMailLine} />}
          title={mail.subject ?? "(no subject)"}
          detail={`${mail.from ?? "Unknown sender"} · ${mail.ignore_reason ? `ignored, ${mail.ignore_reason.replaceAll("_", " ")}` : mail.status}`}
        />
      ))}
    </>
  );
}

/*
 * AddAddress connects another public address.
 */
function AddAddress() {
  const me = useAccount();
  const queryClient = useQueryClient();
  const [address, setAddress] = useState("");
  const add = useMutation({
    mutationFn: () => createEmailAddress(me.workspace.id, address.trim()),
    onSuccess: () => {
      setAddress("");
      return queryClient.invalidateQueries({ queryKey: ["workspace", me.workspace.id, "email-addresses"] });
    },
  });

  return (
    <Section title="Add an address" description="Connect another address your customers write to.">
      <Row label="Address">
        <Input
          aria-label="New email address"
          size="small"
          placeholder="support@yourcompany.com"
          value={address}
          onChange={setAddress}
          className="w-64"
        />
        <Button size="small" disabled={!address.trim() || add.isPending} onClick={() => add.mutate()}>
          Add address
        </Button>
      </Row>
      <Failure error={add.error} />
    </Section>
  );
}

export function EmailSettings() {
  const me = useAccount();
  const { data: addresses, error } = useQuery(emailAddressesQuery(me.workspace.id));
  const { data: caller } = useQuery(callerQuery(me.workspace.id));
  const permissions = caller?.member?.permissions ?? [];
  const manage = permissions.includes("email:manage");

  return (
    <SettingsPage title="Email" description="Forward your support address to dimmi and reply from it.">
      {addresses && addresses.length === 0 ? (
        <Section title="Addresses">
          <Empty>No address is connected yet.</Empty>
        </Section>
      ) : null}
      {addresses?.map((address) => (
        <AddressCard
          key={address.id}
          address={address}
          manage={manage}
          readRaw={permissions.includes("email:read_raw")}
        />
      ))}
      {manage && addresses ? <AddAddress /> : null}
      <Failure error={error} />
    </SettingsPage>
  );
}
