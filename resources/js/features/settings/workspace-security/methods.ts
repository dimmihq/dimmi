/*
 * storedMethods is the list of sign-in methods to store for the ticked
 * ones: empty when all of them are ticked, which is no rule at all, so a
 * way the install adds later stays open.
 */
export function storedMethods(ticked: string[], available: string[]): string[] {
  return available.every((method) => ticked.includes(method)) ? [] : ticked;
}
