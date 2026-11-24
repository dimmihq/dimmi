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
  procedure: { width: 720, height: 781, ready: true },
  "procedure-hero": { width: 1680, height: 1000, ready: true },
  "operator-hero": { width: 1680, height: 1000, ready: true },
  "procedure-simulations": { width: 388, height: 740, ready: true },
  "procedure-history": { width: 720, height: 680, ready: true },
  "agent-voice": { width: 720, height: 486, ready: true },
  "help-center": { width: 1680, height: 1000, ready: false },
  "agent-studio-hero": { width: 1680, height: 1000, ready: true },
  "agent-studio-guidance": { width: 720, height: 609, ready: true },
  "agent-studio-simulations": { width: 388, height: 940, ready: true },
  "agent-studio-compare": { width: 792, height: 860, ready: true },
  "agent-studio-operator": { width: 373, height: 660, ready: true },
  "agent-studio-history": { width: 720, height: 680, ready: true },
  "routines-list": { width: 1680, height: 1000, ready: false },
  "routine-pr-diff": { width: 720, height: 640, ready: false },
  "routine-procedure-diff": { width: 720, height: 640, ready: false },
  "routine-thread": { width: 720, height: 640, ready: false },
  "routine-guardrails": { width: 720, height: 640, ready: false },
};
