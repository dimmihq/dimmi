/*
 * Who is looking at settings and in which workspace, for the pages that
 * show the person's own details or the workspace's.
 */
import { useSuspenseQuery } from "@tanstack/react-query";

import { useCurrentWorkspace } from "@/features/shell/workspace";
import { meQuery } from "@/lib/api/auth";

export function useAccount() {
  const { data: me } = useSuspenseQuery(meQuery);
  const { current } = useCurrentWorkspace(me?.workspaces ?? []);

  return {
    name: me?.user.name ?? "",
    email: me?.user.email ?? "",
    hasPassword: me?.user.has_password ?? false,
    workspace: {
      id: current?.id ?? "",
      name: current?.name ?? "Workspace",
      slug: current?.slug ?? "workspace",
      role: current?.role ?? "",
    },
    cloud: me?.instance?.edition === "cloud",
  };
}
