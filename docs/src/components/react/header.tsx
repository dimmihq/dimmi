import {
  type RemixiconComponentType,
  RiArrowRightSLine,
  RiArrowRightUpLine,
  RiBookOpenLine,
  RiBracesLine,
  RiCloseLine,
  RiCodeSSlashLine,
  RiGithubFill,
  RiMenuLine,
  RiServerLine,
  RiShapesLine,
} from "@remixicon/react";

import { Chip } from "@/components/base/badges/chip";
import { ButtonLink } from "@/components/base/buttons/button";
import { IconButton, IconLinkButton } from "@/components/base/buttons/icon-button";
import { DimmiLogo } from "@/features/brand/logo";
import { cx } from "@/utils/cx";

import { stripBase, withBase } from "../../base";
import { APP_URL, GITHUB_URL, SITE_URL, sectionFor, sections } from "../../nav";

/*
 * The header's static parts, in the marketing site's frame. They render
 * to plain HTML; only search and the theme control hydrate.
 */

export function SiteTitle() {
  return (
    <a
      href={withBase("/")}
      aria-label="dimmi docs home"
      className="flex shrink-0 items-center gap-2.5 text-text-primary"
    >
      <DimmiLogo className="h-5.5 w-auto" title="dimmi" />
      {/* Down 2px: centred on the wordmark's lowercase letters, not its ascenders. */}
      <Chip variant="caption" color="neutral" className="translate-y-0.5">
        Docs
      </Chip>
    </a>
  );
}

export function SocialLinks() {
  return (
    <>
      <a
        href={SITE_URL}
        className="rounded-lg px-2.5 py-1.5 text-body-medium text-text-secondary transition-colors hover:bg-background-secondary-default hover:text-text-primary"
      >
        trydimmi.com
      </a>
      <IconLinkButton
        icon={RiGithubFill}
        size="small"
        href={GITHUB_URL}
        aria-label="dimmi on GitHub"
        data-traffic-event="github_click"
      />
    </>
  );
}

export function OpenApp() {
  return (
    <ButtonLink href={APP_URL} size="small" variant="primary" trailingIcon={RiArrowRightUpLine}>
      Open app
    </ButtonLink>
  );
}

// Opens the sidebar drawer through the popover API, so it needs no script.
export function MenuToggle({ open = false }: { open?: boolean }) {
  return (
    <IconButton
      icon={open ? RiCloseLine : RiMenuLine}
      size="small"
      aria-label={open ? "Close navigation" : "Open navigation"}
      popoverTarget="docs-sidebar"
      popoverTargetAction={open ? "hide" : "show"}
    />
  );
}

// The underline glides to the clicked tab across the page change, as
// BoardUI's tabs slide theirs (see the view transition in docs.css).
// Below lg the tabs move into the sidebar drawer (DrawerSections).
export function SectionTabs({ path }: { path: string }) {
  const current = sectionFor(stripBase(path));
  return (
    <nav aria-label="Docs sections" className="hidden gap-1 lg:flex">
      {sections.map((section) => {
        const selected = section === current;
        return (
          <a
            key={section.key}
            href={withBase(section.href)}
            aria-current={selected ? "page" : undefined}
            className={cx(
              "relative shrink-0 px-2.5 pt-1 pb-3 text-body-medium transition-colors",
              selected ? "text-text-primary" : "text-text-secondary hover:text-text-primary",
            )}
          >
            {section.label}
            {selected && (
              <span
                aria-hidden
                className="docs-tab-indicator absolute inset-x-2.5 bottom-0 h-0.5 rounded-full bg-text-primary"
              />
            )}
          </a>
        );
      })}
    </nav>
  );
}

const sectionIcons: Record<string, RemixiconComponentType> = {
  guides: RiBookOpenLine,
  developers: RiCodeSSlashLine,
  api: RiBracesLine,
  "self-hosting": RiServerLine,
  components: RiShapesLine,
};

// The header's tabs as rows at the top of the small-screen drawer.
export function DrawerSections({ path }: { path: string }) {
  const current = sectionFor(stripBase(path));
  return (
    <nav aria-label="Docs sections" className="flex flex-col gap-0.5">
      {sections.map((section) => {
        const Icon = sectionIcons[section.key] ?? RiBookOpenLine;
        const selected = section === current;
        return (
          <a
            key={section.key}
            href={withBase(section.href)}
            aria-current={selected ? "page" : undefined}
            className={cx(
              "flex min-h-9 items-center gap-2.5 rounded-2lg px-2 text-body-medium transition-colors",
              selected
                ? "bg-background-secondary-default text-text-primary"
                : "text-text-secondary hover:bg-background-secondary-hover hover:text-text-primary",
            )}
          >
            <Icon
              aria-hidden
              className={cx("size-4.5", selected ? "text-foreground-icon-primary" : "text-foreground-icon-secondary")}
            />
            {section.label}
          </a>
        );
      })}
    </nav>
  );
}

// Below lg: the bar under the header that opens the drawer and says where you are.
export function MobileBar({ group, title }: { group?: string; title: string }) {
  return (
    <button
      type="button"
      popoverTarget="docs-sidebar"
      popoverTargetAction="show"
      aria-label="Open navigation"
      className="flex h-11 w-full cursor-pointer items-center gap-2 px-4 text-start text-body-medium outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring focus-visible:ring-inset sm:px-6 lg:hidden"
    >
      <RiMenuLine aria-hidden className="size-4.5 shrink-0 text-foreground-icon-secondary" />
      {group ? (
        <>
          <span className="shrink-0 text-text-tertiary">{group}</span>
          <RiArrowRightSLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" />
        </>
      ) : null}
      <span className="min-w-0 truncate text-text-primary">{title}</span>
    </button>
  );
}
