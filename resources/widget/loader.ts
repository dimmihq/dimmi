/*
 * The script customers paste. It stays tiny and never changes its address,
 * so it only reads its own tag, keeps every Dimmi(...) call made so far in
 * a queue, and adds the core module once the page has parsed. The server
 * fills in the core's hashed file name, its integrity hash and the assets
 * host when it serves this file, so a cached loader never asks for a core
 * that no longer exists. A second copy of the snippet on the same page does
 * nothing but warn.
 */
import { report } from "./lib/report";
import type { Command, DimmiWindow, Queue } from "./settings";

/*
 * Filled in by the server with the build it serves: the core's file name,
 * its integrity hash, and the assets host, empty for the loader's own.
 */
declare const __DIMMI_BUILD__: { core: string; integrity: string; assets: string };

export function load(win: Window & DimmiWindow, doc: Document, script: HTMLScriptElement | null): void {
  if (win.__dimmi) {
    report("warn", "boot_twice", "the Dimmi snippet is on this page more than once; remove the copies.");
    return;
  }

  const base = (script?.src ?? "").replace(/\/widget\/v1\/loader\.js.*$/, "");
  const data = script?.dataset ?? {};
  const queue: Queue =
    win.Dimmi ??
    ((...command: Command) => {
      queue.q = queue.q ?? [];
      queue.q.push(command);
    });
  win.Dimmi = queue;
  win.__dimmi = {
    appId: data.appId ?? "",
    apiBase: data.apiBase ?? base,
    identityUrl: data.identityUrl,
    latency: data.latency === undefined ? undefined : Number(data.latency),
    preview: data.preview === "true" || data.preview === "",
  };

  const mount = () => {
    const core = doc.createElement("script");
    core.type = "module";
    const build = __DIMMI_BUILD__;
    core.src = (build.assets || `${base}/widget/assets/`) + build.core;
    core.integrity = build.integrity;
    core.crossOrigin = "anonymous";
    core.onerror = () =>
      report(
        "error",
        "assets_blocked",
        `${core.src} did not load. An ad blocker or CSP may block it; see the proxy setup.`,
      );
    doc.head.append(core);
  };

  if (doc.readyState === "loading") {
    doc.addEventListener("DOMContentLoaded", mount, { once: true });
  } else {
    mount();
  }
}
