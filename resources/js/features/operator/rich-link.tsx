/*
 * A link in one of Operator's messages, drawn as a small chip: a glyph
 * for what it opens and its name, never a raw address. A page inside
 * dimmi opens in this window; a public page or another site opens in a
 * new tab and says so with an arrow.
 */
import {
  RiArrowRightUpLine,
  RiBookOpenLine,
  RiCalendarScheduleLine,
  RiChat3Line,
  RiGlobalLine,
  RiLightbulbLine,
  RiLinksLine,
  RiListOrdered2,
  RiMegaphoneLine,
} from "@remixicon/react";
import { useRouter } from "@tanstack/react-router";
import type { ComponentProps, ComponentType, MouseEvent } from "react";

import { type LinkKind, readLink } from "./link-kind";

const glyphs: Record<LinkKind, ComponentType<{ className?: string }>> = {
  conversation: RiChat3Line,
  article: RiBookOpenLine,
  procedure: RiListOrdered2,
  routine: RiCalendarScheduleLine,
  changelog: RiMegaphoneLine,
  request: RiLightbulbLine,
  page: RiLinksLine,
  public: RiGlobalLine,
  outside: RiLinksLine,
};

const chip =
  "inline-flex max-w-full items-center gap-1 rounded-md border border-border-button-default bg-background-primary-default px-1.5 py-px align-baseline text-body-2-medium text-text-primary no-underline shadow-xs outline-none transition-colors duration-150 hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring";

/*
 * written reports whether a link's text is only its address, as when
 * Operator pastes one instead of naming what it opens.
 */
function written(children: ComponentProps<"a">["children"], href: string): boolean {
  const text = typeof children === "string" ? children : Array.isArray(children) ? children.join("") : "";

  return text.trim() === "" || text.trim() === href || /^https?:\/\//.test(text.trim());
}

export function RichLink({ href = "", children }: ComponentProps<"a">) {
  const router = useRouter();
  const link = readLink(href, window.location.origin);
  const Glyph = glyphs[link.kind];
  const label = written(children, href) ? link.name : children;
  const open = (event: MouseEvent<HTMLAnchorElement>) => {
    if (!link.inApp || event.metaKey || event.ctrlKey || event.shiftKey || event.button !== 0) {
      return;
    }
    event.preventDefault();
    router.history.push(link.inApp);
  };

  return (
    <a
      href={link.inApp ?? href}
      title={link.inApp ? undefined : href}
      onClick={open}
      className={chip}
      {...(link.inApp ? {} : { target: "_blank", rel: "noopener noreferrer" })}
    >
      <Glyph aria-hidden className="size-3.5 shrink-0 text-foreground-icon-secondary" />
      <span className="min-w-0 truncate">{label}</span>
      {link.inApp ? null : <RiArrowRightUpLine aria-hidden className="size-3 shrink-0 text-foreground-icon-tertiary" />}
    </a>
  );
}
