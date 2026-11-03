/*
 * Says whether what the person sees is live. While the connection is up
 * nothing shows, only screen readers hear that updates are live; once it
 * has been down for a moment, a small pill says the app is reconnecting,
 * so nobody answers a customer from a screen that stopped updating
 * without knowing it. A blip shorter than that, such as the first
 * connect of a page, shows nothing.
 */
import { useEffect, useState } from "react";

import { StatusDot } from "@/components/base/badges/status-dot";
import { useRealtime } from "@/lib/realtime/context";
import { cx } from "@/utils/cx";

const labels = {
  connected: "Live updates on",
  connecting: "Reconnecting…",
  disconnected: "Offline. Updates paused",
} as const;

const graceMs = 2000;

export function ConnectionIndicator({ className }: { className?: string }) {
  const { client, state } = useRealtime();
  const [late, setLate] = useState(false);

  useEffect(() => {
    if (state === "connected") {
      setLate(false);
      return;
    }

    const timer = window.setTimeout(() => setLate(true), graceMs);
    return () => window.clearTimeout(timer);
  }, [state]);

  if (!client) {
    return null;
  }

  if (state === "connected" || !late) {
    return (
      <span role="status" className="sr-only" data-realtime={state}>
        {state === "connected" ? labels.connected : ""}
      </span>
    );
  }

  return (
    <span
      role="status"
      data-realtime={state}
      className={cx(
        "inline-flex items-center gap-1.5 rounded-full border border-border-button-default bg-background-primary-default px-2.5 py-1 text-caption-1-medium text-text-secondary shadow-sm",
        className,
      )}
    >
      <StatusDot color="yellow" />
      {labels[state]}
    </span>
  );
}
