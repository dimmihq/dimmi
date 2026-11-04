/*
 * The person's own settings: their profile, how the app behaves for them
 * and how they sign in. Nothing here changes anything for teammates. Each
 * section lives in its own file, and every control on these pages works.
 */
import { applyTheme, type ThemeMode, useThemeMode } from "@/components/application/theme/theme-toggle";

import { Choice } from "../controls";
import { Empty, Row, Section, SettingsPage } from "../layout";
import { PasswordSection } from "../password-dialog";
import { TimeZoneRows } from "../preferences/time-zone-row";
import { AvatarRow } from "../profile/avatar-row";
import { DeleteAccount } from "../profile/delete-account";
import { EmailRow } from "../profile/email-row";
import { LeaveWorkspace } from "../profile/leave-workspace";
import { NameRow } from "../profile/name-row";
import { RecentSecurityActivity } from "../security-activity";
import { SessionsSection } from "../sessions";
import { SignInMethodsSection } from "../sign-in-methods";
import { TwoFactorSection } from "../two-factor";
import { useAccount } from "../use-account";

export function ProfileSettings() {
  const me = useAccount();

  return (
    <SettingsPage title="Profile" description="How teammates and customers see you.">
      <Section title="You">
        <AvatarRow name={me.name} avatarUrl={me.avatarUrl} />
        <NameRow name={me.name} />
        <EmailRow email={me.email} hasPassword={me.hasPassword} />
      </Section>
      <LeaveWorkspace id={me.workspace.id} name={me.workspace.name} />
      <DeleteAccount email={me.email} hasPassword={me.hasPassword} />
    </SettingsPage>
  );
}

const themes: Record<string, ThemeMode> = { Light: "light", Dark: "dark" };

export function PreferencesSettings() {
  const theme = useThemeMode();

  return (
    <SettingsPage title="Preferences" description="How dimmi looks and behaves for you.">
      <Section title="Region">
        <TimeZoneRows />
      </Section>
      <Section title="Interface">
        <Row label="Theme" description="Remembered on this device.">
          <Choice
            label="Theme"
            options={Object.keys(themes)}
            value={theme === "dark" ? "Dark" : "Light"}
            onChange={(option) => applyTheme(themes[option] ?? "light")}
          />
        </Row>
      </Section>
    </SettingsPage>
  );
}

export function NotificationsSettings() {
  return (
    <SettingsPage title="Notifications" description="Choose what reaches you, and where.">
      <Section title="Channels">
        <Empty>Everything lands in the app for now.</Empty>
      </Section>
    </SettingsPage>
  );
}

export function SecuritySettings() {
  const me = useAccount();

  return (
    <SettingsPage title="Security & access" description="How you sign in, and where you are signed in.">
      <PasswordSection hasPassword={me.hasPassword} />
      <SignInMethodsSection hasPassword={me.hasPassword} />
      <TwoFactorSection hasPassword={me.hasPassword} />
      <SessionsSection hasPassword={me.hasPassword} />
      <RecentSecurityActivity />
    </SettingsPage>
  );
}
