/*
 * The last time a workspace refused the person until they meet one of its
 * rules. The API client reports every such refusal here, so one screen
 * can explain any rule, a future one included, without each page having
 * to catch it. The page behind the screen stays where it is, so nothing
 * typed there is lost while the person sorts it out.
 */

export interface AccessRequirement {
  requirement: string;
  message: string;
  actionUrl: string | null;
  ip?: string;
}

let current: AccessRequirement | null = null;
const listeners = new Set<() => void>();

export function reportAccessRequirement(next: AccessRequirement | null) {
  if (current?.requirement === next?.requirement && current?.message === next?.message) {
    return;
  }

  current = next;

  for (const listener of listeners) {
    listener();
  }
}

export function accessRequirement(): AccessRequirement | null {
  return current;
}

export function subscribeToAccessRequirement(listener: () => void): () => void {
  listeners.add(listener);

  return () => {
    listeners.delete(listener);
  };
}
