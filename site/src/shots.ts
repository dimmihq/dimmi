/*
 * The product shots site/shots/capture.ts takes, at their CSS size (the
 * files are twice that). A shot that is not captured yet shows a frame.
 */
export const SHOTS: Record<string, { width: number; height: number; ready: boolean }> = {
  inbox: { width: 1680, height: 1000, ready: true },
  "inbox-list": { width: 320, height: 660, ready: true },
  "inbox-details": { width: 384, height: 640, ready: true },
  "inbox-operator": { width: 765, height: 660, ready: true },
  "inbox-assign": { width: 531, height: 540, ready: true },
  "inbox-snooze": { width: 531, height: 540, ready: true },
  workflow: { width: 1680, height: 1000, ready: true },
  "workflow-canvas": { width: 742, height: 632, ready: true },
  procedure: { width: 664, height: 800, ready: true },
  "agent-voice": { width: 640, height: 486, ready: true },
  "help-center": { width: 1680, height: 1000, ready: false },
};
