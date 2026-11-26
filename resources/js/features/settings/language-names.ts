/*
 * How the help center settings name a language.
 */
import type { Language } from "@/lib/api/translations";

/*
 * languageName names a language in English and in itself, such as
 * "French · Français".
 */
export function languageName(languages: Language[], code: string): string {
  const found = languages.find((l) => l.code === code);
  if (!found) {
    return code;
  }
  return found.name === found.native_name ? found.name : `${found.name} · ${found.native_name}`;
}
