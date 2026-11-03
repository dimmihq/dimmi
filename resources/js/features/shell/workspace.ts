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

export function useCurrentWorkspace(workspaces: WorkspaceSummary[]) {
  const [chosen, setChosen] = useState(remembered);
  const current = workspaces.find((workspace) => workspace.id === chosen) ?? workspaces[0] ?? null;

  const choose = useCallback((id: string) => {
    setChosen(id);
    try {
      window.localStorage.setItem(key, id);
    } catch {
      // Blocked storage keeps the choice for this page only.
    }
  }, []);

  return { current, choose };
}
