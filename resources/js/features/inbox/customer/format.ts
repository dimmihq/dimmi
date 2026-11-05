/*
 * How the customer panel writes values: attributes as text, and times as
 * short dates in the reader's locale.
 */

/*
 * show turns an attribute's value into text: strings as they are, other
 * values as JSON.
 */
export function show(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }

  return JSON.stringify(value) ?? "";
}

/*
 * when writes a time as a short date in the reader's locale, or a dash
 * when there is none.
 */
export function when(iso: string | null | undefined): string {
  if (!iso) {
    return "—";
  }

  return new Date(iso).toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });
}
