/*
 * A help center's own domain, such as help.acme.com. The team types the
 * domain, then creates the two records shown: a CNAME pointing it here
 * and a TXT record proving they own it. We check in the background, more
 * often at first, and the status says where it stands and what to fix.
 * Once verified, the help center answers there with its own certificate
 * and its links point there; the old address keeps working.
 */
import { RiCheckLine, RiFileCopyLine, RiGlobalLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Notification } from "@/components/base/notification/notification";
import {
  checkHelpCenterDomain,
  type DNSRecord,
  type DomainStatus,
  helpCenterDomainQuery,
  removeHelpCenterDomain,
  setHelpCenterDomain,
} from "@/lib/api/help-center-sites";
import { type HelpCenter, helpCentersQuery } from "@/lib/api/help-centers";

import { Action, Danger, Field } from "./controls";
import { Glyph, Item, Row, Section } from "./layout";
import { useCopy } from "./use-copy";

const statuses: Record<DomainStatus, { label: string; color: "lime" | "orange" | "rose" }> = {
  pending: { label: "Checking", color: "orange" },
  verified: { label: "Live", color: "lime" },
  failed: { label: "Failed", color: "rose" },
};

export function HelpCenterDomain({ workspace, center }: { workspace: string; center: HelpCenter }) {
  const queryClient = useQueryClient();
  const query = helpCenterDomainQuery(workspace, center.id);
  const { data: domain, isPending } = useQuery(query);
  const [hostname, setHostname] = useState("");
  const settle = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: query.queryKey }),
      queryClient.invalidateQueries({ queryKey: helpCentersQuery(workspace).queryKey }),
    ]);
  const add = useMutation({
    mutationFn: () => setHelpCenterDomain(workspace, center.id, hostname.trim()),
    onSuccess: settle,
  });
  const check = useMutation({ mutationFn: () => checkHelpCenterDomain(workspace, center.id), onSuccess: settle });
  const remove = useMutation({ mutationFn: () => removeHelpCenterDomain(workspace, center.id), onSuccess: settle });
  const error = add.error ?? check.error ?? remove.error;

  return (
    <Section
      title="Custom domain"
      description="Serve this help center on a domain you own, such as help.yourcompany.com."
      loading={isPending}
      rows={1}
    >
      {error ? (
        <div className="py-2.5 pe-2.5">
          <Notification status="error" title={error.message} dismissible={false} role="alert" />
        </div>
      ) : null}
      {domain ? (
        <>
          <Item
            mark={<Glyph icon={RiGlobalLine} />}
            title={domain.hostname}
            detail={
              domain.status === "verified"
                ? "Readers reach this help center here. Its old address still works."
                : (domain.failure_reason ?? "Create the two records below. We check every few minutes.")
            }
          >
            <Chip variant="caption" color={statuses[domain.status].color}>
              {statuses[domain.status].label}
            </Chip>
            {domain.status === "verified" ? null : (
              <Action loading={check.isPending} onPress={() => check.mutate()}>
                Check now
              </Action>
            )}
          </Item>
          {domain.records.map((record) => (
            <RecordRow key={record.type} record={record} />
          ))}
          <Row label="Remove domain" description="The help center answers only at its own address again.">
            <Danger onPress={() => remove.mutate()}>Remove</Danger>
          </Row>
        </>
      ) : (
        <Row label="Domain" description="A subdomain you own. You then add two DNS records we show you.">
          <div className="flex items-center gap-2">
            <Field label="Domain" value={hostname} placeholder="help.yourcompany.com" onChange={setHostname} />
            <Action loading={add.isPending} onPress={() => add.mutate()}>
              Add domain
            </Action>
          </div>
        </Row>
      )}
    </Section>
  );
}

/*
 * RecordRow is one record to create at the DNS provider, with its name
 * and value ready to copy.
 */
function RecordRow({ record }: { record: DNSRecord }) {
  const name = useCopy();
  const value = useCopy();
  const purpose = record.type === "CNAME" ? "Points your domain here." : "Proves you own the domain.";

  return (
    <Row label={`${record.type} record`} description={purpose}>
      <div className="flex min-w-0 flex-col items-end gap-1">
        <Copyable label="Name" text={record.name} copied={name.copied} onCopy={() => name.copy(record.name)} />
        <Copyable label="Value" text={record.value} copied={value.copied} onCopy={() => value.copy(record.value)} />
      </div>
    </Row>
  );
}

function Copyable({
  label,
  text,
  copied,
  onCopy,
}: {
  label: string;
  text: string;
  copied: boolean;
  onCopy: () => void;
}) {
  return (
    <span className="flex min-w-0 max-w-full items-center gap-1">
      <span className="shrink-0 text-caption-1-regular text-text-tertiary">{label}</span>
      <code className="min-w-0 truncate rounded-md bg-background-secondary-default px-1.5 py-0.5 font-mono text-caption-1-regular text-text-primary">
        {text}
      </code>
      <IconButton
        size="small"
        icon={copied ? RiCheckLine : RiFileCopyLine}
        aria-label={copied ? `${label} copied` : `Copy ${label.toLowerCase()}`}
        onClick={onCopy}
      />
    </span>
  );
}
