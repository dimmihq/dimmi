/*
 * Holds what the teammate has open for the whole signed-in app, so any
 * Operator panel can send it with a question. Pages set it with
 * useViewing.
 */
import { type ReactNode, useMemo, useState } from "react";

import type { OperatorViewing } from "@/lib/api/operator";

import { ViewingContext } from "./viewing";

export function ViewingProvider({ children }: { children: ReactNode }) {
  const [viewing, show] = useState<OperatorViewing | null>(null);
  const held = useMemo(() => ({ viewing, show }), [viewing]);

  return <ViewingContext.Provider value={held}>{children}</ViewingContext.Provider>;
}
