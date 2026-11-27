/*
 * Thin wrapper around Rustrak's Web Traffic script (`window.webTraffic`) plus
 * first-touch UTM capture in sessionStorage. Calls are silent no-ops when the
 * script is not loaded (local builds, staging, self-hosted installs) or when
 * the payload breaks the collector's limits (< 100 chars name, < 4 KB object).
 */

export type TrackValue = string | number | boolean;
export type TrackData = Record<string, TrackValue>;

export const UTM_STORAGE_KEY = "dimmi_utm";

export const UTM_KEYS = ["utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term"] as const;

export type UtmKey = (typeof UTM_KEYS)[number];
export type UtmParams = Partial<Record<UtmKey, string>>;

const MAX_EVENT_NAME_LENGTH = 99;
const MAX_DATA_BYTES = 4095;
const MAX_UTM_VALUE_LENGTH = 200;

declare global {
  interface Window {
    webTraffic?: {
      track?: (name: string, data?: TrackData) => void;
    };
  }
}

type PendingEvent = {
  name: string;
  data?: TrackData;
};

let loadingScript = false;
const pendingEvents: PendingEvent[] = [];

function byteLength(text: string): number {
  if (typeof TextEncoder !== "undefined") {
    return new TextEncoder().encode(text).length;
  }
  return text.length;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return false;
  }
  const proto = Object.getPrototypeOf(value);
  return proto === Object.prototype || proto === null;
}

function sanitizeData(data: unknown): TrackData | undefined | null {
  if (data === undefined) {
    return undefined;
  }
  if (!isPlainObject(data)) {
    return null;
  }

  const clean: TrackData = {};
  for (const [key, value] of Object.entries(data)) {
    if (value === undefined) {
      continue;
    }
    if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
      if (typeof value === "number" && !Number.isFinite(value)) {
        return null;
      }
      clean[key] = value;
      continue;
    }
    return null;
  }

  if (Object.keys(clean).length === 0) {
    return undefined;
  }

  let serialized: string;
  try {
    serialized = JSON.stringify(clean);
  } catch {
    return null;
  }

  if (byteLength(serialized) > MAX_DATA_BYTES) {
    return null;
  }

  return clean;
}

function dispatch(name: string, data?: TrackData): boolean {
  if (typeof window === "undefined" || typeof window.webTraffic?.track !== "function") {
    return false;
  }
  try {
    if (data !== undefined) {
      window.webTraffic.track(name, data);
    } else {
      window.webTraffic.track(name);
    }
    return true;
  } catch {
    return false;
  }
}

/**
 * Registers a dynamically injected Web Traffic script so events fired while it
 * is downloading flush as soon as it finishes loading.
 */
export function attachTrafficScript(script: HTMLScriptElement): void {
  loadingScript = true;
  const finish = (flush: boolean) => {
    loadingScript = false;
    const queued = pendingEvents.splice(0, pendingEvents.length);
    if (flush) {
      for (const item of queued) {
        dispatch(item.name, item.data);
      }
    }
  };
  script.addEventListener("load", () => finish(true), { once: true });
  script.addEventListener("error", () => finish(false), { once: true });
}

/** Resets internal script-loading state (for tests). */
export function resetTrackState(): void {
  loadingScript = false;
  pendingEvents.length = 0;
}

/**
 * Counts a named event with optional plain-object properties.
 * Never send emails, names, or IDs that point to a person.
 */
export function track(name: string, data?: TrackData): void {
  if (typeof window === "undefined") {
    return;
  }
  const trimmed = name.trim();
  if (trimmed.length === 0 || trimmed.length > MAX_EVENT_NAME_LENGTH) {
    return;
  }
  const clean = sanitizeData(data);
  if (clean === null) {
    return;
  }
  if (dispatch(trimmed, clean)) {
    return;
  }
  if (loadingScript && pendingEvents.length < 32) {
    pendingEvents.push({ name: trimmed, data: clean });
  }
}

function parseUtm(search: string): UtmParams {
  const params = new URLSearchParams(search);
  const parsed: UtmParams = {};
  for (const key of UTM_KEYS) {
    const raw = params.get(key)?.trim();
    if (raw) {
      parsed[key] = raw.slice(0, MAX_UTM_VALUE_LENGTH);
    }
  }
  return parsed;
}

function readStoredUtm(): UtmParams | null {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    const raw = window.sessionStorage.getItem(UTM_STORAGE_KEY);
    if (!raw) {
      return null;
    }
    const parsed: unknown = JSON.parse(raw);
    if (!isPlainObject(parsed)) {
      return null;
    }
    const stored: UtmParams = {};
    for (const key of UTM_KEYS) {
      const value = parsed[key];
      if (typeof value === "string" && value.trim()) {
        stored[key] = value.trim().slice(0, MAX_UTM_VALUE_LENGTH);
      }
    }
    return Object.keys(stored).length > 0 ? stored : null;
  } catch {
    return null;
  }
}

/**
 * Captures first-touch `utm_*` parameters into sessionStorage. Once any UTM
 * parameters are saved for the current tab, later navigations keep them intact.
 */
export function captureUtm(search?: string): UtmParams {
  if (typeof window === "undefined") {
    return {};
  }
  const existing = readStoredUtm();
  if (existing) {
    return existing;
  }
  const incoming = parseUtm(search ?? window.location.search);
  if (Object.keys(incoming).length === 0) {
    return {};
  }
  try {
    window.sessionStorage.setItem(UTM_STORAGE_KEY, JSON.stringify(incoming));
  } catch {
    // Private browsing or blocked storage: still return what the URL carried.
  }
  return incoming;
}

/**
 * Reads first-touch `utm_*` parameters from sessionStorage, falling back to
 * the current URL query string if nothing was stored yet.
 */
export function readUtm(search?: string): UtmParams {
  return captureUtm(search);
}
