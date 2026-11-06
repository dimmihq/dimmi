/*
 * What the single sign-on sections share: one way of sending a change
 * that asks for the password when the server wants a fresh proof, keeps
 * the server's reason when it refuses, and refreshes the connections and
 * domains afterwards, since a change to one can move the other.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useRecentAuth } from "../recent-auth";
import { useAccount } from "../use-account";
import { failureText } from "../workspace-security/use-access-policy";

export function useSSOChange() {
  const { workspace, hasPassword } = useAccount();
  const queryClient = useQueryClient();
  const { run, prompt } = useRecentAuth(hasPassword);
  const [failure, setFailure] = useState<string | null>(null);

  /*
   * send runs the change and resolves true once it went through. With
   * rethrow, a refusal also rejects, so a dialog stays open on it.
   */
  const send = async (change: (workspace: string) => Promise<unknown>, rethrow = false): Promise<boolean> => {
    setFailure(null);

    try {
      const done = await run(() => change(workspace.id));
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["workspace", workspace.id, "sso-connections"] }),
        queryClient.invalidateQueries({ queryKey: ["workspace", workspace.id, "organization-domains"] }),
      ]);

      return done;
    } catch (error) {
      setFailure(failureText(error));
      if (rethrow) {
        throw error;
      }

      return false;
    }
  };

  return { send, failure, prompt, workspace: workspace.id };
}
