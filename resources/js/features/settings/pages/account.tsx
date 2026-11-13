/*
 * The person's own settings: their profile, how the app behaves for them
 * and how they sign in. Nothing here changes anything for teammates. Each
 * section lives in its own file, and every control on these pages works.
 */
import { useMutation } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";

import { applyTheme, type ThemeMode, useThemeMode } from "@/components/application/theme/theme-toggle";
import { toast } from "@/components/base/toast/toast";
import { PersonalSignature, usePersonalSignatureDraft } from "@/features/email/signatures/personal-signature";

import { Choice } from "../controls";
import { Row, Section, SettingsPage } from "../layout";
import { PasswordSection } from "../password-dialog";
import { LanguageRow, useLanguageDraft } from "../preferences/language-row";
import { TimeZoneRows, useTimeZoneDraft } from "../preferences/time-zone-row";
import { AvatarRow } from "../profile/avatar-row";
import { DeleteAccount } from "../profile/delete-account";
import { EmailRow } from "../profile/email-row";
import { LeaveWorkspace } from "../profile/leave-workspace";
import { NameRow } from "../profile/name-row";
import { SaveBar } from "../save-bar";
import { RecentSecurityActivity } from "../security-activity";
import { SessionsSection } from "../sessions";
import { SignInMethodsSection } from "../sign-in-methods";
import { TwoFactorSection } from "../two-factor";
import { useAccount } from "../use-account";

export function ProfileSettings() {
  const me = useAccount();
  const signature = usePersonalSignatureDraft(me.workspace.id);
  useBlocker({
    shouldBlockFn: () => signature.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => signature.dirty,
  });

  return (
    <SettingsPage
      title="Profile"
      description="How teammates and customers see you."
      footer={
        <SaveBar
          dirty={signature.dirty}
          saving={signature.save.isPending}
          error={signature.save.error?.message}
          onSave={() => signature.save.mutate()}
          onDiscard={signature.discard}
        />
      }
    >
      <Section title="You">
        <AvatarRow name={me.name} avatarUrl={me.avatarUrl} />
        <NameRow name={me.name} />
        <EmailRow email={me.email} hasPassword={me.hasPassword} />
      </Section>
      <PersonalSignature draft={signature} />
      <LeaveWorkspace id={me.workspace.id} name={me.workspace.name} />
      <DeleteAccount email={me.email} hasPassword={me.hasPassword} />
    </SettingsPage>
  );
}

const themes: Record<string, ThemeMode> = { Light: "light", Dark: "dark" };

/*
 * usePreferencesDraft holds the language and time zone picked but not
 * saved yet. Save sends only the ones that changed, the time zone first
 * so the app changes language last; Discard puts both back.
 */
function usePreferencesDraft() {
  const language = useLanguageDraft();
  const zone = useTimeZoneDraft();
  const save = useMutation({
    mutationFn: async () => {
      if (zone.dirty) {
        await zone.save.mutateAsync();
      }
      if (language.dirty) {
        await language.save.mutateAsync();
      } else {
        toast.success("Preferences saved");
      }
    },
  });

  return {
    language,
    zone,
    dirty: language.dirty || zone.dirty,
    save,
    discard: () => {
      save.reset();
      language.save.reset();
      zone.save.reset();
      language.discard();
      zone.discard();
    },
  };
}

/*
 * PreferencesSettings keeps the language and time zone picked in a
 * draft, saved from the bar at the foot; leaving with it unsaved asks
 * first. The theme only lives on this device, so it changes at once.
 */
export function PreferencesSettings() {
  const theme = useThemeMode();
  const draft = usePreferencesDraft();
  useBlocker({
    shouldBlockFn: () => draft.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => draft.dirty,
  });

  return (
    <SettingsPage
      title="Preferences"
      description="How dimmi looks and behaves for you."
      footer={
        <SaveBar
          dirty={draft.dirty}
          saving={draft.save.isPending}
          error={draft.save.error?.message}
          onSave={() => draft.save.mutate()}
          onDiscard={draft.discard}
        />
      }
    >
      <Section title="Region">
        <LanguageRow draft={draft.language} />
        <TimeZoneRows draft={draft.zone} />
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
