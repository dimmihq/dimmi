/*
 * The widget's voice in the host page's console. A messenger that fails on
 * someone else's site must say why, loudly and in words the developer can
 * search, so every setup problem is reported here with a stable code, such
 * as "[dimmi] boot_twice: ...", never swallowed.
 */

export function report(level: "warn" | "error", code: string, message: string): void {
  globalThis.console[level](`[dimmi] ${code}: ${message}`);
}
