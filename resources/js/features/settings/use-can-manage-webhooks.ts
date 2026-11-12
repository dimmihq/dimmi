/*
 * Whether the person may add, change and test webhook endpoints and see
 * their secrets, from the permissions their role gives them.
 */
import { useQuery } from "@tanstack/react-query";

import { callerQuery } from "@/lib/api/workspace";

export function useCanManageWebhooks(workspace: string): boolean {
  const { data: caller } = useQuery(callerQuery(workspace));

  return (caller?.member?.permissions ?? []).includes("webhooks:write");
}
