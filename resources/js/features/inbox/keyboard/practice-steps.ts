/*
 * The practice page's lessons: one move at a time, each the key to press
 * and what it does, on sample conversations that never reach the server.
 * Progress is kept in the browser; when storage is unavailable the page
 * simply starts over.
 */

export interface PracticeStep {
  key: string;
  prompt: string;
}

export const practiceSteps: PracticeStep[] = [
  { key: "j", prompt: "Press J to move to the next conversation." },
  { key: "k", prompt: "Press K to move back up." },
  { key: "z", prompt: "Press Z to snooze it until tomorrow." },
  { key: "?", prompt: "Press ? to open the shortcut sheet any time." },
];

const storageKey = "dimmi:shortcuts:practice";

export function savedStep(): number {
  try {
    const n = Number(window.localStorage.getItem(storageKey) ?? "0");
    return Number.isInteger(n) && n >= 0 && n <= practiceSteps.length ? n : 0;
  } catch {
    return 0;
  }
}

export function saveStep(step: number) {
  try {
    window.localStorage.setItem(storageKey, String(step));
  } catch {
    return;
  }
}

/*
 * advance returns the step after pressing the key: the next one when it
 * is the key asked for, the same one otherwise.
 */
export function advance(step: number, key: string): number {
  return practiceSteps[step]?.key === key.toLowerCase() || practiceSteps[step]?.key === key ? step + 1 : step;
}
