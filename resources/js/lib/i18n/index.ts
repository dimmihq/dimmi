/*
 * The language the browser app reads in. English is the source and ships
 * inside the main bundle, so the first page never waits for text; every
 * other language is a chunk of its own, fetched the first time someone
 * reads in it. A key a language lacks shows its English text, never a
 * blank. Messages live in resources/js/lang/<locale>/<namespace>.json,
 * one namespace per area of the app.
 */
import i18next, { type BackendModule, type ResourceLanguage } from "i18next";
import { initReactI18next } from "react-i18next";

/*
 * sourceLocale is the language every message is written in first, and
 * the last fallback of every other one.
 */
export const sourceLocale = "en";

/*
 * namespaces are the message files of each language.
 */
export const namespaces = ["common", "settings", "sla", "tickets", "trackers"] as const;

const bundled = import.meta.glob<ResourceLanguage>("../../lang/en/*.json", { eager: true, import: "default" });
const chunks = import.meta.glob<ResourceLanguage>(["../../lang/*/*.json", "!../../lang/en/*.json"], {
  import: "default",
});

function file(locale: string, namespace: string): string {
  return `../../lang/${locale}/${namespace}.json`;
}

function english(): Record<string, ResourceLanguage> {
  return Object.fromEntries(namespaces.map((namespace) => [namespace, bundled[file(sourceLocale, namespace)] ?? {}]));
}

/*
 * lazyMessages loads a language's namespace from its own chunk. A
 * language or namespace with no file loads as empty, so its keys show in
 * English.
 */
const lazyMessages: BackendModule = {
  type: "backend",
  init: () => undefined,
  read(language, namespace, callback) {
    const load = chunks[file(language, namespace)];
    if (!load) {
      callback(null, {});
      return;
    }

    load().then(
      (messages) => callback(null, messages),
      (error: Error) => callback(error, null),
    );
  },
};

i18next
  .use(lazyMessages)
  .use(initReactI18next)
  .init({
    lng: sourceLocale,
    fallbackLng: sourceLocale,
    ns: namespaces,
    defaultNS: "common",
    resources: { [sourceLocale]: english() },
    partialBundledLanguages: true,
    load: "currentOnly",
    initAsync: false,
    interpolation: { escapeValue: false },
    react: { useSuspense: false },
  });

export const i18n = i18next;

/*
 * activeLocale is the language the app reads in right now.
 */
export function activeLocale(): string {
  return i18next.language || sourceLocale;
}
