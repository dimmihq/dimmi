/*
 * What the teammate has open, for Operator. A page says what it shows
 * with useViewing; every question to Operator, from whichever panel it
 * is asked, then carries it, and Operator is given that record in full.
 * This is the one way a page hands Operator its context: to give
 * Operator a new page, call useViewing there and add the page's kind to
 * the engine's page registry (app/ai/operator/pages.go).
 */
import { createContext, useContext, useEffect, useRef } from "react";

import type { OperatorViewing } from "@/lib/api/operator";

export interface HeldViewing {
  viewing: OperatorViewing | null;
  show: (viewing: OperatorViewing | null) => void;
}

export const ViewingContext = createContext<HeldViewing | null>(null);

/*
 * useViewing says what this page shows while it is on screen: the kind
 * of page, the id of what it shows, and the suggestion shown on it.
 */
export function useViewing(page: OperatorViewing["page"], id: string | null | undefined, suggestion?: string | null) {
  const held = useContext(ViewingContext);
  const show = held?.show;
  useEffect(() => {
    if (!show || !id) {
      return;
    }
    show({ page, id, suggestion_id: suggestion ?? undefined });
    return () => show(null);
  }, [show, page, id, suggestion]);
}

/*
 * useCurrentViewing reads what is open, always the latest, for the
 * moment a question is sent.
 */
export function useCurrentViewing(): () => OperatorViewing | undefined {
  const viewing = useContext(ViewingContext)?.viewing ?? null;
  const latest = useRef(viewing);
  latest.current = viewing;

  return () => latest.current ?? undefined;
}
