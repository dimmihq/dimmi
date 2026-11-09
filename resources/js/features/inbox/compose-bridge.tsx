/*
 * ComposeBridge lets a conversation's panes reach its reply box: the
 * composer registers itself and Operator's "Use in reply" puts a draft
 * into it.
 */
import { type ReactNode, useCallback, useMemo, useState } from "react";

import { type Bridge, ComposeBridgeContext, type Put } from "./compose-bridge-context";

export function ComposeBridge({ children }: { children: ReactNode }) {
  const [put, setPut] = useState<{ fn: Put } | null>(null);
  const register = useCallback((fn: Put | null) => setPut(fn ? { fn } : null), []);
  const value = useMemo<Bridge>(() => ({ put: put?.fn ?? null, register }), [put, register]);

  return <ComposeBridgeContext value={value}>{children}</ComposeBridgeContext>;
}
