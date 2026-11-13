/*
 * The draft of the person's own time zone, held for the Preferences
 * page's save bar until it is sent or discarded.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { updateProfile } from "@/lib/api/account";

import { useAccount } from "../use-account";

/*
 * useTimeZoneDraft holds the person's time zone picked but not saved
 * yet: a zone, null to follow the workspace, or undefined while nothing
 * differs from what is saved. Save sends it and reads the person again;
 * Discard puts it back.
 */
export function useTimeZoneDraft() {
  const me = useAccount();
  const queryClient = useQueryClient();
  const [pending, setPending] = useState<string | null | undefined>(undefined);
  const save = useMutation({
    mutationFn: () => updateProfile({ timezone: pending ?? null }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["me"] });
      setPending(undefined);
    },
  });

  return {
    pending,
    dirty: pending !== undefined,
    save,
    pick: (zone: string | null) => setPending(zone === (me.timezone ?? null) ? undefined : zone),
    discard: () => setPending(undefined),
  };
}

export type TimeZoneDraft = ReturnType<typeof useTimeZoneDraft>;
