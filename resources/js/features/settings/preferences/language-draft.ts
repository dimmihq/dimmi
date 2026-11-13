/*
 * The draft of the person's own language, held for the Preferences
 * page's save bar until it is sent or discarded.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast } from "@/components/base/toast/toast";
import { updateProfile } from "@/lib/api/account";
import { applyLocale } from "@/lib/i18n/locale-sync";

import { useAccount } from "../use-account";

/*
 * useLanguageDraft holds the person's language picked but not saved yet:
 * a code, null to follow the workspace, or undefined while nothing
 * differs from what is saved. Save sends it, switches the app to it and
 * reads the person again; Discard puts it back.
 */
export function useLanguageDraft() {
  const me = useAccount();
  const queryClient = useQueryClient();
  const [pending, setPending] = useState<string | null | undefined>(undefined);
  const save = useMutation({
    mutationFn: () => updateProfile({ locale: pending ?? null }),
    onSuccess: async () => {
      if (pending) {
        await applyLocale(pending);
      }
      await queryClient.invalidateQueries({ queryKey: ["me"] });
      setPending(undefined);
      toast.success("Preferences saved");
    },
  });

  return {
    pending,
    dirty: pending !== undefined,
    save,
    pick: (locale: string | null) => setPending(locale === (me.locale ?? null) ? undefined : locale),
    discard: () => setPending(undefined),
  };
}

export type LanguageDraft = ReturnType<typeof useLanguageDraft>;
