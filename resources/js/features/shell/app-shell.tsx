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
import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { useRouter, useRouterState } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import {
  type DashboardAccount,
  DashboardSidebar,
  type DashboardSwitcher,
} from "@/components/application/dashboard/dashboard-sidebar";
import { MenuDivider, MenuRowList } from "@/components/application/dashboard/dashboard-user-menu";
import { Button } from "@/components/base/buttons/button";
import { Drawer } from "@/components/base/drawer/drawer";
import { ActionProvider } from "@/features/inbox/actions/action-provider";
import { useSidebarViews } from "@/features/inbox/views/use-sidebar-views";
import { ConnectionIndicator } from "@/features/realtime/connection-indicator";
import { type Me, meQuery, signOut } from "@/lib/api/auth";
import { RealtimeProvider } from "@/lib/realtime/provider";

import { GetStarted } from "./get-started";
import { appSections, selectedKey, settingsLink, settingsPages, settingsSections } from "./navigation";
import { useCurrentWorkspace } from "./workspace";

const focusedPath = /^\/d\/(inbox\/[^/]+|workflows|procedures)\/[^/]+$/;

/*
 * useRailCollapse folds the sidebar while a conversation, workflow or
 * procedure is open. A choice made by hand wins until the page switches
 * between one of those and anything else.
 */
function useRailCollapse(path: string) {
  const reading = focusedPath.test(path);
  const [choice, setChoice] = useState<{ reading: boolean; collapsed: boolean } | null>(null);
  const collapsed = choice?.reading === reading ? choice.collapsed : reading;

  return [collapsed, (next: boolean) => setChoice({ reading, collapsed: next })] as const;
}

function initialsOf(name: string): string {
  return name.trim().charAt(0).toUpperCase() || "?";
}

function useSwitcher(me: Me): DashboardSwitcher {
  const router = useRouter();
  const { current, choose } = useCurrentWorkspace(me.workspaces);

  return {
    label: current?.name ?? "No workspace",
    initials: initialsOf(current?.name ?? "?"),
    menuLabel: "Workspaces",
    menu: (close) => (
      <>
        {me.workspaces.length > 0 ? (
          <MenuRowList
            title="Workspaces"
            rows={me.workspaces.map((workspace) => ({
              key: workspace.id,
              initials: initialsOf(workspace.name),
              color: "neutral",
              name: workspace.name,
              isSelected: workspace.id === current?.id,
            }))}
            onSelect={(id) => {
              choose(id);
              close();
            }}
          />
        ) : (
          <p className="px-2 pt-1 text-body-regular text-text-secondary">You are not in a workspace yet.</p>
        )}
        <MenuDivider />
        <div className="flex w-full items-center gap-3 px-2 pb-2">
          <Button variant="secondary" size="small" leadingIcon={RiAddLine} className="flex-1" disabled>
            New workspace
          </Button>
          <Button
            variant="secondary"
            size="small"
            leadingIcon={RiSettings4Line}
            className="flex-1"
            onClick={() => {
              close();
              router.history.push(settingsPages.workspace.href);
            }}
          >
            Settings
          </Button>
        </div>
      </>
    ),
  };
}

function useAccount(me: Me): DashboardAccount {
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
    product: "Dimmi",
    version: me.instance?.version,
    groups: [
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
  const account = useAccount(me);
  const { current: workspace } = useCurrentWorkspace(me.workspaces);
  const [collapsed, setCollapsed] = useRailCollapse(path);
  const inSettings = path === settingsLink.href || path.startsWith(`${settingsLink.href}/`);
  const views = useSidebarViews(workspace?.id ?? "");
  const sections = inSettings
    ? settingsSections(me.instance?.edition === "cloud")
    : appSections(views.sections, views.canDelete);
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
      footer={inSettings ? null : <GetStarted />}
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
          <Button
            variant="ghost"
            size="small"
            iconOnly
            leadingIcon={RiMenuLine}
            aria-label="Open navigation"
            onClick={() => setDrawerOpen(true)}
          />
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
          <ActionProvider>{children}</ActionProvider>
        </main>
      </div>
      <ConnectionIndicator className="fixed end-4 bottom-4 z-50" />
    </RealtimeProvider>
  );
}
