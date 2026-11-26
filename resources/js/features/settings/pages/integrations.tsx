/*
 * The tools the workspace connects, grouped by what they are for:
 * messaging, issue trackers, incident tools and
 * knowledge. A tool is installed once and every feature shares it. Each
 * tool is one row: a filled check when it is connected, an empty one
 * when it is not, a warning when it needs attention. A connected row
 * opens the tool's page; an unconnected one connects it, sending the
 * browser to the tool to approve or asking for its API key. A tool this
 * server has no app for says so and opens nothing.
 */
import {
  RiArrowRightSLine,
  RiCheckboxBlankCircleLine,
  RiCheckboxCircleFill,
  RiErrorWarningFill,
  RiWhatsappLine,
} from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Spinner } from "@/components/base/spinner/spinner";
import { toast } from "@/components/base/toast/toast";
import { KeyConnectDialog } from "@/features/tickets/issues/key-connect";
import {
  type Integration,
  type IntegrationProvider,
  integrationProvidersQuery,
  integrationsQuery,
  startInstallation,
} from "@/lib/api/integrations";
import { type SocialPlatform, socialChannelsQuery } from "@/lib/api/social";
import { whatsappChannelsQuery } from "@/lib/api/whatsapp";
import { cx } from "@/utils/cx";
import { usePermission } from "../inboxes";
import { healthLabels, integrationHealth, providerLook } from "../integrations";
import { Empty, Logo, RowsSkeleton, Section, SettingsPage } from "../layout";
import { socialLook } from "../social-look";
import { useAccount } from "../use-account";
import { useProviderLanding } from "../use-provider-landing";
import { whatsappGreen } from "../whatsapp-look";

/*
 * groups list the sections in order and the tools each holds; a tool
 * none names falls under Other.
 */
const messaging = "Messaging";

const groups: { title: string; description: string; providers: string[] }[] = [
  { title: messaging, description: "Answer customers and alert your team where they talk.", providers: ["slack"] },
  {
    title: "Issue trackers",
    description: "Link tickets to engineering issues; their status moves the ticket.",
    providers: ["linear", "jira", "github"],
  },
  {
    title: "Incidents",
    description: "Link tickets to incidents, or declare one from a ticket.",
    providers: ["incident_io", "rootly", "pagerduty"],
  },
  { title: "Knowledge", description: "Answer from the docs your team already keeps.", providers: ["notion"] },
];

/*
 * sections sorts the server's tools into the groups, keeping only
 * groups that hold one.
 */
function sections(providers: IntegrationProvider[]) {
  const named = new Set(groups.flatMap((g) => g.providers));
  const out = groups
    .map((g) => ({
      ...g,
      list: g.providers.flatMap((name) => providers.filter((p) => p.name === name)),
    }))
    .filter((g) => g.list.length > 0 || g.title === messaging);
  const other = providers.filter((p) => !named.has(p.name));
  if (other.length > 0) {
    out.push({ title: "Other", description: "", providers: [], list: other });
  }

  return out;
}

/*
 * State is the mark at a row's end: filled when connected and working,
 * a warning when it needs attention, empty when not connected.
 */
function State({ installation }: { installation: Integration | undefined }) {
  if (!installation) {
    return <RiCheckboxBlankCircleLine aria-label="Not connected" className="size-4 text-foreground-icon-quaternary" />;
  }
  const health = integrationHealth(installation);
  if (health !== "connected") {
    return <RiErrorWarningFill aria-label={healthLabels[health]} className="size-4 text-status-yellow-text" />;
  }

  return <RiCheckboxCircleFill aria-label="Connected" className="size-4 text-status-lime-text" />;
}

/*
 * ProviderRow is one tool. The whole row opens the tool's page, or
 * connects it when it has no installation yet.
 */
function ProviderRow({
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
  const [asking, setAsking] = useState(false);
  const start = useMutation({
    mutationFn: () => startInstallation(workspace, provider.name, { return_to: window.location.pathname }),
    onSuccess: ({ authorize_url }) => window.location.assign(authorize_url),
    onError: (error) => toast.error(error.message),
  });

  const connectable = !installation && canManage && provider.installable;
  const opens = Boolean(installation) || connectable;
  const detail = installation
    ? (installation.last_error ?? installation.name)
    : provider.installable
      ? look.detail
      : "This server has no app for it yet. Ask whoever runs dimmi to add one.";

  const open = () => {
    if (installation) {
      void navigate({ to: "/d/settings/integrations/$integration", params: { integration: installation.id } });
    } else if (provider.connect === "api_key") {
      setAsking(true);
    } else {
      start.mutate();
    }
  };

  return (
    <>
      <button
        type="button"
        disabled={!opens || start.isPending}
        onClick={open}
        aria-label={installation ? `Open ${provider.label}` : `Connect ${provider.label}`}
        className={cx(
          "-ms-1.5 flex min-h-[52px] w-[calc(100%+0.375rem)] items-center gap-3 rounded-lg border-b border-separator-border py-2.5 ps-1.5 pe-2.5 text-start outline-none transition-colors duration-150 last:border-b-0 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
          opens ? "cursor-pointer hover:bg-background-secondary-hover" : "cursor-default",
        )}
      >
        <span className="flex shrink-0 items-center">
          <Logo icon={look.icon} brand={look.brand} />
        </span>
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="truncate text-body-regular text-text-primary">{provider.label}</span>
          <span className="truncate text-body-2-regular text-text-secondary">{detail}</span>
        </span>
        <span className="flex shrink-0 items-center gap-1.5">
          {start.isPending ? <Spinner size="sm" /> : <State installation={installation} />}
          <RiArrowRightSLine
            aria-hidden
            className={cx(
              "size-5 rtl:-scale-x-100",
              opens ? "text-foreground-icon-tertiary" : "text-foreground-icon-quaternary opacity-0",
            )}
          />
        </span>
      </button>
      {provider.connect === "api_key" ? (
        <KeyConnectDialog workspace={workspace} provider={provider.name} isOpen={asking} onOpenChange={setAsking} />
      ) : null}
    </>
  );
}

/*
 * WhatsAppRow is WhatsApp among the tools: its numbers are channels of
 * their own rather than one installation, so the row opens the page that
 * lists them and connects more.
 */
function WhatsAppRow({ workspace }: { workspace: string }) {
  const navigate = useNavigate();
  const { data: numbers } = useQuery(whatsappChannelsQuery(workspace));
  const connected = (numbers?.length ?? 0) > 0;
  const detail = connected
    ? numbers?.map((n) => n.display_phone_number).join(", ")
    : "Answer customers who write to your business number.";

  return (
    <button
      type="button"
      onClick={() => navigate({ to: "/d/settings/integrations/whatsapp" })}
      aria-label="Open WhatsApp"
      className="-ms-1.5 flex min-h-[52px] w-[calc(100%+0.375rem)] cursor-pointer items-center gap-3 rounded-lg border-b border-separator-border py-2.5 ps-1.5 pe-2.5 text-start outline-none transition-colors duration-150 last:border-b-0 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <span className="flex shrink-0 items-center">
        <Logo icon={RiWhatsappLine} brand={whatsappGreen} />
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-regular text-text-primary">WhatsApp</span>
        <span className="truncate text-body-2-regular text-text-secondary">{detail}</span>
      </span>
      <span className="flex shrink-0 items-center gap-1.5">
        {connected ? (
          <RiCheckboxCircleFill aria-label="Connected" className="size-4 text-status-lime-text" />
        ) : (
          <RiCheckboxBlankCircleLine aria-label="Not connected" className="size-4 text-foreground-icon-quaternary" />
        )}
        <RiArrowRightSLine aria-hidden className="size-5 text-foreground-icon-tertiary rtl:-scale-x-100" />
      </span>
    </button>
  );
}

/*
 * SocialRow is Facebook Messenger or Instagram among the tools: like
 * WhatsApp, its accounts are channels of their own, so the row opens the
 * page that lists them and connects more.
 */
function SocialRow({ workspace, platform }: { workspace: string; platform: SocialPlatform }) {
  const navigate = useNavigate();
  const look = socialLook[platform];
  const { data: accounts } = useQuery(socialChannelsQuery(workspace, platform));
  const connected = (accounts?.length ?? 0) > 0;
  const detail = connected ? accounts?.map((a) => a.name).join(", ") : look.pitch;

  return (
    <button
      type="button"
      onClick={() =>
        navigate({
          to: platform === "facebook" ? "/d/settings/integrations/facebook" : "/d/settings/integrations/instagram",
        })
      }
      aria-label={`Open ${look.name}`}
      className="-ms-1.5 flex min-h-[52px] w-[calc(100%+0.375rem)] cursor-pointer items-center gap-3 rounded-lg border-b border-separator-border py-2.5 ps-1.5 pe-2.5 text-start outline-none transition-colors duration-150 last:border-b-0 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <span className="flex shrink-0 items-center">
        <Logo icon={look.icon} brand={look.brand} />
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-regular text-text-primary">{look.name}</span>
        <span className="truncate text-body-2-regular text-text-secondary">{detail}</span>
      </span>
      <span className="flex shrink-0 items-center gap-1.5">
        {connected ? (
          <RiCheckboxCircleFill aria-label="Connected" className="size-4 text-status-lime-text" />
        ) : (
          <RiCheckboxBlankCircleLine aria-label="Not connected" className="size-4 text-foreground-icon-quaternary" />
        )}
        <RiArrowRightSLine aria-hidden className="size-5 text-foreground-icon-tertiary rtl:-scale-x-100" />
      </span>
    </button>
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
      {loading ? (
        <Section title="Tools">
          <RowsSkeleton rows={3} />
        </Section>
      ) : null}
      {!loading && list.length === 0 ? (
        <Section title="Tools">
          <Empty>This server offers no integrations.</Empty>
        </Section>
      ) : null}
      {sections(list).map((group) => (
        <Section key={group.title} title={group.title} description={group.description || undefined}>
          {group.list.map((provider) => (
            <ProviderRow
              key={provider.name}
              workspace={workspace}
              provider={provider}
              installation={installations.data?.find((installation) => installation.provider === provider.name)}
              canManage={canManage}
            />
          ))}
          {group.title === messaging ? (
            <>
              <WhatsAppRow workspace={workspace} />
              <SocialRow workspace={workspace} platform="facebook" />
              <SocialRow workspace={workspace} platform="instagram" />
            </>
          ) : null}
        </Section>
      ))}
    </SettingsPage>
  );
}
