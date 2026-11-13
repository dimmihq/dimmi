/*
 * The draft of the teammate's own signature switch, held for the profile
 * page's save bar.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { toast } from "@/components/base/toast/toast";
import { signaturesKey } from "@/lib/api/email-signatures";

import { useSignatureToggles } from "./signature-toggles";

/*
 * usePersonalSignatureDraft holds the signature switch flipped but not
 * saved yet, for the page's save bar.
 */
export function usePersonalSignatureDraft(workspace: string) {
  const queryClient = useQueryClient();
  const toggles = useSignatureToggles(workspace);
  const save = useMutation({
    mutationFn: () => toggles.send(),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: signaturesKey(workspace) });
      toggles.clear();
      toast.success("Signature saved");
    },
  });

  return {
    toggles,
    save,
    dirty: toggles.dirty,
    discard: () => {
      save.reset();
      toggles.clear();
    },
  };
}

export type PersonalSignatureDraft = ReturnType<typeof usePersonalSignatureDraft>;
