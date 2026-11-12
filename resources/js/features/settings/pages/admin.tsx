/*
 * Running the workspace: the rules every member signs in under. Admins
 * and owners change these; some actions are for owners only. The workspace's name and its
 * members live in workspace.tsx, its teams in teams.tsx and its audit
 * log in audit-log.tsx.
 */
import { SettingsPage } from "../layout";
import { RequireTwoFactorSection } from "../require-two-factor";
import { useAccount } from "../use-account";
import { IPAllowlistSections } from "../workspace-security/ip-allowlists";
import { SessionLengthSection } from "../workspace-security/session-length";
import { SignInRulesSection } from "../workspace-security/sign-in-rules";

export function WorkspaceSecuritySettings() {
  const me = useAccount();

  return (
    <SettingsPage title="Security" description="Rules every member of the workspace signs in under.">
      <SignInRulesSection />
      <SessionLengthSection />
      <IPAllowlistSections />
      <RequireTwoFactorSection workspace={me.workspace.id} hasPassword={me.hasPassword} title="Authentication" />
    </SettingsPage>
  );
}
