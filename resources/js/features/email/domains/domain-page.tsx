/*
 * One sending domain: where it stands, what to fix, the records to
 * publish and the steps for its DNS host. The page follows the domain
 * live: every check the server runs refreshes it over the workspace's
 * channel, and while the connection is down it reads the domain again
 * every thirty seconds. Removing a domain addresses still send from asks
 * again before those addresses fall back to the dimmi address.
 */
import { RiArrowLeftLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { Failure } from "@/features/email/setup/failure";
import { Danger } from "@/features/settings/controls";
import { Row, Section, SettingsPage } from "@/features/settings/layout";
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

function DomainView({ domain, permissions }: { domain: EmailDomain; permissions: string[] }) {
  const workspace = useAccount().workspace.id;
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const changes = useDomainChanges(workspace, domain.id);
  const [removing, setRemoving] = useState<"ask" | "force" | null>(null);
  const manage = permissions.includes("email:manage");
  const error = changes.check.error ?? changes.rotate.error ?? changes.host.error ?? changes.remove.error;

  const remove = async (force: boolean) => {
    try {
      await changes.remove.mutateAsync(force);
    } catch (failure) {
      if (failure instanceof ApiError && failure.code === "domain_in_use") {
        setRemoving("force");
      }
      return;
    }
    await navigate({ to: "/d/settings/$section", params: { section: "email" } });
    await queryClient.invalidateQueries({ queryKey: domainsKey(workspace) });
  };

  return (
    <SettingsPage title={domain.domain} description={statusLines[domain.status]}>
      <BackToEmail />
      <Section
        title="Status"
        action={
          manage ? (
            <Button size="small" disabled={changes.check.isPending} onClick={() => changes.check.mutate()}>
              Check now
            </Button>
          ) : null
        }
      >
        <Row label="Domain" description={domain.failure_message ?? undefined}>
          <DomainStatusChip status={domain.status} />
        </Row>
        {domain.warnings.map((warning) => (
          <Row key={`${warning.code}-${warning.host ?? ""}`} label="To fix" description={warning.message} />
        ))}
        <Failure error={error} />
      </Section>
      <DomainRecords records={domain.records} />
      <HostGuide guide={domain.host_guide} manage={manage} onPick={(host) => changes.host.mutate(host)} />
      {manage ? (
        <Section title="Manage">
          {domain.driver === "smtp" && permissions.includes("email:rotate_dkim") ? (
            <Row label="Signing key" description="Publish a new key. The current one signs until the new one shows.">
              <Button
                variant="secondary"
                size="small"
                disabled={changes.rotate.isPending}
                onClick={() => changes.rotate.mutate()}
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
        <Failure error={error} />
      </SettingsPage>
    );
  }

  return <DomainView domain={data} permissions={caller?.member?.permissions ?? []} />;
}
