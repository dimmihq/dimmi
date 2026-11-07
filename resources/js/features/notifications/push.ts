/*
 * Turning on push for this browser: register the dashboard's service
 * worker, ask the person's permission, subscribe with the install's key
 * and hand the subscription to the server. Each step says plainly what
 * stopped it, such as a browser without push or a permission refused.
 */
import { registerPushDevice } from "@/lib/api/notifications";

export type PushSupport = "supported" | "unsupported" | "denied";

/*
 * pushSupport reports whether this browser can receive push at all, and
 * whether the person already refused it.
 */
export function pushSupport(): PushSupport {
  if (typeof window === "undefined" || !("serviceWorker" in navigator) || !("PushManager" in window)) {
    return "unsupported";
  }
  return Notification.permission === "denied" ? "denied" : "supported";
}

/*
 * keyBytes turns a base64url VAPID key into the bytes subscribe wants.
 */
export function keyBytes(key: string): Uint8Array<ArrayBuffer> {
  const padded = key.replace(/-/g, "+").replace(/_/g, "/") + "=".repeat((4 - (key.length % 4)) % 4);
  const raw = atob(padded);
  const out = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) {
    out[i] = raw.charCodeAt(i);
  }
  return out;
}

const browsers: [RegExp, string][] = [
  [/Edg\//, "Edge"],
  [/Firefox\//, "Firefox"],
  [/Chrome\//, "Chrome"],
  [/Safari\//, "Safari"],
];

const systems: [RegExp, string][] = [
  [/iPhone|iPad/, "iOS"],
  [/Android/, "Android"],
  [/Mac OS X/, "macOS"],
  [/Windows/, "Windows"],
];

/*
 * deviceLabel names this browser in the device list.
 */
export function deviceLabel(userAgent: string): string {
  const browser = browsers.find(([pattern]) => pattern.test(userAgent))?.[1] ?? "Browser";
  const system = systems.find(([pattern]) => pattern.test(userAgent))?.[1] ?? "Linux";
  return `${browser} on ${system}`;
}

/*
 * enablePush subscribes this browser and registers it.
 */
export async function enablePush(publicKey: string): Promise<void> {
  if (pushSupport() === "unsupported") {
    throw new Error("This browser cannot receive push notifications.");
  }
  const registration = await navigator.serviceWorker.register("/sw.js", { scope: "/" });
  const permission = await Notification.requestPermission();
  if (permission !== "granted") {
    throw new Error("Notifications are blocked for this site. Allow them in your browser's settings.");
  }
  const existing = await registration.pushManager.getSubscription();
  const subscription =
    existing ??
    (await registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: keyBytes(publicKey) }));
  await registerPushDevice(subscription.toJSON(), deviceLabel(navigator.userAgent));
}
