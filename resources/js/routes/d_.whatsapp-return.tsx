/*
 * Where Meta's sign-in popup returns: it hands the code Meta gave back
 * to the settings page that opened it, over a channel that page listens
 * on, since Meta's pages cut the popup off from its opener, then closes.
 * When no page answers, as when the browser opened Meta in place of the
 * settings page, it goes back to that page itself, which finishes the
 * sign-in. When the browser will not let it close, it says the window
 * can be closed.
 */
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { leaveForPage, signupChannel, signupMessage, signupTaken } from "@/features/settings/whatsapp-connect";

export const Route = createFileRoute("/d_/whatsapp-return")({
  component: WhatsAppReturn,
});

function WhatsAppReturn() {
  const [handedBack, setHandedBack] = useState(false);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const message = {
      type: signupMessage,
      state: params.get("state") ?? undefined,
      code: params.get("code") ?? undefined,
      error: params.get("error") ?? params.get("error_reason") ?? undefined,
    };
    if (!message.state) {
      return;
    }

    const channel = new BroadcastChannel(signupChannel);
    let taken = false;
    channel.onmessage = (event) => {
      if (event.data?.type === signupTaken && event.data.state === message.state) {
        taken = true;
        setHandedBack(true);
        window.close();
      }
    };
    channel.postMessage(message);
    (window.opener as Window | null)?.postMessage(message, window.location.origin);

    const alone = window.setTimeout(() => {
      channel.close();
      const page = !taken && message.code ? leaveForPage(message.state ?? "", message.code) : null;
      if (page) {
        window.location.replace(page);
      } else {
        setHandedBack(true);
      }
    }, 1500);

    return () => {
      window.clearTimeout(alone);
      channel.close();
    };
  }, []);

  return (
    <main className="flex min-h-dvh items-center justify-center bg-background-primary-default p-6">
      <p className="text-body-regular text-text-secondary">
        {handedBack
          ? "Connected to Meta. You can close this window and go back to dimmi."
          : "You can close this window."}
      </p>
    </main>
  );
}
