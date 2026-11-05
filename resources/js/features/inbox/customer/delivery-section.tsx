/*
 * Whether mail to the customer is blocked, and why. The email channel
 * keeps the list of addresses it will not send to; this section reads it
 * for the contact and shows nothing when nothing is blocked or when the
 * workspace's server does not offer the list. Lifting a block takes the
 * permission the server checks, and a complaint takes its own.
 */
import { RiMailForbidLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/base/buttons/button";
import { ApiError, request } from "@/lib/api/client";
import { callerQuery } from "@/lib/api/workspace";

import { when } from "./format";

interface Suppression {
  id: string;
  email: string;
  reason: string;
  created_at: string;
}

const reasonText: Record<string, string> = {
  hard_bounce: "hard bounce",
  complaint: "complaint",
  unsubscribe: "unsubscribe",
  manual: "blocked by the team",
};

function suppressionsQuery(workspace: string, contact: string) {
  return {
    queryKey: ["workspace", workspace, "contacts", contact, "suppressions"] as const,
    queryFn: async () => {
      try {
        return await request<Suppression[]>(
          "GET",
          `/email-suppressions?filter[contact]=${encodeURIComponent(contact)}`,
          undefined,
          { workspace },
        );
      } catch (error) {
        if (error instanceof ApiError && error.status === 404) {
          return [];
        }
        throw error;
      }
    },
    retry: false,
  };
}

export function DeliverySection({ workspace, contact }: { workspace: string; contact: string }) {
  const client = useQueryClient();
  const query = suppressionsQuery(workspace, contact);
  const { data: suppressions = [] } = useQuery(query);
  const { data: caller } = useQuery(callerQuery(workspace));
  const held = caller?.member?.permissions ?? [];
  const lift = useMutation({
    mutationFn: (id: string) =>
      request<unknown>("DELETE", `/email-suppressions/${encodeURIComponent(id)}`, undefined, { workspace }),
    onSuccess: () => client.invalidateQueries({ queryKey: query.queryKey }),
  });

  if (suppressions.length === 0) {
    return null;
  }

  return (
    <section aria-label="Email delivery" className="flex flex-col gap-2 border-t border-separator-border pt-3">
      {suppressions.map((suppression) => {
        const canLift = held.includes(
          suppression.reason === "complaint" ? "email_suppressions:lift_complaint" : "email_suppressions:manage",
        );

        return (
          <div key={suppression.id} className="flex items-center gap-2">
            <RiMailForbidLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
            <span className="min-w-0 flex-1 text-body-2-regular text-text-primary">
              Delivery blocked: {reasonText[suppression.reason] ?? suppression.reason} on {when(suppression.created_at)}
            </span>
            {canLift ? (
              <Button variant="secondary" size="xs" onClick={() => lift.mutate(suppression.id)}>
                Lift
              </Button>
            ) : null}
          </div>
        );
      })}
    </section>
  );
}
