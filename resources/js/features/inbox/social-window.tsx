/*
 * Where a conversation stands on Messenger or Instagram, for the
 * composer. Meta lets anyone reply for 24 hours after the customer's last
 * message, then only a teammate for the rest of a week, then nobody
 * until the customer writes again. A chip counts down the time left, and
 * a line above the field says when only teammates may reply, or that
 * nobody may. It is read only for conversations on those two channels,
 * and again whenever a new message lands, since a customer's message
 * starts the time over.
 */
import { RiTimeLine } from "@remixicon/react";
import { useEffect, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import type { SocialConversation } from "@/lib/api/social";

import { left, socialNames as names } from "./social-state";

function useNow(): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const tick = window.setInterval(() => setNow(Date.now()), 60_000);
    return () => window.clearInterval(tick);
  }, []);

  return now;
}

/*
 * SocialChip counts down what the current phase leaves: the 24 hours
 * anyone may reply, then the days a teammate still may.
 */
export function SocialChip({ state }: { state: SocialConversation | undefined }) {
  const now = useNow();
  if (!state || state.phase === "closed") {
    return null;
  }

  const human = state.phase === "human";
  const remaining = left(human ? state.human_until : state.open_until, now);
  if (!remaining) {
    return null;
  }

  const label = human ? `Teammates only · ${remaining}` : remaining;

  return (
    <Chip
      variant="caption"
      color={human ? "orange" : "soft"}
      className="gap-1"
      title={
        human
          ? `${names[state.platform]} lets only teammates reply now, for 7 days after the customer's last message.`
          : `${names[state.platform]} lets anyone reply for 24 hours after the customer's last message.`
      }
    >
      <RiTimeLine aria-hidden className="size-3" />
      <span>{label}</span>
    </Chip>
  );
}

/*
 * SocialNotice says, above the field, when the AI agent may no longer
 * reply, or that nobody may until the customer writes again.
 */
export function SocialNotice({ customer, state }: { customer: string; state: SocialConversation | undefined }) {
  if (!state || state.phase === "open") {
    return null;
  }

  return (
    <p className="flex items-center gap-1.5 px-1.5 text-body-2-regular text-text-secondary">
      <RiTimeLine aria-hidden className="size-3.5 shrink-0 text-foreground-icon-tertiary" />
      <span>
        {state.phase === "human"
          ? `More than 24 hours passed since ${customer} last wrote. You can still reply; the AI agent cannot.`
          : `More than 7 days passed since ${customer} last wrote. ${names[state.platform]} lets you reply once they write again.`}
      </span>
    </p>
  );
}
