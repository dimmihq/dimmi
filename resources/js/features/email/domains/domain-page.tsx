/*
 * One sending domain: where it stands, what to fix, the records to
 * publish and the steps for its DNS host. The page follows the domain
 * live: every check the server runs refreshes it over the workspace's
 * channel, and while the connection is down it reads the domain again
 * every thirty seconds. Removing a domain addresses still send from asks
 * again before those addresses fall back to the dimmi address. The DNS
 * host picked waits in a draft for the save bar at the foot; leaving
 * with it unsaved asks first. Checking, rotating the key and removing
 * are actions and happen at once.
 */
import { RiArrowLeftLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useCallback, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { Failure } from "@/features/email/setup/failure";
import { Danger } from "@/features/settings/controls";
import { Row, Section, SettingsPage } from "@/features/settings/layout";
import { SaveBar } from "@/features/settings/save-bar";
import { useAccount } from "@/features/settings/use-account";
import { ApiError } from "@/lib/api/client";
import {
  checkEmailDomain,
  deleteEmailDomain,
  domainsKey,
  type EmailDomain,
  emailDomainQuery,
  rotateEmailDomainKey,
  updateEmailDomain,
} from "@/lib/api/email-domains";
import { callerQuery } from "@/lib/api/workspace";
import { useRealtime } from "@/lib/realtime/context";
import { channels } from "@/lib/realtime/keys";
import { useChannel } from "@/lib/realtime/use-channel";

import { DNSSetup } from "./dns-setup";
import { DomainRecords } from "./domain-records";
import { DomainStatusChip } from "./domain-status";
import { HostGuide } from "./host-guide";

const statusLines: Record<EmailDomain["status"], string> = {
  pending: "Add the records below, then check the domain.",
  verifying: "We check the records every few minutes. This page updates by itself.",
  verified: "Replies from addresses on this domain come from it.",
  degraded: "Some records are gone, so replies come from your dimmi address until they are back.",
  failed: "Replies come from your dimmi address. Fix the records and check again.",
};

function BackToEmail() {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "email" } })}
      >
        Email settings
      </Button>
    </div>
  );
}

/*
 * useLiveDomain reads the domain and refreshes it whenever the server
 * says it changed, polling only while the connection is down.
 */
function useLiveDomain(workspace: string, domain: string) {
  const queryClient = useQueryClient();
  const { state } = useRealtime();
  const query = useQuery({
    ...emailDomainQuery(workspace, domain),
    refetchInterval: state === "connected" ? false : 30_000,
  });

  useChannel(channels.workspace(workspace), {
    onEnvelope: (envelope) => {
      if (envelope.event.startsWith("email_domain.")) {
        void queryClient.invalidateQueries({ queryKey: domainsKey(workspace) });
      }
    },
  });

  return query;
}

/*
 * useDomainChanges holds the calls that change the domain, each putting
 * the answer in the cache.
 */
function useDomainChanges(workspace: string, domain: string) {
  const queryClient = useQueryClient();
  const keep = (changed: EmailDomain) => {
    queryClient.setQueryData(emailDomainQuery(workspace, domain).queryKey, changed);
    return queryClient.invalidateQueries({ queryKey: domainsKey(workspace), exact: true });
  };

  return {
    check: useMutation({ mutationFn: () => checkEmailDomain(workspace, domain), onSuccess: keep }),
    rotate: useMutation({ mutationFn: () => rotateEmailDomainKey(workspace, domain), onSuccess: keep }),
    host: useMutation({
      mutationFn: (host: string) => updateEmailDomain(workspace, domain, { dns_host: host }),
      onSuccess: keep,
    }),
    remove: useMutation({ mutationFn: (force: boolean) => deleteEmailDomain(workspace, domain, force) }),
  };
}

/*
 * sendersOf reads which broadcasts and series still send from a domain
 * out of the refusal to remove it, as "Broadcast: name" lines.
 */
function sendersOf(failure: ApiError): string[] {
  const senders = failure.details.senders;
  if (!Array.isArray(senders)) {
    return [];
  }

  return senders.map((line) => {
    const text = String(line);
    return text.charAt(0).toUpperCase() + text.slice(1);
  });
}

function DomainView({ domain, permissions }: { domain: EmailDomain; permissions: string[] }) {
  const workspace = useAccount().workspace.id;
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const changes = useDomainChanges(workspace, domain.id);
  const [removing, setRemoving] = useState<"ask" | "force" | null>(null);
  const [blocked, setBlocked] = useState<string[] | null>(null);
  const [rotating, setRotating] = useState(false);
  const [host, setHost] = useState<string | null>(null);
  const manage = permissions.includes("email:manage");
  const applied = useCallback(() => changes.check.mutate(), [changes.check.mutate]);
  const error = changes.check.error ?? changes.rotate.error ?? changes.remove.error;
  const dirty = host !== null;
  useBlocker({
    shouldBlockFn: () => dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => dirty,
  });
  const discard = () => {
    changes.host.reset();
    setHost(null);
  };

  const remove = async (force: boolean) => {
    try {
      await changes.remove.mutateAsync(force);
    } catch (failure) {
      if (failure instanceof ApiError && failure.code === "domain_in_use") {
        setRemoving("force");
      }
      if (failure instanceof ApiError && failure.code === "domain_sending") {
        setRemoving(null);
        setBlocked(sendersOf(failure));
      }
      return;
    }
    setHost(null);
    await navigate({ to: "/d/settings/$section", params: { section: "email" } });
    await queryClient.invalidateQueries({ queryKey: domainsKey(workspace) });
  };

  return (
    <SettingsPage
      title={domain.domain}
      description={statusLines[domain.status]}
      footer={
        <SaveBar
          dirty={dirty}
          saving={changes.host.isPending}
          error={changes.host.error?.message}
          onSave={() => {
            if (host !== null) {
              changes.host.mutate(host, { onSuccess: () => setHost(null) });
            }
          }}
          onDiscard={discard}
        />
      }
    >
      <BackToEmail />
      <Section
        title="Status"
        action={
          manage ? (
            <Button size="small" loading={changes.check.isPending} onClick={() => changes.check.mutate()}>
              Check now
            </Button>
          ) : null
        }
      >
        <Row label="Domain" description={domain.failure_message ?? undefined}>
          <DomainStatusChip status={domain.status} />
        </Row>
        <DNSSetup workspace={workspace} domain={domain} manage={manage} onApplied={applied} />
        {domain.warnings.map((warning) => (
          <Row key={`${warning.code}-${warning.host ?? ""}`} label="To fix" description={warning.message} />
        ))}
        <Failure error={error} />
      </Section>
      <DomainRecords records={domain.records} />
      <HostGuide
        guide={domain.host_guide}
        manage={manage}
        selected={host ?? domain.host_guide.key}
        onPick={(picked) => setHost(picked === domain.host_guide.key ? null : picked)}
      />
      {manage ? (
        <Section title="Manage">
          {domain.driver === "smtp" && permissions.includes("email:rotate_dkim") ? (
            <Row label="Signing key" description="Publish a new key. The current one signs until the new one shows.">
              <Button
                variant="secondary"
                size="small"
                loading={changes.rotate.isPending}
                onClick={() => setRotating(true)}
              >
                Rotate key
              </Button>
            </Row>
          ) : null}
          <Row label="Remove domain" description="Replies from its addresses come from your dimmi address.">
            <Danger onPress={() => setRemoving("ask")}>Remove domain</Danger>
          </Row>
        </Section>
      ) : null}
      <AlertDialog
        isOpen={removing !== null}
        onOpenChange={(open) => setRemoving(open ? removing : null)}
        title={`Remove ${domain.domain}?`}
        description={
          removing === "force"
            ? "Addresses on this domain still send from it. Remove it anyway and they reply from your dimmi address."
            : "Replies stop coming from this domain."
        }
        confirmLabel={removing === "force" ? "Remove anyway" : "Remove domain"}
        onConfirm={() => remove(removing === "force")}
      />
      <AlertDialog
        isOpen={rotating}
        onOpenChange={setRotating}
        variant="default"
        title="Rotate the signing key?"
        description={
          "A new key record appears below. Add it at your DNS host. The current key keeps signing until " +
          "the new one is found, so nothing bounces meanwhile."
        }
        confirmLabel="Rotate key"
        onConfirm={() => changes.rotate.mutateAsync()}
      />
      <AlertDialog
        isOpen={blocked !== null}
        onOpenChange={(open) => (open ? undefined : setBlocked(null))}
        isDismissable
        title={`${domain.domain} is still sending`}
        description="Stop or archive these, or move them to another address, then remove the domain."
        confirmLabel="Got it"
        cancelLabel="Close"
        onConfirm={() => setBlocked(null)}
      >
        <ul className="flex list-disc flex-col gap-1 ps-5 text-body-2-regular text-text-primary">
          {(blocked ?? []).map((name) => (
            <li key={name}>{name}</li>
          ))}
        </ul>
      </AlertDialog>
    </SettingsPage>
  );
}

/*
 * DomainPage is the sending domain the route names.
 */
export function DomainPage({ domain }: { domain: string }) {
  const workspace = useAccount().workspace.id;
  const { data, error } = useLiveDomain(workspace, domain);
  const { data: caller } = useQuery(callerQuery(workspace));

  if (!data) {
    return (
      <SettingsPage title="Sending domain">
        <BackToEmail />
        {error ? <Failure error={error} /> : <Section title="" loading rows={4} />}
      </SettingsPage>
    );
  }

  return <DomainView domain={data} permissions={caller?.member?.permissions ?? []} />;
}
