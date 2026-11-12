/*
 * What the webhook pages say about an endpoint and its deliveries, and
 * how the event picker reads and writes a subscription. A subscription
 * is *, area wildcards such as conversation.* or exact types; the
 * picker shows it as the exact types it covers, and saves exact types,
 * or * when every event is picked. Named-only types are never covered
 * by a wildcard.
 */
import type { ChipColor } from "@/components/base/badges/chip";
import type {
  WebhookDelivery,
  WebhookDeliveryStatus,
  WebhookEndpoint,
  WebhookEventType,
  WebhookStats,
} from "@/lib/api/webhooks";

export const everyEvent = "*";

/*
 * EventGroup is one heading of the picker with its types, in catalog
 * order.
 */
export interface EventGroup {
  name: string;
  types: WebhookEventType[];
}

/*
 * groupEventTypes keeps the catalog's order and gathers its types under
 * their group.
 */
export function groupEventTypes(catalog: WebhookEventType[]): EventGroup[] {
  const groups: EventGroup[] = [];
  for (const type of catalog) {
    const group = groups.find((g) => g.name === type.group);
    if (group) {
      group.types.push(type);
    } else {
      groups.push({ name: type.group, types: [type] });
    }
  }

  return groups;
}

function areaOf(type: string): string {
  return type.split(".")[0] ?? type;
}

/*
 * coveredTypes returns the exact types a subscription receives.
 */
export function coveredTypes(subscription: string[], catalog: WebhookEventType[]): string[] {
  return catalog
    .filter((t) => {
      if (subscription.includes(t.type)) {
        return true;
      }
      if (t.named_only) {
        return false;
      }

      return subscription.includes(everyEvent) || subscription.includes(`${areaOf(t.type)}.*`);
    })
    .map((t) => t.type);
}

/*
 * subscriptionFor is what the picker saves: * when every type a
 * wildcard covers is picked and nothing named-only, the exact types
 * otherwise, since the server keeps * alone.
 */
export function subscriptionFor(picked: string[], catalog: WebhookEventType[]): string[] {
  const wildcardable = catalog.filter((t) => !t.named_only);
  const all = wildcardable.length > 0 && wildcardable.every((t) => picked.includes(t.type));
  const named = catalog.filter((t) => t.named_only && picked.includes(t.type)).map((t) => t.type);

  if (all && named.length === 0) {
    return [everyEvent];
  }

  return catalog.filter((t) => picked.includes(t.type)).map((t) => t.type);
}

/*
 * subscriptionLine sums up what an endpoint subscribes to.
 */
export function subscriptionLine(types: string[]): string {
  if (types.includes(everyEvent)) {
    return "Every event";
  }
  if (types.length <= 2) {
    return types.join(", ");
  }

  return `${types.slice(0, 2).join(", ")} and ${types.length - 2} more`;
}

/*
 * endpointState is the chip of an endpoint: off, failing or healthy.
 */
export function endpointState(e: WebhookEndpoint): { label: string; color: ChipColor } {
  if (!e.enabled) {
    return { label: e.disabled_reason === "auto_failure" ? "Turned off" : "Off", color: "gray" };
  }
  if (e.health === "failing") {
    return { label: "Failing", color: "rose" };
  }

  return { label: "Healthy", color: "lime" };
}

const statusNames: Record<WebhookDeliveryStatus, { label: string; color: ChipColor }> = {
  pending: { label: "Pending", color: "neutral" },
  retrying: { label: "Retrying", color: "yellow" },
  succeeded: { label: "Delivered", color: "lime" },
  failed: { label: "Failed", color: "rose" },
  canceled: { label: "Canceled", color: "gray" },
};

/*
 * deliveryState is the chip of a delivery.
 */
export function deliveryState(d: Pick<WebhookDelivery, "status">): { label: string; color: ChipColor } {
  return statusNames[d.status] ?? { label: d.status, color: "neutral" };
}

/*
 * answerLine says how a delivery was last answered: its HTTP status, or
 * the error that kept it from one.
 */
export function answerLine(d: Pick<WebhookDelivery, "last_response_status" | "last_error_code">): string {
  if (d.last_response_status !== null) {
    return `HTTP ${d.last_response_status}`;
  }
  if (d.last_error_code) {
    return d.last_error_code.replaceAll("_", " ");
  }

  return "Not sent yet";
}

/*
 * successRate is the share of the last day's attempts that were
 * delivered, as a whole percent, or a dash when there were none.
 */
export function successRate(s: WebhookStats): string {
  if (s.attempts === 0) {
    return "–";
  }

  return `${Math.round((s.succeeded / s.attempts) * 100)}%`;
}

/*
 * validURL reports whether the text is an http or https address worth
 * sending; the server checks the rest.
 */
export function validURL(text: string): boolean {
  try {
    const url = new URL(text.trim());

    return url.protocol === "https:" || url.protocol === "http:";
  } catch {
    return false;
  }
}
