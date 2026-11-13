/*
 * Running the workspace: the rules every member signs in under. Admins
 * and owners change these; some actions are for owners only. The workspace's name and its
 * members live in workspace.tsx, its teams in teams.tsx and its audit
 * log in audit-log.tsx. Sign-in methods, session length and requiring
 * two-factor authentication wait in a draft: a bar at the foot says they
 * are unsaved, Discard puts them back and Save sends the rules in one
 * change and then the two-factor requirement, behind one password check
 * when the server wants a fresh proof. Leaving with unsaved changes asks
 * first.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
import { useState } from "react";

import { toast } from "@/components/base/toast/toast";
import type { AccessPolicyChange } from "@/lib/api/access-policy";
import { updateWorkspace, workspaceQuery } from "@/lib/api/workspace";

import { SettingsPage } from "../layout";
import { RequireTwoFactorSection } from "../require-two-factor";
import { SaveBar } from "../save-bar";
import { useAccount } from "../use-account";
import { IPAllowlistSections } from "../workspace-security/ip-allowlists";
import { SessionLengthSection } from "../workspace-security/session-length";
import { SignInRulesSection } from "../workspace-security/sign-in-rules";
import { refusedForOwnAccount, useAccessPolicy } from "../workspace-security/use-access-policy";

/*
 * same says whether a pending value is what the server already has.
 */
function same(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

/*
 * useSecurityDraft holds the sign-in and session rules changed but not
 * saved yet, laid over the policy as the server has it, and whether
 * two-factor authentication is to be required, undefined while it is as
 * the server has it. The domains box keeps its own text while typed, so
 * commas and spaces stay put. A requirement refused because the person
 * has no second factor themselves goes back to the server's, so the
 * switch says what holds while the bar says why.
 */
function useSecurityDraft() {
  const me = useAccount();
  const queryClient = useQueryClient();
  const access = useAccessPolicy();
  const { data: workspace } = useQuery(workspaceQuery(me.workspace.id));
  const [pending, setPending] = useState<AccessPolicyChange>({});
  const [domainsText, setDomainsText] = useState<string | null>(null);
  const [requireTwoFactor, setRequireTwoFactor] = useState<boolean | undefined>(undefined);
  const save = useMutation({
    mutationFn: async () => {
      const also =
        requireTwoFactor === undefined
          ? undefined
          : async () => {
              try {
                const saved = await updateWorkspace(me.workspace.id, { require_two_factor: requireTwoFactor });
                queryClient.setQueryData(workspaceQuery(me.workspace.id).queryKey, saved);
              } catch (error) {
                if (refusedForOwnAccount(error)) {
                  setRequireTwoFactor(undefined);
                }
                throw error;
              }
            };

      if (await access.save(pending, also)) {
        setPending({});
        setDomainsText(null);
        setRequireTwoFactor(undefined);
        toast.success("Security saved");
      }
    },
  });
  const policy = access.policy ? { ...access.policy, ...pending } : undefined;

  const set = (change: AccessPolicyChange) =>
    setPending((before) => {
      const next: Record<string, unknown> = { ...before, ...change };
      for (const [key, value] of Object.entries(change)) {
        if (access.policy && same(value, access.policy[key as keyof AccessPolicyChange])) {
          delete next[key];
        }
      }

      return next as AccessPolicyChange;
    });

  return {
    policy,
    canManage: access.canManage,
    pending,
    dirty: Object.keys(pending).length > 0 || requireTwoFactor !== undefined,
    domainsText,
    setDomainsText,
    set,
    requireTwoFactor,
    setRequireTwoFactor: (on: boolean) =>
      setRequireTwoFactor(workspace && on === workspace.require_two_factor ? undefined : on),
    save,
    failure: access.failure,
    prompt: access.prompt,
    discard: () => {
      setPending({});
      setDomainsText(null);
      setRequireTwoFactor(undefined);
      access.clearFailure();
    },
  };
}

export type SecurityDraft = ReturnType<typeof useSecurityDraft>;

export function WorkspaceSecuritySettings() {
  const me = useAccount();
  const draft = useSecurityDraft();
  useBlocker({
    shouldBlockFn: () => draft.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => draft.dirty,
  });

  return (
    <SettingsPage
      title="Security"
      description="Rules every member of the workspace signs in under."
      footer={
        <SaveBar
          dirty={draft.dirty}
          saving={draft.save.isPending}
          error={draft.failure}
          onSave={() => draft.save.mutate()}
          onDiscard={draft.discard}
        />
      }
    >
      <SignInRulesSection draft={draft} />
      <SessionLengthSection draft={draft} />
      <IPAllowlistSections />
      <RequireTwoFactorSection
        workspace={me.workspace.id}
        hasPassword={me.hasPassword}
        title="Authentication"
        value={draft.requireTwoFactor}
        onChange={draft.setRequireTwoFactor}
      />
      {draft.prompt}
    </SettingsPage>
  );
}
