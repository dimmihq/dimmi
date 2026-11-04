/*
 * Which language the app reads in, kept in step with the server. A
 * signed-in person reads in the language they picked, else in their
 * workspace's, else in the best match for their browser among the ones
 * the server serves, else in English: the same order the server follows,
 * so the page and every message the server writes agree. Before sign-in
 * only the browser counts. Nothing here ever reloads the page.
 */
import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";

import type { Me } from "@/lib/api/auth";
import { workspaceQuery } from "@/lib/api/workspace";

import { i18n, sourceLocale } from ".";

/*
 * LocaleChoices are the languages that may decide: the ones the server
 * serves, the person's own pick, their workspace's, and the browser's in
 * order of preference.
 */
export interface LocaleChoices {
  enabled: string[];
  user?: string | null;
  workspace?: string | null;
  browser: readonly string[];
}

function base(locale: string): string {
  return locale.split("-")[0]?.toLowerCase() ?? "";
}

/*
 * matchLocale returns the served language closest to the one wanted:
 * the same one, else the plain language, such as "fr" for "fr-CA", else
 * a regional one of the same language, such as "pt-BR" for "pt".
 */
export function matchLocale(wanted: string | null | undefined, enabled: string[]): string | null {
  if (!wanted) {
    return null;
  }

  const lower = wanted.toLowerCase();
  const exact = enabled.find((locale) => locale.toLowerCase() === lower);
  const plain = enabled.find((locale) => locale.toLowerCase() === base(wanted));
  const regional = enabled.find((locale) => base(locale) === base(wanted));

  return exact ?? plain ?? regional ?? null;
}

/*
 * resolveLocale picks the language to read in from the choices, in the
 * order the server follows.
 */
export function resolveLocale(choices: LocaleChoices): string {
  for (const wanted of [choices.user, choices.workspace, ...choices.browser]) {
    const match = matchLocale(wanted, choices.enabled);
    if (match) {
      return match;
    }
  }

  return choices.enabled.includes(sourceLocale) ? sourceLocale : (choices.enabled[0] ?? sourceLocale);
}

/*
 * applyLocale switches the app to the language, loading its messages
 * first when it has never been read in.
 */
export async function applyLocale(locale: string): Promise<void> {
  if (i18n.language !== locale) {
    await i18n.changeLanguage(locale);
  }
}

/*
 * SyncInput is what the app knows: the languages the server serves, the
 * signed-in caller (null when signed out, undefined while still asking)
 * and the workspace on screen.
 */
export interface SyncInput {
  enabled: string[];
  me: Me | null | undefined;
  workspace?: string;
}

/*
 * useLocaleSync keeps the app in the language the choices resolve to and
 * returns the language it reads in now. The workspace's language is only
 * asked for when it counts: several languages served, and a signed-in
 * person who picked none. Nothing switches until every choice is known,
 * so a person never sees a language flash by.
 */
export function useLocaleSync({ enabled, me, workspace }: SyncInput): string {
  const { i18n: active } = useTranslation();
  const several = enabled.length > 1;
  const own = me?.user.locale ?? null;
  const needsWorkspace = several && Boolean(me) && own === null && Boolean(workspace);
  const { data: current, isPending } = useQuery({ ...workspaceQuery(workspace ?? ""), enabled: needsWorkspace });
  const settled = !several || (me !== undefined && !(needsWorkspace && isPending));

  const locale = resolveLocale({ enabled, user: own, workspace: current?.locale, browser: navigator.languages });

  useEffect(() => {
    if (settled) {
      void applyLocale(locale);
    }
  }, [locale, settled]);

  return active.language || sourceLocale;
}
