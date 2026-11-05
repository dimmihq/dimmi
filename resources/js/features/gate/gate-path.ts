/*
 * Where the shared password page of a private deployment lives. The
 * server sends every unadmitted page there and refuses every API call,
 * so whatever runs around every page asks the API nothing on it.
 */
export const GATE_PATH = "/__gate";

export function onGatePage(): boolean {
  return window.location.pathname === GATE_PATH;
}
