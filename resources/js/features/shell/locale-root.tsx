/*
 * The language around the whole app. It reads in the language the person,
 * their workspace or their browser resolves to, and tells React Aria the
 * same one, so keyboard behavior, date fields and menus follow it; the
 * page's own lang and dir attributes follow too, for screen readers and
 * for right-to-left layouts. With one language served, it only ever asks
 * the server which languages there are.
 */
import { useQuery } from "@tanstack/react-query";
import { type ReactNode, useEffect } from "react";
import { useLocale } from "react-aria-components";

import { DirectionProvider } from "@/components/foundations/direction/direction";
import { authConfigQuery, meQuery } from "@/lib/api/auth";
import { sourceLocale } from "@/lib/i18n";
import { useLocaleSync } from "@/lib/i18n/locale-sync";

import { useCurrentWorkspace } from "./workspace";

/*
 * PageLanguage writes the language React Aria holds onto the page
 * itself.
 */
function PageLanguage() {
  const { locale, direction } = useLocale();

  useEffect(() => {
    document.documentElement.lang = locale;
    document.documentElement.dir = direction;
  }, [locale, direction]);

  return null;
}

export function LocaleRoot({ children }: { children: ReactNode }) {
  const { data: config } = useQuery(authConfigQuery);
  const enabled = config?.locales.map((locale) => locale.code) ?? [sourceLocale];
  const { data: me } = useQuery({ ...meQuery, enabled: enabled.length > 1 });
  const { current } = useCurrentWorkspace(me?.workspaces ?? []);
  const locale = useLocaleSync({ enabled, me, workspace: current?.id });

  return (
    <DirectionProvider locale={locale}>
      <PageLanguage />
      {children}
    </DirectionProvider>
  );
}
