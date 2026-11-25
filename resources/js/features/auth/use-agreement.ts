/*
 * The state of the box that accepts the legal terms, shared by the form
 * and the provider buttons of one page so neither creates an account
 * before it is ticked. On a server that publishes no terms there is
 * nothing to tick and every form goes on.
 */
import { useState } from "react";

import type { Terms } from "@/lib/api/auth";

/*
 * Agreement is the state of the box on one form: whether the server asks
 * for it, whether it is ticked, and whether the person tried to go on
 * without it.
 */
export interface Agreement {
  terms: Terms | null;
  accepted: boolean;
  missing: boolean;
  setAccepted: (accepted: boolean) => void;
  /** Reports whether the form may go on, and shows the reminder when it may not. */
  confirm: () => boolean;
}

export function useAgreement(terms: Terms | null | undefined): Agreement {
  const [accepted, setAccepted] = useState(false);
  const [missing, setMissing] = useState(false);

  return {
    terms: terms ?? null,
    accepted,
    missing: missing && !accepted,
    setAccepted,
    confirm: () => {
      const allowed = !terms || accepted;
      setMissing(!allowed);

      return allowed;
    },
  };
}
