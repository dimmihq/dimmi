/*
 * emailsIn reads the addresses a person typed or pasted, split on commas,
 * semicolons and whitespace, without duplicates. The server checks each
 * one, so this only splits.
 */
export function emailsIn(text: string): string[] {
  const parts = text
    .split(/[\s,;]+/)
    .map((part) => part.trim().toLowerCase())
    .filter((part) => part.length > 0);

  return [...new Set(parts)];
}
