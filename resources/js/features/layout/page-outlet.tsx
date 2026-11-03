/*
 * Page transitions for the whole app. A layout renders PageOutlet where it
 * would render the router's Outlet, and each time the route below it
 * changes, the new page arrives with BoardUI's reveal: it fades in, rises
 * a few pixels and sharpens from a slight blur, on BoardUI's soft-landing
 * curve. Only the part of the screen that changed moves, so a layout's own
 * chrome stays still while its pages swap.
 *
 * The first page a layout shows does not animate: on a fresh load the page
 * should simply be there, and when a whole layout arrives, the outlet above
 * it is already revealing everything, so a second reveal inside would
 * double the movement.
 */
import { Outlet, useChildMatches } from "@tanstack/react-router";
import { useRef } from "react";

import { cx } from "@/utils/cx";

export function PageOutlet({ className }: { className?: string }) {
  const page = useChildMatches({ select: (matches) => matches[0]?.id ?? "" });
  const first = useRef(page);
  const navigated = useRef(false);

  if (page !== first.current) {
    navigated.current = true;
  }

  return (
    <div key={page} className={cx(navigated.current && "animate-page-in", className)}>
      <Outlet />
    </div>
  );
}
