/*
 * The channels step: where customers reach the team. The messenger comes
 * first and is recommended, since a snippet on the team's site is the
 * quickest way to get a first conversation. Email takes the support
 * address and answers with the address to forward it to. Chat apps are
 * listed as coming, so the team sees what will be there.
 */
import { RiChat3Line, RiFileCopyLine, RiMailLine } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { type FormEvent, type ReactNode, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { fieldError } from "@/features/auth/form-errors";
import { createEmailAddress } from "@/lib/api/email";
import { installSnippet, widgetSettingsQuery } from "@/lib/api/widget-settings";

import { StepActions, StepHeader } from "./frame";
import type { StepProps } from "./step-props";

const comingSoon = ["Slack", "WhatsApp", "Telegram", "Discord"];

function ChannelCard({
  icon: Icon,
  title,
  badge,
  description,
  children,
}: {
  icon: typeof RiChat3Line;
  title: string;
  badge?: ReactNode;
  description: string;
  children?: ReactNode;
}) {
  return (
    <section
      aria-label={title}
      className="flex flex-col gap-4 rounded-3xl border border-border-button-default bg-background-secondary-default p-5"
    >
      <div className="flex items-start gap-3">
        <Icon aria-hidden className="mt-0.5 size-5 shrink-0 text-foreground-icon-secondary" />
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <h2 className="text-body-semibold text-text-primary">{title}</h2>
            {badge}
          </div>
          <p className="text-body-2-regular text-text-secondary">{description}</p>
        </div>
      </div>
      {children}
    </section>
  );
}

function MessengerInstall({ workspace }: { workspace: string }) {
  const { data: widgets } = useQuery(widgetSettingsQuery(workspace));
  const [copied, setCopied] = useState(false);
  const widget = widgets?.[0];

  if (!widget) {
    return null;
  }

  const snippet = installSnippet(window.location.origin, widget.id);

  return (
    <div className="flex flex-col gap-3">
      <code
        dir="ltr"
        className="block break-all rounded-xl bg-background-primary-default p-3 font-mono text-body-2-regular text-text-primary"
      >
        {snippet}
      </code>
      <div className="flex items-center gap-3">
        <Button
          variant="secondary"
          size="small"
          leadingIcon={RiFileCopyLine}
          onClick={() => {
            void navigator.clipboard.writeText(snippet).then(() => setCopied(true));
          }}
        >
          {copied ? "Copied" : "Copy snippet"}
        </Button>
        <span className="text-body-2-regular text-text-tertiary">
          Paste it before the closing body tag of your site.
        </span>
      </div>
    </div>
  );
}

function EmailSetup({ workspace }: { workspace: string }) {
  const [address, setAddress] = useState("");
  const add = useMutation({ mutationFn: () => createEmailAddress(workspace, address) });
  const addressError = fieldError(add.error, "address", ["address"], "address");

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    add.mutate();
  };

  if (add.data) {
    return (
      <p className="text-body-2-regular text-text-secondary">
        Forward mail sent to <bdi dir="ltr">{add.data.address}</bdi> to{" "}
        <bdi dir="ltr" className="font-mono text-text-primary">
          {add.data.forwarding_address}
        </bdi>
        . Settings walk you through it for your mail provider.
      </p>
    );
  }

  return (
    <form onSubmit={submit} className="flex items-end gap-2" aria-busy={add.isPending}>
      <Input
        className="flex-1"
        name="address"
        type="email"
        label="Your support address"
        placeholder="support@acme.com"
        inputDir="ltr"
        value={address}
        onChange={(value) => {
          setAddress(value);
          add.reset();
        }}
        isInvalid={Boolean(addressError)}
        errorMessage={addressError}
      />
      <Button size="small" type="submit" variant="secondary" disabled={add.isPending || address.trim() === ""}>
        Add
      </Button>
    </form>
  );
}

export function ChannelsStep({ workspace, label, onNext, onBack }: StepProps) {
  return (
    <div>
      <StepHeader
        label={label}
        title="Connect a channel"
        description="Choose where your customers reach you. You can add more channels later in settings."
      />
      <div className="mt-8 flex flex-col gap-3">
        <ChannelCard
          icon={RiChat3Line}
          title="Messenger"
          badge={
            <Chip color="lime" variant="subtle">
              Recommended
            </Chip>
          }
          description="A chat window on your website. Customers write to you without leaving the page."
        >
          <MessengerInstall workspace={workspace} />
        </ChannelCard>
        <ChannelCard
          icon={RiMailLine}
          title="Email"
          description="Answer mail sent to your support address from the inbox."
        >
          <EmailSetup workspace={workspace} />
        </ChannelCard>
        <div className="flex flex-wrap items-center gap-2 pt-2">
          {comingSoon.map((name) => (
            <Chip key={name} color="neutral" variant="subtle">
              {name} · coming soon
            </Chip>
          ))}
        </div>
      </div>
      <StepActions primary={onNext} primaryLabel="Continue" onBack={onBack} />
    </div>
  );
}
