/*
 * The site's frame and its page transitions. Every page is prerendered,
 * so a first visit or a reload is plain HTML. After that, a click on a
 * link to another page of the site swaps the page in place, the way the
 * app does: the navigation and footer stay still and the new page arrives
 * with the app's own reveal (animate-page-in: a fade, a small rise and a
 * blur that clears). Links with a modifier key, a target, or outside the
 * site still load the ordinary way.
 */
import { type MouseEvent, useEffect, useLayoutEffect, useRef, useState } from "react";

import { cx } from "@/utils/cx";

import { SiteFooter } from "./components/site-footer";
import { SiteNav } from "./components/site-nav";
import { findRoute } from "./routes";

// useLayoutEffect warns when it runs on the server; the prerender has no scroll to set.
const useBrowserLayoutEffect = typeof window === "undefined" ? useEffect : useLayoutEffect;

export function App({ path: initial }: { path: string }) {
  const [location, setLocation] = useState({ path: initial, hash: "", scroll: 0 });
  const navigated = useRef(false);
  const shown = useRef(location.path);
  shown.current = location.path;
  const route = findRoute(location.path);
  const Page = route?.page;

  // A link to another page of the site, clicked plainly, becomes an in-place swap.
  const onClick = (event: MouseEvent<HTMLDivElement>) => {
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    ) {
      return;
    }
    const link = (event.target as HTMLElement).closest("a");
    if (!link || link.target || link.hasAttribute("download") || link.origin !== window.location.origin) {
      return;
    }
    const next = findRoute(link.pathname);
    if (!next) {
      return;
    }
    // A hash on the page already showing is the browser's to scroll to.
    if (next.path === findRoute(window.location.pathname)?.path) {
      if (link.hash) {
        return;
      }
      event.preventDefault();
      window.scrollTo({ top: 0, behavior: "smooth" });
      return;
    }
    event.preventDefault();
    window.history.replaceState({ scroll: window.scrollY }, "");
    window.history.pushState({ scroll: 0 }, "", link.pathname + link.hash);
    navigated.current = true;
    setLocation({ path: next.path, hash: link.hash, scroll: 0 });
  };

  // Back and forward swap pages too, and return to where the reader was.
  useEffect(() => {
    const onPop = (event: PopStateEvent) => {
      // Moving between sections of the page already showing is the browser's.
      if (findRoute(window.location.pathname)?.path === shown.current) {
        return;
      }
      navigated.current = true;
      const scroll = (event.state as { scroll?: number } | null)?.scroll ?? 0;
      setLocation({ path: window.location.pathname, hash: window.location.hash, scroll });
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  // After a swap: the title and description follow the page, and the view
  // jumps to the top, the linked section, or the reader's place on the way back.
  useBrowserLayoutEffect(() => {
    if (!navigated.current) {
      return;
    }
    if (route) {
      document.title = route.title;
      document.querySelector('meta[name="description"]')?.setAttribute("content", route.description);
    }
    const target = location.hash ? document.getElementById(decodeURIComponent(location.hash.slice(1))) : null;
    if (target) {
      target.scrollIntoView({ behavior: "instant" });
    } else {
      window.scrollTo({ top: location.scroll, behavior: "instant" });
    }
  }, [location]);

  return (
    // The handler only reads clicks that bubble up from real links.
    // biome-ignore lint/a11y/noStaticElementInteractions: links inside stay the interactive elements
    // biome-ignore lint/a11y/useKeyWithClickEvents: Enter on a link fires the same click
    <div className="flex min-h-screen flex-col bg-background-full" onClick={onClick}>
      <SiteNav path={location.path} />
      <main key={location.path} className={cx("flex-1", navigated.current && "animate-page-in")}>
        {Page ? (
          <Page />
        ) : (
          <div className="mx-auto max-w-2xl px-6 py-40 text-center">
            <h1 className="text-title-1-semibold">Page not found</h1>
            <p className="mt-3 text-body-regular text-text-secondary">This page does not exist yet.</p>
          </div>
        )}
      </main>
      <SiteFooter />
    </div>
  );
}
