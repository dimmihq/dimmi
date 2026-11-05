/*
 * What every section of the workspace security page shares: the rules as
 * the server has them, whether the person may change them, and one way of
 * saving that asks for the password when the server wants a fresh proof
 * and keeps the server's reason when it refuses.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { type AccessPolicyChange, accessPolicyQuery, updateAccessPolicy } from "@/lib/api/access-policy";
import { ApiError } from "@/lib/api/client";
import { callerQuery } from "@/lib/api/workspace";

import { useRecentAuth } from "../recent-auth";
import { useAccount } from "../use-account";

/*
 * failureText is the sentence for a refused change: the first rejected
 * field's message when the server names one, else its message.
 */
export function failureText(error: unknown): string {
  if (error instanceof ApiError) {
    return Object.values(error.fields)[0] ?? error.message;
  }

  return error instanceof Error ? error.message : "Something went wrong. Try again.";
}

export function useAccessPolicy() {
  const { workspace, hasPassword } = useAccount();
  const queryClient = useQueryClient();
  const query = accessPolicyQuery(workspace.id);
  const { data: policy } = useQuery(query);
  const { data: caller } = useQuery(callerQuery(workspace.id));
  const { run, prompt } = useRecentAuth(hasPassword);
  const [failure, setFailure] = useState<string | null>(null);

  const save = async (change: AccessPolicyChange): Promise<boolean> => {
    setFailure(null);

    try {
      return await run(async () => {
        queryClient.setQueryData(query.queryKey, await updateAccessPolicy(workspace.id, change));
      });
    } catch (error) {
      setFailure(failureText(error));

      return false;
    }
  };

  return {
    policy,
    canManage: caller?.member?.permissions.includes("security_policy:manage") ?? false,
    save,
    failure,
    prompt,
  };
}
