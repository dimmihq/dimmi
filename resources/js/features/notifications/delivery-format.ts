/*
 * How a line of the delivery log reads: whether it was sent, failed or
 * held back, and why in plain words.
 */
import type { DeliveryStatus, NotificationDelivery } from "@/lib/api/notification-deliveries";

export const deliveryReasons: Record<string, string> = {
  preference_off: "turned off in your settings",
  no_route: "no verified address to reach you",
  quiet_hours: "held during quiet hours",
  paused: "notifications were paused",
  suppressed: "your address bounced or complained",
  ai_handled: "the agent was answering",
  already_seen: "you had already seen it",
  no_longer_needed: "it no longer needed you",
  not_visible: "you can no longer see the conversation",
  cooldown: "a similar one went out moments before",
  rejected: "the provider refused it",
  error: "it could not be sent",
};

export const deliveryStatusLabels: Record<DeliveryStatus, string> = {
  sent: "Sent",
  failed: "Failed",
  skipped: "Skipped",
};

/*
 * deliveryOutcome says how a delivery ended and why, such as "Skipped:
 * held during quiet hours". An unknown reason reads as its words.
 */
export function deliveryOutcome(d: Pick<NotificationDelivery, "status" | "reason">): string {
  const status = deliveryStatusLabels[d.status];
  if (!d.reason) {
    return status;
  }
  return `${status}: ${deliveryReasons[d.reason] ?? d.reason.replaceAll("_", " ")}`;
}
