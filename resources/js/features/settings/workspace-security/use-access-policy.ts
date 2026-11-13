/*
 * What every section of the workspace security page shares: the rules as
 * the server has them, whether the person may change them, and one way of
 * saving that asks for the password when the server wants a fresh proof
 * and keeps the server's reason when it refuses. A save may carry a change
 * to the workspace itself, such as requiring two-factor authentication,
 * sent after the rules behind the same password check.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { type AccessPolicyChange, accessPolicyQuery, updateAccessPolicy } from "@/lib/api/access-policy";
import { ApiError } from "@/lib/api/client";
import { callerQuery } from "@/lib/api/workspace";

import { useRecentAuth } from "../recent-auth";
import { useAccount } from "../use-account";

/*
 * refusedForOwnAccount reports whether the server kept the requirement
 * off because the person changing it has no second factor themselves.
 */
export function refusedForOwnAccount(error: unknown): boolean {
  return error instanceof ApiError && error.code === "two_factor_not_enabled";
}

/*
 * twoFactorRefusal says why the server kept the requirement as it was.
 */
export function twoFactorRefusal(error: unknown): string {
  if (refusedForOwnAccount(error)) {
    return "Turn on two-factor authentication for your own account first, under Security & access.";
  }

  return error instanceof Error ? error.message : "Something went wrong. Try again.";
}

/*
 * failureText is the sentence for a refused change: why two-factor could
 * not be required, the first rejected field's message when the server
 * names one, else its message.
 */
export function failureText(error: unknown): string {
  if (refusedForOwnAccount(error)) {
    return twoFactorRefusal(error);
  }

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

  /*
   * save sends the rule changes, when there are any, then the extra
   * workspace change, resolving true once both went through.
   */
  const save = async (change: AccessPolicyChange, also?: () => Promise<unknown>): Promise<boolean> => {
    setFailure(null);

    try {
      return await run(async () => {
        if (Object.keys(change).length > 0) {
          queryClient.setQueryData(query.queryKey, await updateAccessPolicy(workspace.id, change));
        }
        await also?.();
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
    clearFailure: () => setFailure(null),
    prompt,
  };
}
