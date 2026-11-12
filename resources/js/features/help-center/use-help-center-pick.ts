/*
 * Which of the workspace's help centers the dashboard is working on. The
 * pick is shared by the Help center page and its settings and kept per
 * workspace in this browser, so switching on one shows the same help
 * center on the other. A pick that no longer exists, such as one just
 * deleted, falls back to the default help center, which lists first.
 */
import { useQuery } from "@tanstack/react-query";
import { useSyncExternalStore } from "react";

import { useAccount } from "@/features/settings/use-account";
import { type HelpCenter, helpCentersQuery } from "@/lib/api/help-centers";

const listeners = new Set<() => void>();

function keyOf(workspace: string): string {
  return `dimmi:help-center:${workspace}`;
}

function read(workspace: string): string {
  try {
    return window.localStorage.getItem(keyOf(workspace)) ?? "";
  } catch {
    return "";
  }
}

/*
 * pickHelpCenter makes the help center the one the dashboard works on.
 */
export function pickHelpCenter(workspace: string, id: string) {
  try {
    window.localStorage.setItem(keyOf(workspace), id);
  } catch {
    // A browser without storage keeps the pick for this page only.
  }
  memory.set(workspace, id);
  for (const listener of listeners) {
    listener();
  }
}

const memory = new Map<string, string>();

function subscribe(listener: () => void) {
  listeners.add(listener);

  return () => listeners.delete(listener);
}

/*
 * chosen returns the picked help center, or the default one.
 */
export function chosen(centers: HelpCenter[], picked: string): HelpCenter | undefined {
  return centers.find((c) => c.id === picked) ?? centers.find((c) => c.is_default) ?? centers[0];
}

/*
 * useHelpCenterPick returns the workspace's help centers, the one picked
 * and a way to pick another.
 */
export function useHelpCenterPick() {
  const workspace = useAccount().workspace.id;
  const enabled = workspace !== "";
  const query = useQuery({ ...helpCentersQuery(workspace), enabled });
  const picked = useSyncExternalStore(subscribe, () => memory.get(workspace) ?? read(workspace));
  const centers = query.data ?? [];

  return {
    workspace,
    centers,
    center: chosen(centers, picked),
    isPending: query.isPending,
    pick: (id: string) => pickHelpCenter(workspace, id),
  };
}
