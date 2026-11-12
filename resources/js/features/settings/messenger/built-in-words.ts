/*
 * The messenger's built-in words in one language, read from the same
 * language files the messenger ships. Each language loads only when the
 * texts tab asks for it, so the dashboard never carries them all.
 */
import { useQuery } from "@tanstack/react-query";

const files = import.meta.glob<Record<string, string>>("../../../../widget/lang/*.json", { import: "default" });

/*
 * builtInWords returns the language's built-in words, English when the
 * messenger has no file for it.
 */
export async function builtInWords(locale: string): Promise<Record<string, string>> {
  const load = files[`../../../../widget/lang/${locale}.json`] ?? files["../../../../widget/lang/en.json"];

  return load ? await load() : {};
}

export function useBuiltInWords(locale: string) {
  return useQuery({
    queryKey: ["messenger-built-in-words", locale],
    queryFn: () => builtInWords(locale),
    staleTime: Number.POSITIVE_INFINITY,
  }).data;
}
