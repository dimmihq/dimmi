/*
 * Which of the person's workspaces the app shows. The choice is the
 * browser's, kept in local storage, so each browser reopens where the
 * person left it; a workspace they no longer belong to falls back to the
 * one they used most recently, which the server lists first.
 */
import { useCallback, useState } from "react";

import type { WorkspaceSummary } from "@/lib/api/auth";

const key = "dimmi:workspace";

function remembered(): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

/*
 * rememberWorkspace makes the workspace the one this browser opens, such
 * as one the person just joined. Blocked storage keeps the choice for
 * the current page only, and the answer says whether it was kept.
 */
export function rememberWorkspace(id: string): boolean {
  try {
    window.localStorage.setItem(key, id);

    return true;
  } catch {
    return false;
  }
}

/*
 * currentWorkspace is the workspace this browser opens, as routes need to
 * know before they render: the one it chose last, or the most recent.
 */
export function currentWorkspace(workspaces: WorkspaceSummary[]): WorkspaceSummary | null {
  const chosen = remembered();

  return workspaces.find((workspace) => workspace.id === chosen) ?? workspaces[0] ?? null;
}

export function useCurrentWorkspace(workspaces: WorkspaceSummary[]) {
  const [chosen, setChosen] = useState(remembered);
  const current = workspaces.find((workspace) => workspace.id === chosen) ?? workspaces[0] ?? null;

  const choose = useCallback((id: string) => {
    setChosen(id);
    rememberWorkspace(id);
  }, []);

  return { current, choose };
}
