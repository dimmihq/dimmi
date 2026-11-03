"use client";

import { useTemplateCopy } from "@/components/foundations/template-copy/template-copy";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ComponentType,
  type ReactNode,
} from "react";
import { RiAddLine, RiCloseLine, RiSearchLine, RiSideBarFill } from "@remixicon/react";
import { Link } from "@tanstack/react-router";
import { ThemeToggle } from "@/components/application/theme/theme-toggle";
import { Badge } from "@/components/base/badges/badge";
import { CloseButton } from "@/components/base/buttons/close-button";
import { Kbd } from "@/components/base/kbd/kbd";
import { cx } from "@/utils/cx";
import { DashboardTeamMenu, type MenuGroup } from "./dashboard-team-menu";
import { type AvatarColor, DashboardUserMenu } from "./dashboard-user-menu";

/**
 * Figma sources:
 *   expanded  → Board UI → dashboard 1 → Sidebar (node 3731:2934)
 *   collapsed → Board UI → Sidebar (node 3768:3382)
 *
 * Floating sidebar panel. Expanded: 260px wide, p 12, radius/3xl (24px),
 * white 1px border, "Background/Sidebar Elevation" shadow, bg
 * background/secondary. Collapsed: 60px wide (36px icon items + 12px
 * padding); collapse button sits centered above the workspace avatar, quick
 * search becomes a 36px neutral-200 square, nav items become icon-only
 * squares, the team card reduces to its avatar.
 *
 * The two states morph into each other: the panel width animates while
 * labels / badges / kbd collapse via max-width + opacity.
 *
 * Dimmi: the rows come in labelled sections from the host, links inside the
 * app go through the router, the switcher at the top and the card at the
 * foot take the host's data and menus, and a footer slot above the theme
 * control carries the host's extras, such as a getting-started checklist.
 */

type IconComponent = ComponentType<{
  className?: string;
  "aria-hidden"?: boolean | "true" | "false";
}>;

/**
 * Collapsible text/badge slot: blurs + fades + shrinks away when the rail
 * closes, and blurs back in on expand. The icons/rows themselves stay pinned in
 * place — only these label/badge slots animate — so nothing jumps to center.
 */
function Collapsible({ collapsed, children, className }: { collapsed: boolean; children: ReactNode; className?: string }) {
  const localize = useTemplateCopy();
  return localize((
    <span
      className={cx(
        "flex min-w-0 items-center overflow-hidden transition-[max-width,opacity,filter] duration-300 ease-in-out",
        // Expanded, the cap is the row itself: a fixed cap (it was 160px)
        // clipped any label wider than it, "Components and Blocks" included.
        collapsed ? "max-w-0 opacity-0 blur-[3px]" : "max-w-full opacity-100 blur-0",
        className,
      )}
    >
      {children}
    </span>
  ));
}

/**
 * Row density. `regular` is the Figma 36px row (8px padding around a 20px
 * icon). `compact` is a 28px row with a 16px icon, Body 2 labels and
 * tighter section spacing, for sidebars with many pages such as settings.
 */
export type SidebarDensity = "regular" | "compact";

const DensityContext = createContext<SidebarDensity>("regular");

const density = {
  regular: { row: "p-2", icon: "size-5", text: "text-body-medium", rail: "w-9", rows: "gap-1", heading: "h-7" },
  compact: { row: "px-2 py-1", icon: "size-4", text: "text-body-2-medium", rail: "w-8", rows: "gap-0.5", heading: "h-6" },
} as const;

function NavItem({
  icon: Icon,
  label,
  badge,
  isSelected = false,
  collapsed = false,
  href = "#",
  onClick,
}: {
  icon: IconComponent;
  label: string;
  badge?: ReactNode;
  isSelected?: boolean;
  collapsed?: boolean;
  href?: string;
  /** Action rows (e.g. Settings → modal) intercept the navigation. */
  onClick?: () => void;
}) {
  const localize = useTemplateCopy();
  const sizes = density[useContext(DensityContext)];
  const Anchor = href.startsWith("/") && !onClick ? RouterAnchor : "a";
  return localize((
    <Anchor
      href={href}
      onClick={
        onClick
          ? (event) => {
              event.preventDefault();
              onClick();
            }
          : undefined
      }
      aria-current={isSelected ? "page" : undefined}
      aria-label={label}
      title={collapsed ? label : undefined}
      className={cx(
        "flex items-center justify-between overflow-hidden rounded-2lg",
        sizes.row,
        "transition-[width,background-color] duration-300 ease-in-out",
        collapsed ? sizes.rail : "w-full",
        isSelected
          ? "bg-linear-to-b from-accent-500 to-accent-600 shadow-nav-selected"
          : "hover:bg-background-secondary-hover",
      )}
    >
      <span className="flex min-w-0 items-center gap-2">
        <Icon
          className={cx(sizes.icon, "shrink-0", isSelected ? "text-white" : "text-foreground-icon-secondary")}
          aria-hidden
        />
        <Collapsible collapsed={collapsed}>
          <span
            className={cx(
              sizes.text,
              "whitespace-nowrap",
              isSelected ? "text-white" : "text-text-secondary",
            )}
          >
            {label}
          </span>
        </Collapsible>
      </span>
      {badge && <Collapsible collapsed={collapsed}>{badge}</Collapsible>}
    </Anchor>
  ));
}

/** An in-app link: the router navigates without reloading the page. */
function RouterAnchor({ href, ...props }: React.ComponentProps<"a"> & { href: string }) {
  return <Link to={href} {...props} />;
}

/** A primary navigation row. Rows without an `href` are decoration only. */
export interface DashboardNavItem {
  key: string;
  label: string;
  icon: IconComponent;
  href?: string;
  badge?: string | number;
}

/** Kept as a name for callers that typed their `selected` prop; any key works. */
export type DashboardNavKey = string;

/** A group of rows under an optional heading, with an optional action beside it. */
export interface DashboardNavSection {
  key: string;
  label?: string;
  items: DashboardNavItem[];
  /** E.g. "+" to create a view: shown beside the heading. */
  action?: { label: string; onClick: () => void };
}

/** The workspace switcher at the top: what it shows and its panel. */
export interface DashboardSwitcher {
  label: string;
  initials: string;
  color?: AvatarColor;
  menuLabel: string;
  menu: (close: () => void) => ReactNode;
}

/** The card at the foot: who it stands for and its menu. */
export interface DashboardAccount {
  name: string;
  detail: string;
  initials: string;
  src?: string;
  groups: MenuGroup[];
  product: string;
  version?: string;
}

/**
 * The primary rows. A component of its own so the closures over `collapsed`
 * and the search query live here: built inline in the sidebar, the compiler
 * could not tell they leave `mobile` untouched and dropped the sidebar's
 * manual memoization.
 */
function NavSections({
  sections,
  query,
  selected,
  collapsed,
  secondaryMatch,
}: {
  sections: DashboardNavSection[];
  query: string;
  selected: string;
  collapsed: boolean;
  /** Whether a secondary row matches, so "No results" only shows when nothing does. */
  secondaryMatch: boolean;
}) {
  const localize = useTemplateCopy();
  const sizes = density[useContext(DensityContext)];
  const shown = sections
    .map((section) => ({
      ...section,
      items: section.items.filter((item) => item.label.toLocaleLowerCase().includes(query)),
    }))
    .filter((section) => section.items.length > 0 || (section.action && !query));
  if (shown.length === 0 && !secondaryMatch && !collapsed) {
    return localize(<p className="px-2 py-3 text-body-regular text-text-tertiary">No results</p>);
  }
  return shown.map((section) => (
    <div key={section.key} className={cx("flex w-full flex-col", sizes.rows)}>
      {section.label ? (
        <div className={cx("flex items-center justify-between px-2", sizes.heading, collapsed && "hidden")}>
          <span className="text-caption-1-medium whitespace-nowrap text-text-tertiary">{section.label}</span>
          {section.action ? (
            <button
              type="button"
              aria-label={section.action.label}
              title={section.action.label}
              onClick={section.action.onClick}
              className="flex size-5 cursor-pointer items-center justify-center rounded-md text-foreground-icon-tertiary hover:bg-background-secondary-hover"
            >
              <RiAddLine className="size-4" aria-hidden />
            </button>
          ) : null}
        </div>
      ) : null}
      <NavRows items={section.items} selected={selected} collapsed={collapsed} />
    </div>
  ));
}

function NavRows({ items, selected, collapsed }: { items: DashboardNavItem[]; selected: string; collapsed: boolean }) {
  return items.map((item) => {
      const isSelected = selected === item.key;
      return (
        <NavItem
          key={item.key}
          icon={item.icon}
          label={item.label}
          href={item.href}
          isSelected={isSelected}
          collapsed={collapsed}
          badge={
            item.badge !== undefined ? (
              <Badge color={isSelected ? "primary" : "neutral"}>{item.badge}</Badge>
            ) : undefined
          }
        />
      );
    });
}

export function DashboardSidebar({
  mobile = false,
  onClose,
  fluid = false,
  showThemeToggle = true,
  selected = "",
  sections,
  secondary = [],
  switcher,
  account,
  footer,
  collapsed: collapsedProp,
  onCollapsedChange,
  flat = false,
  density: densityProp = "regular",
  className,
}: {
  /** Rendered inside the mobile drawer: always expanded, close button instead of collapse. */
  mobile?: boolean;
  onClose?: () => void;
  /** Expanded width fills its container below `lg` instead of the fixed
   *  260px (e.g. the landing page, where the sidebar isn't in a drawer).
   *  Collapsed width stays the fixed 60px rail at every breakpoint — the
   *  whole point of collapsing is to shrink, so it must never get
   *  overridden back to full width. */
  fluid?: boolean;
  /** Hide the app-level theme control when the sidebar is used as marketing artwork. */
  showThemeToggle?: boolean;
  /** Which nav item shows the selected (filled blue) state. */
  selected?: DashboardNavKey;
  /** Primary navigation, in labelled sections. */
  sections: DashboardNavSection[];
  /** Rows above the account card, such as Settings. */
  secondary?: DashboardNavItem[];
  switcher: DashboardSwitcher;
  account: DashboardAccount;
  /** Extras above the theme control, hidden while the rail is collapsed. */
  footer?: ReactNode;
  /** Dimmi: set to control the rail from outside, e.g. folding it while a conversation is open. */
  collapsed?: boolean;
  onCollapsedChange?: (collapsed: boolean) => void;
  /** Removes the floating panel treatment for a sidebar revealed beneath mobile content. */
  flat?: boolean;
  /** Row height: `regular` 36px (default) or `compact` 28px. */
  density?: SidebarDensity;
  className?: string;
}) {
  const localize = useTemplateCopy();
  const [ownCollapsed, setOwnCollapsed] = useState(false);
  const collapsedState = collapsedProp ?? ownCollapsed;
  const setCollapsed = (next: boolean) => {
    setOwnCollapsed(next);
    onCollapsedChange?.(next);
  };
  const [suppressUserHover, setSuppressUserHover] = useState(false);
  const [searchActive, setSearchActive] = useState(false);
  const [query, setQuery] = useState("");
  const searchTriggerRef = useRef<HTMLButtonElement>(null);
  const searchFieldRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const collapsed = mobile ? false : collapsedState;
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const matches = (label: string) => label.toLocaleLowerCase().includes(normalizedQuery);
  const secondaryMatch = secondary.some((item) => matches(item.label));

  const activateSearch = useCallback(() => {
    if (!mobile) setCollapsed(false);
    setSearchActive(true);
  }, [mobile]);

  const deactivateSearch = useCallback((restoreFocus: boolean) => {
    setQuery("");
    setSearchActive(false);
    if (restoreFocus) {
      window.requestAnimationFrame(() => searchTriggerRef.current?.focus());
    }
  }, []);

  useEffect(() => {
    if (!searchActive) return;
    const frame = window.requestAnimationFrame(() => searchInputRef.current?.focus());
    return () => window.cancelAnimationFrame(frame);
  }, [searchActive]);

  useEffect(() => {
    if (!searchActive) return;

    const onOutsideClick = (event: MouseEvent) => {
      const target = event.target;
      if (target instanceof Node && searchFieldRef.current?.contains(target)) return;
      deactivateSearch(false);
    };

    document.addEventListener("click", onOutsideClick);
    return () => document.removeEventListener("click", onOutsideClick);
  }, [deactivateSearch, searchActive]);

  useEffect(() => {
    const onShortcut = (event: KeyboardEvent) => {
      if (event.key.toLocaleLowerCase() === "l" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        activateSearch();
      }
    };

    window.addEventListener("keydown", onShortcut);
    return () => window.removeEventListener("keydown", onShortcut);
  }, [activateSearch]);

  return localize((
    <DensityContext.Provider value={densityProp}>
    <aside
      className={cx(
        "flex h-full shrink-0 flex-col justify-between overflow-hidden",
        flat
          ? "bg-background-full"
          : "rounded-3xl border border-border-button-white bg-background-secondary-default shadow-sidebar",
        "transition-[width] duration-300 ease-in-out",
        // Collapsed rail keeps the 60px spec: 1px border + 11px padding on each
        // side leaves an exactly 36px column so the w-9 (36px) icon items center.
        collapsed
          ? "w-[60px] px-[11px] py-3"
          : fluid
            ? "w-full p-3 lg:w-[260px]"
            : "w-[260px] p-3",
        className,
      )}
    >
      {/* `overflow-y: auto` forces the x axis to clip too, and this box hugs
          its contents on every side — so the selected item's 1px ring, the
          profile's hover pill (which outsets 6px) and focus rings all landed
          outside it. Padding moves the clip edge out; the matching negative
          margin borrows that space back from the rail's own padding, leaving
          every child exactly where it was. */}
      <div
        className="-m-2 flex min-h-0 w-[calc(100%+16px)] flex-col gap-3 overflow-y-auto p-2 [scrollbar-width:none]"
      >
        {/* Workspace switcher / collapse control */}
        <div
          className={cx(
            "flex w-full transition-[gap] duration-300 ease-in-out",
            collapsed
              ? "flex-col-reverse items-start justify-center gap-2.5"
              : "flex-row items-center justify-between",
          )}
        >
          {/* The clip is here to hide the label as `max-width` animates shut,
              but it also cropped the trigger's hover pill down to four corner
              arcs. Same trick as the scroller: pad the clip box out by the
              pill's 8px reach and pull it back with a negative margin, so the
              widths below are 16px larger than the footprint they produce. */}
          <div
            className={cx(
              "-m-2 min-w-0 overflow-hidden p-2 transition-[max-width,opacity,transform] duration-300 ease-in-out",
              mobile && flat && searchActive
                ? "max-w-0 scale-95 opacity-0"
                : "max-w-[206px] scale-100 opacity-100",
            )}
          >
            <DashboardUserMenu
              label={switcher.label}
              initials={switcher.initials}
              color={switcher.color}
              menuLabel={switcher.menuLabel}
              collapsed={collapsed}
              suppressHover={suppressUserHover}
              onHoverSuppressionEnd={() => setSuppressUserHover(false)}
              avatarClassName={
                flat
                  ? "bg-background-tertiary-default dark:bg-background-secondary-default"
                  : undefined
              }
            >
              {switcher.menu}
            </DashboardUserMenu>
          </div>
          {mobile && flat ? (
            <div
              ref={searchFieldRef}
              className={cx(
                "flex h-9 items-center overflow-hidden rounded-full bg-background-tertiary-default transition-[width,box-shadow] duration-300 ease-in-out",
                searchActive
                  ? "w-full gap-2 pe-2.5 ps-2 ring-2 ring-inset ring-border-button-active"
                  : "w-9 gap-0 px-2",
              )}
            >
              <button
                ref={searchTriggerRef}
                type="button"
                aria-label="Search"
                onClick={activateSearch}
                className="flex size-5 shrink-0 cursor-pointer items-center justify-center text-foreground-icon-secondary"
              >
                <RiSearchLine className="size-5" aria-hidden />
              </button>
              <input
                ref={searchInputRef}
                type="search"
                aria-label="Filter navigation"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Escape") {
                    event.preventDefault();
                    deactivateSearch(true);
                  }
                }}
                placeholder="Search..."
                tabIndex={searchActive ? 0 : -1}
                className={cx(
                  "min-w-0 bg-transparent text-body-medium tracking-[-0.015em] text-text-primary outline-none placeholder:text-text-tertiary",
                  "transition-[width,opacity] duration-200 ease-in-out",
                  searchActive
                    ? "w-full flex-1 opacity-100 delay-100"
                    : "pointer-events-none w-0 flex-none opacity-0 delay-0",
                )}
              />
              <CloseButton
                size="2xs"
                aria-label="Clear navigation search"
                onClick={() => deactivateSearch(true)}
                className={cx(
                  "shrink-0 bg-background-tertiary-hover transition-opacity duration-150",
                  searchActive ? "opacity-100 delay-150" : "pointer-events-none opacity-0 delay-0",
                )}
              />
            </div>
          ) : mobile ? (
            <button
              type="button"
              aria-label="Close sidebar"
              onClick={onClose}
              className="cursor-pointer text-foreground-icon-secondary"
            >
              <RiCloseLine className="size-5" aria-hidden />
            </button>
          ) : (
            <button
              type="button"
              aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
              aria-expanded={!collapsed}
              onClick={() => {
                const isExpanding = collapsedState;
                if (!isExpanding) deactivateSearch(false);
                setCollapsed(!collapsedState);
                setSuppressUserHover(isExpanding);
              }}
              className={cx(
                "cursor-pointer text-foreground-icon-secondary transition-transform duration-300 ease-in-out",
                collapsed && "flex w-9 items-center justify-center",
              )}
            >
              <RiSideBarFill
                className={cx("size-5 transition-transform duration-300 ease-in-out", collapsed ? "rtl:-scale-x-100" : "-scale-x-100 rtl:scale-x-100")}
                aria-hidden
              />
            </button>
          )}
        </div>

        <div className="flex w-full flex-col gap-3">
          {/* Quick search */}
          {!flat && (searchActive && !collapsed ? (
            <div
              ref={searchFieldRef}
              className="flex w-full items-center gap-2 rounded-full bg-background-tertiary-default py-2 pe-2.5 ps-2 ring-2 ring-inset ring-border-button-active transition-[background-color,box-shadow] duration-[var(--input-transition-ms)] ease"
            >
              <RiSearchLine className="size-5 shrink-0 text-foreground-icon-secondary" aria-hidden />
              <input
                ref={searchInputRef}
                type="search"
                aria-label="Filter navigation"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Escape") {
                    event.preventDefault();
                    deactivateSearch(true);
                  }
                }}
                placeholder="Search navigation…"
                className="min-w-0 flex-1 bg-transparent text-body-medium text-text-primary outline-none placeholder:text-text-tertiary"
              />
              <CloseButton
                size="2xs"
                aria-label="Clear navigation search"
                onClick={() => deactivateSearch(true)}
                className="bg-background-tertiary-hover"
              />
            </div>
          ) : (
            <button
              ref={searchTriggerRef}
              type="button"
              aria-label="Quick Search"
              title={collapsed ? "Quick Search" : undefined}
              onClick={activateSearch}
              className={cx(
                "flex cursor-pointer items-center gap-2 p-2 hover:bg-background-tertiary-hover/55",
                "transition-[width,border-radius,background-color] duration-300 ease-in-out",
                collapsed
                  ? "w-9 rounded-full bg-background-tertiary-default"
                  : "w-full rounded-full bg-background-tertiary-default",
              )}
            >
              <span className={cx("flex min-w-0 items-center gap-2", !collapsed && "flex-1")}>
                <RiSearchLine className="size-5 shrink-0 text-foreground-icon-secondary" aria-hidden />
                <Collapsible collapsed={collapsed}>
                  <span className="text-body-medium whitespace-nowrap text-text-secondary">
                    Quick Search
                  </span>
                </Collapsible>
              </span>
              <Collapsible collapsed={collapsed}>
                <Kbd>⌘L</Kbd>
              </Collapsible>
            </button>
          ))}

          {/* Primary nav. The 2px inset is for the expanded rail only: the
              collapsed column is exactly as wide as a 36px item, so padding
              here pushes every item 2px right and the rail's own clip shaves
              that much off its selected fill and hover state. */}
          <nav className={cx("flex w-full flex-col gap-3", !collapsed && "px-0.5")}>
            <NavSections
              sections={sections}
              query={normalizedQuery}
              selected={selected}
              collapsed={collapsed}
              secondaryMatch={secondaryMatch}
            />
          </nav>
        </div>
      </div>

      <div className="flex w-full shrink-0 flex-col gap-3">
        {footer && !collapsed ? footer : null}
        {showThemeToggle &&
          (collapsed ? (
            <ThemeToggle collapsed />
          ) : (
            <ThemeToggle
              appearance="sidebar-segmented"
              className={flat ? "!bg-background-secondary-default" : undefined}
            />
          ))}
        {/* Secondary nav */}
        <nav className="flex w-full flex-col gap-1">
          <NavRows
            items={secondary.filter((item) => matches(item.label))}
            selected={selected}
            collapsed={collapsed}
          />
        </nav>

        {/* Account card → opens the account menu next to the sidebar */}
        <DashboardTeamMenu
          {...account}
          showThemeToggle={!showThemeToggle}
          collapsed={collapsed}
          className={flat ? "!bg-background-secondary-default" : undefined}
        />
      </div>
    </aside>
    </DensityContext.Provider>
  ));
}
