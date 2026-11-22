/*
 * Uploading and removing the workspace's logo, shared by settings and
 * setup. A file over the limit is refused before any upload.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { meQuery } from "@/lib/api/auth";
import { removeLogo, uploadLogo } from "@/lib/api/workspace";

const tooLarge = "The picture must be 2 MB or smaller.";

/*
 * useLogo uploads and removes the workspace's logo, refreshing everything
 * that shows the workspace, the sidebar's workspace switcher among them.
 */
export function useLogo(workspace: string) {
  const queryClient = useQueryClient();
  const [refused, setRefused] = useState<string | null>(null);
  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ["workspace", workspace] }),
      queryClient.invalidateQueries({ queryKey: meQuery.queryKey }),
    ]);
  const upload = useMutation({ mutationFn: (file: File) => uploadLogo(workspace, file), onSuccess: refresh });
  const remove = useMutation({ mutationFn: () => removeLogo(workspace), onSuccess: refresh });

  return {
    upload,
    remove,
    failure: refused ?? upload.error?.message ?? remove.error?.message ?? null,
    pick: (file: File) => {
      setRefused(null);
      upload.mutate(file);
    },
    refuse: () => setRefused(tooLarge),
  };
}
