/*
 * The tools the workspace connects, one card per tool this install
 * offers: Slack now, GitHub and Linear once the server offers them. A
 * tool is installed once and every feature shares it. Connecting sends
 * the browser to the tool to approve and back here, where a toast says
 * how it went. A connected tool shows how it stands and opens its own
 * page; one missing permissions a feature needs offers to update them.
 * A tool this server has no app for says so instead of offering to
 * connect.
 */
import { RiArrowRightSLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import {
  type Integration,
  type IntegrationProvider,
  integrationProvidersQuery,
  integrationsQuery,
} from "@/lib/api/integrations";
import { usePermission } from "../inboxes";
import { healthColor, healthLabels, integrationHealth, providerLook } from "../integrations";
import { Empty, Item, Logo, RowsSkeleton, Section, SettingsPage } from "../layout";
import { useAccount } from "../use-account";
import { useProviderLanding } from "../use-provider-landing";
import { ConnectButton } from "./integration-parts";

/*
 * ProviderCard is one tool: its installation when it has one, or the
 * way to connect it.
 */
function ProviderCard({
  workspace,
  provider,
  installation,
  canManage,
}: {
  workspace: string;
  provider: IntegrationProvider;
  installation: Integration | undefined;
  canManage: boolean;
}) {
  const navigate = useNavigate();
  const look = providerLook(provider.name);

  if (!installation) {
    return (
      <Item
        mark={<Logo icon={look.icon} brand={look.brand} />}
        title={provider.label}
        detail={
          provider.installable ? look.detail : "This server has no app for it yet. Ask whoever runs dimmi to add one."
        }
      >
        {canManage && provider.installable ? (
          <ConnectButton workspace={workspace} provider={provider.name} label="Connect" />
        ) : null}
      </Item>
    );
  }

  const health = integrationHealth(installation);

  return (
    <Item
      mark={<Logo icon={look.icon} brand={look.brand} />}
      title={
        <span className="flex items-center gap-2">
          {provider.label}
          <Chip variant="caption" color={healthColor[health]}>
            {healthLabels[health]}
          </Chip>
        </span>
      }
      detail={installation.last_error ?? installation.name}
    >
      <Button
        variant="secondary"
        size="small"
        trailingIcon={RiArrowRightSLine}
        aria-label={`Manage ${provider.label}`}
        onClick={() =>
          navigate({ to: "/d/settings/integrations/$integration", params: { integration: installation.id } })
        }
      >
        Manage
      </Button>
    </Item>
  );
}

export function IntegrationsSettings() {
  const workspace = useAccount().workspace.id;
  const canManage = usePermission(workspace, "integrations:manage");
  const providers = useQuery(integrationProvidersQuery(workspace));
  const installations = useQuery(integrationsQuery(workspace));

  useProviderLanding(workspace);

  const loading = providers.isPending || installations.isPending;
  const list = providers.data ?? [];

  return (
    <SettingsPage
      title="Integrations"
      description="Connect the tools your team already uses. Each one is installed once and every feature shares it."
    >
      <Section title="Tools" description="Connecting one takes you to it to approve, then brings you back here.">
        {loading ? <RowsSkeleton rows={1} /> : null}
        {!loading && list.length === 0 ? <Empty>This server offers no integrations.</Empty> : null}
        {list.map((provider) => (
          <ProviderCard
            key={provider.name}
            workspace={workspace}
            provider={provider}
            installation={installations.data?.find((installation) => installation.provider === provider.name)}
            canManage={canManage}
          />
        ))}
      </Section>
    </SettingsPage>
  );
}
