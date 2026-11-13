/*
 * The signatures turned on or off on a page but not saved yet, held for
 * its save bar.
 */
import { useState } from "react";

import { type EmailSignature, updateSignature } from "@/lib/api/email-signatures";

/*
 * useSignatureToggles holds the signatures turned on or off but not saved
 * yet, by id. send writes each change; clear forgets them all, after a
 * save or on Discard. A removed signature's change is forgotten with it.
 */
export function useSignatureToggles(workspace: string) {
  const [pending, setPending] = useState<Record<string, boolean>>({});

  return {
    dirty: Object.keys(pending).length > 0,
    enabledOf: (signature: EmailSignature) => pending[signature.id] ?? signature.enabled,
    set: (signature: EmailSignature, on: boolean) =>
      setPending(({ [signature.id]: _, ...rest }) =>
        on === signature.enabled ? rest : { ...rest, [signature.id]: on },
      ),
    forget: (id: string) => setPending(({ [id]: _, ...rest }) => rest),
    send: () =>
      Promise.all(Object.entries(pending).map(([id, enabled]) => updateSignature(workspace, id, { enabled }))),
    clear: () => setPending({}),
  };
}

export type SignatureToggles = ReturnType<typeof useSignatureToggles>;
