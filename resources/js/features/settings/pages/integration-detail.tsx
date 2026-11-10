/*
 * One connected tool: how it stands and why when it does not work, each
 * feature it serves and the permissions one still needs, the person's
 * own account on it, the Slack channels it watches, and disconnecting
 * it. Connecting one's own Slack lets replies sent from dimmi post under
 * one's own name and picture; anyone may do it for themselves. Updating
 * permissions and disconnecting take integrations:manage. Disconnecting
 * stops every feature at once and signs every teammate's own account
 * out of it.
 */
import { RiArrowLeftLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import {
  disconnectIntegration,
  type Integration,
  integrationQuery,
  integrationsQuery,
  revokeMemberGrant,
  startMemberGrant,
} from "@/lib/api/integrations";
import { formatDate } from "@/lib/i18n/format";

import { Action, Danger } from "../controls";
import { usePermission } from "../inboxes";
import { featureLabel, healthColor, healthLabels, integrationHealth, providerLook } from "../integrations";
import { PageSkeleton, Row, Section, SettingsPage } from "../layout";
import { useAccount } from "../use-account";
import { useProviderLanding } from "../use-provider-landing";
import { ConnectButton } from "./integration-parts";
import { SlackNotificationsSection } from "./slack-notifications";
import { RoomsSection } from "./slack-rooms";

function BackToIntegrations() {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "integrations" } })}
      >
        All integrations
      </Button>
    </div>
  );
}

/*
 * StatusSection says whether the installation works and, when it does
 * not, what its provider last said.
 */
function StatusSection({ integration }: { integration: Integration }) {
  const health = integrationHealth(integration);
  const checked = integration.last_checked_at ? `Last checked ${formatDate(integration.last_checked_at)}.` : "";

  return (
    <Section title="Status">
      <Row
        label="Connection"
        description={
          integration.last_error ? (
            <span className="text-text-error-primary">{integration.last_error}</span>
          ) : (
            `Connected ${formatDate(integration.installed_at)}. ${checked}`.trim()
          )
        }
      >
        <Chip variant="caption" color={healthColor[health]}>
          {healthLabels[health]}
        </Chip>
      </Row>
      <Row
        label="Teammates' own accounts"
        description="Teammates who connected their own account, so their replies post as them."
      >
        <span className="text-body-regular text-text-secondary">{integration.member_grant_count}</span>
      </Row>
    </Section>
  );
}

/*
 * FeaturesSection lists what the installation serves, each ready or
 * waiting for permissions, with the way to grant them.
 */
function FeaturesSection({
  workspace,
  integration,
  canManage,
}: {
  workspace: string;
  integration: Integration;
  canManage: boolean;
}) {
  const missing = integration.features.some((feature) => feature.missing_scopes.length > 0);
  const action =
    canManage && missing ? (
      <ConnectButton
        workspace={workspace}
        provider={integration.provider}
        label="Update permissions"
        variant="primary"
      />
    ) : undefined;

  return (
    <Section title="Features" description="Everything that uses this connection." action={action}>
      {integration.features.map((feature) => {
        const { label, detail } = featureLabel(feature.name);
        const blocked = feature.missing_scopes.length > 0;

        return (
          <Row
            key={feature.name}
            label={label}
            description={blocked ? `${detail} Needs permission for ${feature.missing_scopes.join(", ")}.` : detail}
          >
            <Chip variant="caption" color={blocked ? "yellow" : "lime"}>
              {blocked ? "Needs permissions" : "Ready"}
            </Chip>
          </Row>
        );
      })}
    </Section>
  );
}

/*
 * OwnAccountSection connects the person's own account on the tool, or
 * signs it out.
 */
function OwnAccountSection({ workspace, integration }: { workspace: string; integration: Integration }) {
  const queryClient = useQueryClient();
  const [leaving, setLeaving] = useState(false);
  const label = providerLook(integration.provider).label;
  const grant = integration.own_grant?.status === "active" ? integration.own_grant : null;

  const connect = useMutation({
    mutationFn: () => startMemberGrant(workspace, integration.id, window.location.pathname),
    onSuccess: ({ authorize_url }) => window.location.assign(authorize_url),
    onError: (error) => toast.error(error.message),
  });

  const revoke = async () => {
    if (!grant) {
      return;
    }

    try {
      await revokeMemberGrant(workspace, integration.id, grant.id);
      toast.success(`Your ${label} account is disconnected.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Something went wrong. Try again.");
    }

    await queryClient.invalidateQueries({ queryKey: integrationsQuery(workspace).queryKey });
  };

  return (
    <Section title={`Your ${label} account`}>
      {grant ? (
        <Row
          label="Replies post as you"
          description={`What you send from dimmi shows in ${label} under your own name and picture.`}
        >
          <Action onPress={() => setLeaving(true)}>Disconnect</Action>
        </Row>
      ) : (
        <Row
          label={`Connect your own ${label}`}
          description={`Replies you send from dimmi post as you, not as the app with your name.`}
        >
          <Action loading={connect.isPending} onPress={() => connect.mutate()}>
            {`Connect your ${label}`}
          </Action>
        </Row>
      )}
      <AlertDialog
        isOpen={leaving}
        onOpenChange={setLeaving}
        title={`Disconnect your ${label} account?`}
        description="Your replies post as the app again, with your name."
        confirmLabel="Disconnect"
        onConfirm={revoke}
      />
    </Section>
  );
}

/*
 * DisconnectSection ends the installation for every feature.
 */
function DisconnectSection({ workspace, integration }: { workspace: string; integration: Integration }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [confirming, setConfirming] = useState(false);
  const label = providerLook(integration.provider).label;

  const disconnect = async () => {
    try {
      await disconnectIntegration(workspace, integration.id);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Something went wrong. Try again.");
      throw error;
    }

    await queryClient.invalidateQueries({ queryKey: integrationsQuery(workspace).queryKey });
    toast.success(`${label} is disconnected.`);
    await navigate({ to: "/d/settings/$section", params: { section: "integrations" } });
  };

  return (
    <Section title="Disconnect">
      <Row
        label={`Disconnect ${label}`}
        description="Every feature stops using it and teammates' own accounts are signed out of it."
      >
        <Danger onPress={() => setConfirming(true)}>Disconnect</Danger>
      </Row>
      <AlertDialog
        isOpen={confirming}
        onOpenChange={setConfirming}
        title={`Disconnect ${label}?`}
        description="Channels stop turning into conversations, alerts stop and teammates' own accounts are signed out. You can connect it again later."
        confirmLabel="Disconnect"
        onConfirm={disconnect}
      />
    </Section>
  );
}

export function IntegrationSettingsPage({ integration: id }: { integration: string }) {
  const workspace = useAccount().workspace.id;
  const canManage = usePermission(workspace, "integrations:manage");
  const canNotify = usePermission(workspace, "notification_destinations:manage");
  const { data: integration, isPending } = useQuery(integrationQuery(workspace, id));

  useProviderLanding(workspace);

  if (isPending) {
    return <PageSkeleton />;
  }

  if (!integration) {
    return (
      <SettingsPage title="Integration not found" description="It may have been disconnected.">
        <BackToIntegrations />
      </SettingsPage>
    );
  }

  const look = providerLook(integration.provider);

  return (
    <SettingsPage title={integration.name} description={look.detail}>
      <BackToIntegrations />
      <StatusSection integration={integration} />
      <FeaturesSection workspace={workspace} integration={integration} canManage={canManage} />
      <OwnAccountSection workspace={workspace} integration={integration} />
      {integration.provider === "slack"
        ? integration.channel_ids.map((channel) => (
            <RoomsSection key={channel} workspace={workspace} channel={channel} canManage={canManage} />
          ))
        : null}
      {integration.provider === "slack" ? (
        <SlackNotificationsSection workspace={workspace} integration={integration.id} canManage={canNotify} />
      ) : null}
      {canManage ? <DisconnectSection workspace={workspace} integration={integration} /> : null}
    </SettingsPage>
  );
}
