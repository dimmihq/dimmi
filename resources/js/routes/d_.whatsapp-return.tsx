/*
 * Where Meta's signup popup returns: it hands the code Meta gave back to
 * the settings page that opened it, then closes. Opened on its own, it
 * just says the window can be closed.
 */
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { signupMessage } from "@/features/settings/whatsapp-connect";

export const Route = createFileRoute("/d_/whatsapp-return")({
  component: WhatsAppReturn,
});

function WhatsAppReturn() {
  const [handedBack, setHandedBack] = useState(false);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const opener = window.opener as Window | null;
    if (!opener) {
      return;
    }

    opener.postMessage(
      {
        type: signupMessage,
        state: params.get("state") ?? undefined,
        code: params.get("code") ?? undefined,
        error: params.get("error") ?? params.get("error_reason") ?? undefined,
      },
      window.location.origin,
    );
    setHandedBack(true);
    window.setTimeout(() => window.close(), 300);
  }, []);

  return (
    <main className="flex min-h-dvh items-center justify-center bg-background-primary-default p-6">
      <p className="text-body-regular text-text-secondary">
        {handedBack ? "Finishing in dimmi…" : "You can close this window."}
      </p>
    </main>
  );
}
