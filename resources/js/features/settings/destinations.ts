/*
 * How the settings talk about Slack destinations, in plain words: what
 * a channel can hear about, the digests it can get, and a line on how a
 * destination stands. A destination Slack refused says why, so the fix
 * is clear before it is turned back on.
 */
import type {
  DestinationDigest,
  DestinationHearing,
  NotificationDestination,
} from "@/lib/api/notification-destinations";

export const destinationTriggers: { id: string; label: string }[] = [
  { id: "new_conversation", label: "New conversations" },
  { id: "customer_replied", label: "Customer replies" },
  { id: "conversation_assigned", label: "Conversations assigned" },
  { id: "ai_handoff_requested", label: "Handed over by the agent" },
  { id: "snooze_expired", label: "Snoozes ending" },
];

export const destinationDigests: { id: DestinationDigest; label: string; detail: string }[] = [
  { id: "daily_standup", label: "Morning standup", detail: "What is open, waiting and due today." },
  { id: "daily_summary", label: "Daily summary", detail: "How yesterday went: volume, replies and wait times." },
  { id: "weekly_themes", label: "Weekly themes", detail: "What customers asked about most this week." },
];

/*
 * defaultHearing is what a new channel hears about: new conversations
 * and handovers, from every inbox, with previews.
 */
export const defaultHearing: DestinationHearing = {
  triggers: ["new_conversation", "ai_handoff_requested"],
  inbox_ids: null,
  include_ai_handled: false,
  show_previews: true,
  digests: [],
  digest_time: "09:00",
};

export function hearingOf(destination: NotificationDestination): DestinationHearing {
  return {
    triggers: destination.triggers,
    inbox_ids: destination.inbox_ids,
    include_ai_handled: destination.include_ai_handled,
    show_previews: destination.show_previews,
    digests: destination.digests,
    digest_time: destination.digest_time,
  };
}

/*
 * stopped says why a destination is not posting, or null while it
 * posts.
 */
export function stopped(destination: NotificationDestination): string | null {
  if (destination.status === "error") {
    return `Stopped: ${destination.last_error ?? "Slack refused the last post."}`;
  }

  if (destination.status === "disabled") {
    return "Turned off.";
  }

  return null;
}

/*
 * destinationLine is the line under a channel destination: why it
 * stopped, or what it hears about.
 */
export function destinationLine(destination: NotificationDestination): string {
  const reason = stopped(destination);
  if (reason) {
    return reason;
  }

  const heard = [
    ...destinationTriggers.filter((t) => destination.triggers.includes(t.id)).map((t) => t.label),
    ...destinationDigests.filter((d) => destination.digests.includes(d.id)).map((d) => d.label),
  ];

  return heard.length > 0 ? heard.join(" · ") : "Hears about nothing yet.";
}

/*
 * toggled adds the value to the list or takes it out.
 */
export function toggled<T>(list: T[], value: T, on: boolean): T[] {
  const without = list.filter((item) => item !== value);

  return on ? [...without, value] : without;
}
