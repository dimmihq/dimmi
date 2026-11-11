/*
 * What the composer needs to know about voice notes before any recording
 * starts: whether this page may record at all, and how long a note may
 * run. The recorder itself loads only when a visitor starts recording.
 */

/*
 * maxSeconds is the longest voice note; the recording finishes itself
 * there.
 */
export const maxSeconds = 300;

/*
 * canRecord reports whether this page may record: the browser can, and
 * the page is served securely, since browsers keep the microphone from
 * any other.
 */
export function canRecord(win: Window = window): boolean {
  return Boolean(win.isSecureContext && "mediaDevices" in win.navigator && "MediaRecorder" in win);
}
