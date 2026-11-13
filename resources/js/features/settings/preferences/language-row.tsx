/*
 * The language rows: the person's own language on Preferences, and the
 * workspace's on the workspace page, which members who pick none and
 * mail to them fall back to. Both list the languages the server serves,
 * each in its own words, and stay hidden while it serves only one, since
 * a choice of one is no choice. The person's own pick waits in the
 * Preferences page's draft until its save bar sends it; then the app
 * changes language where it stands, without reloading. The workspace's
 * waits the same way in the workspace page's draft.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { updateProfile } from "@/lib/api/account";
import { authConfigQuery, type Locale } from "@/lib/api/auth";
import { workspaceQuery } from "@/lib/api/workspace";
import { applyLocale } from "@/lib/i18n/locale-sync";

import { Row } from "../layout";
import { useAccount } from "../use-account";

/*
 * followWorkspace is the choice that clears the person's own language.
 */
const followWorkspace = "workspace-default";

interface Option {
  id: string;
  label: string;
}

/*
 * LanguageSelect lists the options and says which one was picked.
 */
function LanguageSelect({
  label,
  options,
  value,
  onPick,
}: {
  label: string;
  options: Option[];
  value: string;
  onPick: (id: string) => void;
}) {
  return (
    <Select
      aria-label={label}
      size="sm"
      selectedKey={value}
      onSelectionChange={(key) => {
        if (key !== null && String(key) !== value) {
          onPick(String(key));
        }
      }}
      popoverClassName="w-56"
    >
      {options.map((option) => (
        <SelectItem key={option.id} id={option.id} textValue={option.label}>
          {option.label}
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * useLocales returns the languages the server serves, empty until it
 * says.
 */
function useLocales(): Locale[] {
  const { data: config } = useQuery(authConfigQuery);

  return config?.locales ?? [];
}

function nameOf(locales: Locale[], code: string): string {
  return locales.find((locale) => locale.code === code)?.name ?? code;
}

/*
 * useLanguageDraft holds the person's language picked but not saved yet:
 * a code, null to follow the workspace, or undefined while nothing
 * differs from what is saved. Save sends it, switches the app to it and
 * reads the person again; Discard puts it back.
 */
export function useLanguageDraft() {
  const me = useAccount();
  const queryClient = useQueryClient();
  const [pending, setPending] = useState<string | null | undefined>(undefined);
  const save = useMutation({
    mutationFn: () => updateProfile({ locale: pending ?? null }),
    onSuccess: async () => {
      if (pending) {
        await applyLocale(pending);
      }
      await queryClient.invalidateQueries({ queryKey: ["me"] });
      setPending(undefined);
      toast.success("Preferences saved");
    },
  });

  return {
    pending,
    dirty: pending !== undefined,
    save,
    pick: (locale: string | null) => setPending(locale === (me.locale ?? null) ? undefined : locale),
    discard: () => setPending(undefined),
  };
}

export type LanguageDraft = ReturnType<typeof useLanguageDraft>;

export function LanguageRow({ draft }: { draft: LanguageDraft }) {
  const { t } = useTranslation("settings");
  const me = useAccount();
  const locales = useLocales();
  const { data: workspace } = useQuery({ ...workspaceQuery(me.workspace.id), enabled: locales.length > 1 });

  if (locales.length < 2 || !workspace) {
    return null;
  }

  const options = [
    { id: followWorkspace, label: t("language.followWorkspace", { name: nameOf(locales, workspace.locale) }) },
    ...locales.map((locale) => ({ id: locale.code, label: locale.name })),
  ];
  const chosen = draft.pending === undefined ? me.locale : draft.pending;

  return (
    <Row label={t("language.label")} description={t("language.description")}>
      <LanguageSelect
        label={t("language.label")}
        options={options}
        value={locales.some((locale) => locale.code === chosen) ? (chosen ?? followWorkspace) : followWorkspace}
        onPick={(id) => draft.pick(id === followWorkspace ? null : id)}
      />
    </Row>
  );
}

/*
 * WorkspaceLanguageRow shows the workspace page's drafted language, or
 * the saved one while nothing is drafted, and hands a pick to the page.
 */
export function WorkspaceLanguageRow({
  workspace,
  value,
  onChange,
}: {
  workspace: string;
  value?: string;
  onChange: (locale: string) => void;
}) {
  const { t } = useTranslation("settings");
  const locales = useLocales();
  const { data: current } = useQuery({ ...workspaceQuery(workspace), enabled: locales.length > 1 });

  if (locales.length < 2 || !current) {
    return null;
  }

  return (
    <Row label={t("language.label")} description={t("language.workspaceDescription")}>
      <LanguageSelect
        label={t("language.workspaceLabel")}
        options={locales.map((locale) => ({ id: locale.code, label: locale.name }))}
        value={value ?? current.locale}
        onPick={onChange}
      />
    </Row>
  );
}
