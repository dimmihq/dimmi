/*
 * freshName is the first "Untitled workflow" name no workflow has.
 */
export function freshName(taken: string[]): string {
  const names = new Set(taken.map((name) => name.toLowerCase()));
  for (let count = 1; ; count += 1) {
    const name = count === 1 ? "Untitled workflow" : `Untitled workflow ${count}`;
    if (!names.has(name.toLowerCase())) {
      return name;
    }
  }
}
