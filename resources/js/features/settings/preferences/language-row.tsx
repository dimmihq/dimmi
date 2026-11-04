/*
 * The language rows: the person's own language on Preferences, and the
 * workspace's on the workspace page, which members who pick none and
 * mail to them fall back to. Both list the languages the server serves,
 * each in its own words, and stay hidden while it serves only one, since
 * a choice of one is no choice. Picking saves at once and the app
 * changes language where it stands, without reloading.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import { Select, SelectItem } from "@/components/base/select/select";
import { updateProfile } from "@/lib/api/account";
import { authConfigQuery, type Locale } from "@/lib/api/auth";
import { updateWorkspace, workspaceQuery } from "@/lib/api/workspace";
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

function nameOf(locales: Locale[], code: string | undefined): string {
  return locales.find((locale) => locale.code === code)?.name ?? code ?? "";
}

export function LanguageRow() {
  const { t } = useTranslation("settings");
  const me = useAccount();
  const locales = useLocales();
  const queryClient = useQueryClient();
  const { data: workspace } = useQuery({ ...workspaceQuery(me.workspace.id), enabled: locales.length > 1 });

  const save = useMutation({
    mutationFn: (locale: string | null) => updateProfile({ locale }),
    onSuccess: async (_, locale) => {
      if (locale) {
        await applyLocale(locale);
      }

      return queryClient.invalidateQueries({ queryKey: ["me"] });
    },
  });

  if (locales.length < 2) {
    return null;
  }

  const options = [
    { id: followWorkspace, label: t("language.followWorkspace", { name: nameOf(locales, workspace?.locale) }) },
    ...locales.map((locale) => ({ id: locale.code, label: locale.name })),
  ];

  return (
    <Row label={t("language.label")} description={save.error?.message ?? t("language.description")}>
      <LanguageSelect
        label={t("language.label")}
        options={options}
        value={me.locale ?? followWorkspace}
        onPick={(id) => save.mutate(id === followWorkspace ? null : id)}
      />
    </Row>
  );
}

export function WorkspaceLanguageRow({ workspace }: { workspace: string }) {
  const { t } = useTranslation("settings");
  const locales = useLocales();
  const queryClient = useQueryClient();
  const { data: current } = useQuery({ ...workspaceQuery(workspace), enabled: locales.length > 1 });

  const save = useMutation({
    mutationFn: (locale: string) => updateWorkspace(workspace, { locale }),
    onSuccess: (saved) => queryClient.setQueryData(workspaceQuery(workspace).queryKey, saved),
  });

  if (locales.length < 2 || !current) {
    return null;
  }

  return (
    <Row label={t("language.label")} description={save.error?.message ?? t("language.workspaceDescription")}>
      <LanguageSelect
        label={t("language.workspaceLabel")}
        options={locales.map((locale) => ({ id: locale.code, label: locale.name }))}
        value={current.locale}
        onPick={(id) => save.mutate(id)}
      />
    </Row>
  );
}
