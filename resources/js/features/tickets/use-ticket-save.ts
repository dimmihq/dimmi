/*
 * Saving a change to a ticket safely. Every change sends the version the
 * page read, so a teammate's edit made in between comes back as a clash
 * instead of being overwritten; the page then offers to load the latest.
 * A change refused for missing required fields names each one, so the
 * page marks them in place.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast } from "@/components/base/toast/toast";
import { ApiError } from "@/lib/api/client";
import { type Ticket, type TicketChange, ticketKeys, updateTicket } from "@/lib/api/tickets";
import { i18n } from "@/lib/i18n";

/*
 * missingKeys reads the field keys a 422 says are missing.
 */
export function missingKeys(error: unknown): string[] {
  if (!(error instanceof ApiError) || error.code !== "required_fields_missing") {
    return [];
  }
  const missing = error.details.missing;

  return Array.isArray(missing) ? missing.map(String) : [];
}

/*
 * isClash reports whether a save failed because the ticket changed since
 * it was read.
 */
export function isClash(error: unknown): boolean {
  return (
    error instanceof ApiError && (error.status === 409 || error.status === 412) && error.code === "version_conflict"
  );
}

export function useTicketSave(workspace: string, ticket: Ticket | null | undefined) {
  const client = useQueryClient();
  const [clash, setClash] = useState(false);
  const [missing, setMissing] = useState<string[]>([]);
  const save = useMutation({
    mutationFn: (change: TicketChange) => {
      if (!ticket) {
        throw new Error("No ticket");
      }
      return updateTicket(workspace, ticket, change);
    },
    onSuccess: async () => {
      setMissing([]);
      setClash(false);
      await client.invalidateQueries({ queryKey: ticketKeys.all(workspace) });
    },
    onError: (error) => {
      if (isClash(error)) {
        setClash(true);
        return;
      }
      const keys = missingKeys(error);
      if (keys.length > 0) {
        setMissing(keys);
        toast.error(error.message);
        return;
      }
      toast.error((error as Error).message);
    },
  });

  return {
    save: (change: TicketChange) =>
      save.mutateAsync(change).then(
        (saved) => saved,
        () => undefined,
      ),
    busy: save.isPending,
    clash,
    missing,
    errorOf: (key: string) => (missing.includes(key) ? i18n.t("tickets:errors.missing") : undefined),
    reload: async () => {
      setClash(false);
      setMissing([]);
      await client.invalidateQueries({ queryKey: ticketKeys.all(workspace) });
    },
  };
}

export type TicketSave = ReturnType<typeof useTicketSave>;
