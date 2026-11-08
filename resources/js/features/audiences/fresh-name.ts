/*
 * freshAudienceName is the first "Untitled audience" name no audience
 * has.
 */
export function freshAudienceName(taken: string[]): string {
  const names = new Set(taken.map((name) => name.toLowerCase()));
  for (let count = 1; ; count += 1) {
    const name = count === 1 ? "Untitled audience" : `Untitled audience ${count}`;
    if (!names.has(name.toLowerCase())) {
      return name;
    }
  }
}
