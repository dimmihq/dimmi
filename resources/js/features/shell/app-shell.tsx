/*
 * The frame of the signed-in app, built on BoardUI's floating sidebar:
 * the workspace switcher at the top, the pages in sections, the
 * getting-started checklist, and the person's own menu at the foot, with
 * the current page in floating panels beside it. In settings, the settings pages
 * take the sidebar's place. Below the large breakpoint the sidebar moves
 * into a drawer opened from a slim top bar. While a conversation, a
 * workflow or a procedure is open, the sidebar folds to its rail so the
 * page's panels have room; unfolding it by hand holds until the person
 * leaves that page. The shell also opens the app's one realtime
 * connection, for the workspace on screen, and says when it drops.
 */
import {
  RiAddLine,
  RiLogoutBoxRLine,
  RiMenuLine,
  RiSettings4Line,
  RiShieldUserLine,
  RiUserLine,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { useRouter, useRouterState } from "@tanstack/react-router";
import { type ReactNode, useCallback, useState } from "react";

import {
  type DashboardAccount,
  DashboardSidebar,
  type DashboardSwitcher,
} from "@/components/application/dashboard/dashboard-sidebar";
import {
  MenuActionRow,
  MenuDivider,
  MenuHeader,
  MenuRowList,
} from "@/components/application/dashboard/dashboard-user-menu";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import { Toaster } from "@/components/base/toast/toast";
import { ActionProvider } from "@/features/inbox/actions/action-provider";
import { OpenNotificationsContext } from "@/features/notifications/notification-center-context";
import { NotificationsPanel } from "@/features/notifications/notifications-panel";
import { useNotificationFocus } from "@/features/notifications/use-notification-focus";
import { ConnectionIndicator } from "@/features/realtime/connection-indicator";
import { type Me, meQuery, signOut } from "@/lib/api/auth";
import { countsQuery } from "@/lib/api/inbox";
import { notificationCountsQuery } from "@/lib/api/notifications";
import { callerQuery } from "@/lib/api/workspace";
import { RealtimeProvider } from "@/lib/realtime/provider";

import { useAvailability } from "./availability";
import { GetStarted } from "./get-started";
import { appSections, selectedKey, settingsLink, settingsPages, settingsSections } from "./navigation";
import { useCurrentWorkspace } from "./workspace";

const paneledPath =
  /^\/d\/((inbox|broadcasts|customers|help-center|changelog|feedback)(\/.*)?|(series|workflows|procedures)\/.+)$/;

/*
 * useRailCollapse folds the sidebar on pages that bring a second side
 * panel of their own, such as the inbox, broadcasts, customers and an
 * open series or workflow, so the page keeps room for its own columns.
 * A choice made by hand wins until the page switches between one of
 * those and anything else.
 */
function useRailCollapse(path: string) {
  const reading = paneledPath.test(path);
  const [choice, setChoice] = useState<{ reading: boolean; collapsed: boolean } | null>(null);
  const collapsed = choice?.reading === reading ? choice.collapsed : reading;

  return [collapsed, (next: boolean) => setChoice({ reading, collapsed: next })] as const;
}

function initialsOf(name: string): string {
  return name.trim().charAt(0).toUpperCase() || "?";
}

const roleNames: Record<string, string> = { owner: "Owner", admin: "Admin", member: "Member" };

function roleOf(role: string): string {
  return roleNames[role] ?? role;
}

/*
 * useSwitcher is the workspace menu at the top of the sidebar: the
 * workspace on screen, the others to switch to, and starting a new one.
 * Switching goes back through the app's door, so a workspace still being
 * set up opens its setup instead; a new workspace starts its own setup.
 */
function useSwitcher(me: Me): DashboardSwitcher {
  const router = useRouter();
  const { current, choose } = useCurrentWorkspace(me.workspaces);

  return {
    label: current?.name ?? "No workspace",
    initials: initialsOf(current?.name ?? "?"),
    menuLabel: "Workspaces",
    menu: (close) => (
      <>
        {current && me.workspaces.length < 2 ? (
          <MenuHeader
            initials={initialsOf(current.name)}
            color="neutral"
            name={current.name}
            detail={roleOf(current.role)}
          />
        ) : null}
        {current ? null : (
          <p className="px-2 py-1.5 text-body-2-regular text-text-secondary">You are not in a workspace yet.</p>
        )}
        {me.workspaces.length > 1 ? (
          <MenuRowList
            title="Workspaces"
            rows={me.workspaces.map((workspace) => ({
              key: workspace.id,
              initials: initialsOf(workspace.name),
              color: "neutral",
              name: workspace.name,
              detail: roleOf(workspace.role),
              isSelected: workspace.id === current?.id,
            }))}
            onSelect={(id) => {
              close();
              if (id === current?.id) {
                return;
              }
              choose(id);
              void router.navigate({ to: "/d" });
            }}
          />
        ) : null}
        <MenuDivider />
        <div className="flex flex-col gap-0.5">
          <MenuActionRow
            icon={RiAddLine}
            label="New workspace"
            onSelect={() => {
              close();
              void router.navigate({ to: "/onboarding", search: { new: true } });
            }}
          />
          <MenuActionRow
            icon={RiSettings4Line}
            label="Workspace settings"
            onSelect={() => {
              close();
              router.history.push(settingsPages.workspace.href);
            }}
          />
        </div>
      </>
    ),
  };
}

function useAccount(me: Me, workspace: string | undefined): DashboardAccount {
  const availability = useAvailability(workspace);
  const router = useRouter();
  const queryClient = useQueryClient();
  const leave = useMutation({
    mutationFn: signOut,
    onSuccess: () => {
      queryClient.setQueryData(meQuery.queryKey, null);
      router.history.push("/login");
    },
  });

  return {
    name: me.user.name,
    detail: me.user.email,
    initials: initialsOf(me.user.name),
    product: "dimmi",
    version: me.instance?.version,
    status: availability.status,
    groups: [
      ...(availability.group ? [availability.group] : []),
      {
        id: "account",
        items: [
          { icon: RiUserLine, label: "Profile", onSelect: () => router.history.push(settingsPages.profile.href) },
          {
            icon: RiShieldUserLine,
            label: "Security",
            onSelect: () => router.history.push(settingsPages.security.href),
          },
        ],
      },
      { id: "personal", items: [] },
      {
        id: "session",
        items: [{ icon: RiLogoutBoxRLine, label: "Sign out", onSelect: () => leave.mutate() }],
      },
    ],
  };
}

export function AppShell({ children }: { children: ReactNode }) {
  const { data: me } = useSuspenseQuery(meQuery);
  const path = useRouterState({ select: (state) => state.location.pathname });
  const [drawerOpen, setDrawerOpen] = useState(false);

  if (!me) {
    return null;
  }

  return (
    <SignedInShell me={me} path={path} drawerOpen={drawerOpen} setDrawerOpen={setDrawerOpen}>
      {children}
    </SignedInShell>
  );
}

function SignedInShell({
  me,
  path,
  drawerOpen,
  setDrawerOpen,
  children,
}: {
  me: Me;
  path: string;
  drawerOpen: boolean;
  setDrawerOpen: (open: boolean) => void;
  children: ReactNode;
}) {
  const switcher = useSwitcher(me);
  const { current: onScreen } = useCurrentWorkspace(me.workspaces);
  const account = useAccount(me, onScreen?.id);
  const { current: workspace } = useCurrentWorkspace(me.workspaces);
  const [collapsed, setCollapsed] = useRailCollapse(path);
  const inSettings = path === settingsLink.href || path.startsWith(`${settingsLink.href}/`);
  const { data: counts } = useQuery({ ...countsQuery(workspace?.id ?? ""), enabled: Boolean(workspace) });
  const { data: bell } = useQuery({
    ...notificationCountsQuery(workspace?.id ?? ""),
    enabled: Boolean(workspace),
  });
  const { data: caller } = useQuery({ ...callerQuery(workspace?.id ?? ""), enabled: Boolean(workspace) });
  useNotificationFocus(workspace?.id ?? null, path);
  const [bellOpen, setBellOpen] = useState(false);
  const openBell = useCallback(() => setBellOpen(true), []);
  const sections = inSettings
    ? settingsSections()
    : appSections(
        counts?.views.mine.open,
        bell?.unseen_count,
        {
          isOpen: bellOpen,
          onOpenChange: setBellOpen,
          content: (close) => <NotificationsPanel onClose={close} />,
        },
        caller?.member?.permissions,
      );
  const secondary = inSettings ? [] : [settingsLink];
  const selected = selectedKey(sections, secondary, path);
  const sidebar = (mobile: boolean) => (
    <DashboardSidebar
      mobile={mobile}
      onClose={() => setDrawerOpen(false)}
      sections={sections}
      secondary={secondary}
      selected={selected}
      switcher={switcher}
      account={account}
      footer={inSettings || !workspace ? null : <GetStarted workspace={workspace.id} />}
      showThemeToggle={false}
      density="compact"
      collapsed={mobile ? false : collapsed}
      onCollapsedChange={mobile ? undefined : setCollapsed}
      className={mobile ? "h-full w-full" : undefined}
    />
  );

  return (
    <RealtimeProvider workspace={workspace?.id ?? null}>
      <div className="flex h-dvh flex-col gap-3 bg-background-full p-3 lg:flex-row lg:gap-4">
        <div className="hidden h-full lg:flex">{sidebar(false)}</div>

        <div className="flex items-center gap-2 px-1 lg:hidden">
          <IconButton size="small" icon={RiMenuLine} aria-label="Open navigation" onClick={() => setDrawerOpen(true)} />
          <span className="text-body-semibold text-text-primary">{switcher.label}</span>
        </div>
        <Drawer
          isOpen={drawerOpen}
          onOpenChange={setDrawerOpen}
          placement="start"
          size="nav"
          aria-label="Navigation"
          className="border-0 bg-transparent p-0 shadow-none"
        >
          {sidebar(true)}
        </Drawer>

        <main className="flex min-h-0 min-w-0 flex-1">
          <OpenNotificationsContext.Provider value={openBell}>
            <ActionProvider>{children}</ActionProvider>
          </OpenNotificationsContext.Provider>
        </main>
      </div>
      <ConnectionIndicator className="fixed end-4 bottom-4 z-50" />
      <Toaster className="[html[data-save-bar]_&]:bottom-24" />
    </RealtimeProvider>
  );
}
