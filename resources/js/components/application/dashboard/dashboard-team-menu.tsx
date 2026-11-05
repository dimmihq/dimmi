"use client";

import { useTemplateCopy } from "@/components/foundations/template-copy/template-copy";

import { useDirection } from "@/components/foundations/direction/direction";
import { useState, useSyncExternalStore, type ComponentType, type ReactNode } from "react";
import { RiPaletteLine } from "@remixicon/react";
import {
  Button as AriaButton,
  Dialog as AriaDialog,
  DialogTrigger as AriaDialogTrigger,
  Popover as AriaPopover,
} from "react-aria-components";
import { ThemeToggle } from "@/components/application/theme/theme-toggle";
import { Avatar } from "@/components/base/avatar/avatar";
import { Badge } from "@/components/base/badges/badge";
import { ChevronDownSmall } from "@/components/foundations/icons/chevrons";
import { cx } from "@/utils/cx";

/**
 * Figma source: Board UI → sidebar profile menu (node 3823:3699).
 *
 * Dropdown opened from the card at the foot of the sidebar (Dimmi: the
 * signed-in user). Floats to the right of the sidebar: white panel, 1px
 * border/button/default, radius 16, p 10, shadow/dropdown. Header (avatar +
 * name/email), the host's grouped sections of sidebar-style rows separated by
 * full-bleed dividers, and a product name + version footer. Rows mirror the sidebar nav item pattern (icon + label + optional
 * counter) but use the menu tokens from Figma (text/primary label,
 * background/primary/hover on hover + the selected row).
 */

type IconComponent = ComponentType<{
  className?: string;
  "aria-hidden"?: boolean | "true" | "false";
}>;

export type MenuRow = {
  icon: IconComponent;
  label: string;
  badge?: string;
  isSelected?: boolean;
  /** Runs when the row is chosen; the menu closes after it. */
  onSelect: () => void;
};

export type MenuGroup = {
  id: string;
  label?: string;
  items: MenuRow[];
};

/** Label/chevron slot on the trigger: blurs + fades away as the rail collapses. */
function Collapsible({ collapsed, children }: { collapsed: boolean; children: ReactNode }) {
  const localize = useTemplateCopy();
  return localize((
    <span
      className={cx(
        "flex min-w-0 items-center overflow-hidden transition-[max-width,opacity,filter] duration-300 ease-in-out",
        collapsed ? "max-w-0 opacity-0 blur-[3px]" : "max-w-40 opacity-100 blur-0",
      )}
    >
      {children}
    </span>
  ));
}

function TeamMenuItem({ icon: Icon, label, badge, isSelected, onSelect }: MenuRow) {
  const localize = useTemplateCopy();
  return localize((
    <AriaButton
      type="button"
      aria-current={isSelected ? "page" : undefined}
      onPress={onSelect}
      className={cx(
        "flex w-full cursor-pointer items-center gap-2.5 rounded-2lg p-2 text-start outline-none transition-colors",
        isSelected
          ? "bg-background-primary-hover"
          : "hover:bg-background-primary-hover focus-visible:bg-background-primary-hover",
      )}
    >
      <span className="flex min-w-0 flex-1 items-center gap-2">
        <Icon className="size-5 shrink-0 text-foreground-icon-secondary" aria-hidden />
        <span className="truncate text-body-medium text-text-primary">{label}</span>
      </span>
      {badge && (
        <Badge
          color="neutral"
          className="bg-team-menu-count-background text-team-menu-count-foreground"
        >
          {badge}
        </Badge>
      )}
    </AriaButton>
  ));
}

export type PresenceStatus = "online" | "away";

/** Avatar with a dot at its corner saying whether the person takes conversations. */
function StatusAvatar({ status, ...props }: Parameters<typeof Avatar>[0] & { status?: PresenceStatus }) {
  return (
    <span className="relative inline-flex shrink-0">
      <Avatar {...props} />
      {status && (
        <span
          data-status={status}
          aria-label={status === "online" ? "Active" : "Away"}
          role="img"
          className={cx(
            "absolute -end-0.5 -bottom-0.5 size-2.5 rounded-full ring-2 ring-background-primary-default",
            status === "online" ? "bg-notification-success-foreground" : "bg-foreground-icon-quaternary",
          )}
        />
      )}
    </span>
  );
}

const mobileSnapshot = () => window.matchMedia("(max-width: 639px)").matches;
const subscribeMobile = (listener: () => void) => {
  const query = window.matchMedia("(max-width: 639px)");
  query.addEventListener("change", listener);
  return () => query.removeEventListener("change", listener);
};

export function DashboardTeamMenu({
  collapsed = false,
  className,
  showThemeToggle = false,
  name,
  detail,
  initials,
  src,
  groups,
  product,
  version,
  status,
}: {
  collapsed?: boolean;
  className?: string;
  /** Show appearance controls inside the menu instead of the sidebar. */
  showThemeToggle?: boolean;
  /** Who or what the card stands for, e.g. the signed-in user. */
  name: string;
  /** The line under the name, e.g. the email. */
  detail: string;
  initials: string;
  src?: string;
  groups: MenuGroup[];
  /** Footer: the product's name and the version it runs. */
  product: string;
  version?: string;
  /** Dot on the avatar: whether the person takes conversations. */
  status?: PresenceStatus;
}) {
  const localize = useTemplateCopy();
  const direction = useDirection();
  const [isOpen, setIsOpen] = useState(false);
  // "right" placement assumes room to the sidebar's right (true in-flow on
  // desktop) — on mobile the sidebar can span the full viewport, so the
  // 265px panel would render off-screen. Below sm, drop into a plain
  // dropdown under the trigger instead.
  const isMobile = useSyncExternalStore(subscribeMobile, mobileSnapshot, () => false);

  return localize((
    <AriaDialogTrigger isOpen={isOpen} onOpenChange={setIsOpen}>
      <AriaButton
        aria-label={name}
        className={cx(
          "flex cursor-pointer items-center overflow-hidden outline-none",
          "border-2 border-transparent hover:border-border-button-hover",
          "transition-[width,background-color,border-color,padding] duration-300 ease-in-out",
          "focus-visible:ring-2 focus-visible:ring-border-focus-ring focus-visible:ring-offset-2",
          collapsed
            ? "size-9 justify-start rounded-full bg-transparent p-0"
            : "w-full justify-between rounded-xl bg-background-tertiary-default py-2 pe-4 ps-2.5",
          className,
        )}
      >
        <span className="flex min-w-0 items-center gap-2">
          <StatusAvatar size="md" color="blue" src={src} initials={initials} alt={name} status={status} />
          <Collapsible collapsed={collapsed}>
            <span className="flex min-w-0 flex-col items-start justify-center">
              <span className="max-w-40 truncate text-body-medium whitespace-nowrap text-text-primary">{name}</span>
              <span className="max-w-40 truncate text-body-regular whitespace-nowrap text-text-secondary">{detail}</span>
            </span>
          </Collapsible>
        </span>
        <Collapsible collapsed={collapsed}>
          <span className="flex size-4 shrink-0 items-center justify-center rounded-[3px] bg-background-tertiary-hover">
            <ChevronDownSmall
              className={cx("size-4 text-text-secondary transition-transform duration-200 ease", isOpen && "rotate-180")}
            />
          </span>
        </Collapsible>
      </AriaButton>

      <AriaPopover
        placement={isMobile ? "bottom start" : direction === "rtl" ? "left bottom" : "right bottom"}
        offset={8}
        className={cx(
          "w-[265px] max-w-[calc(100vw-32px)] origin-bottom-left overflow-y-auto",
          "rounded-2xl border border-border-button-default bg-background-primary-default p-2.5 shadow-dropdown",
          "transition duration-150 ease-out",
          "data-[entering]:opacity-0 data-[entering]:scale-95 data-[entering]:blur-[2px]",
          "data-[exiting]:opacity-0 data-[exiting]:scale-95 data-[exiting]:blur-[2px]",
        )}
      >
        <AriaDialog aria-label={`${name} menu`} className="flex flex-col gap-[7px] outline-none">
          {/* Header */}
          <div className="flex w-full items-center gap-2 px-2 pt-1">
            <StatusAvatar size="md" color="blue" src={src} initials={initials} alt={name} status={status} />
            <div className="flex min-w-0 flex-col items-start justify-center">
              <span className="max-w-[200px] truncate text-body-medium whitespace-nowrap text-text-primary">{name}</span>
              <span className="max-w-[200px] truncate text-body-regular whitespace-nowrap text-text-secondary">{detail}</span>
            </div>
          </div>

          {/* Grouped sidebar-style rows */}
          <div className="flex w-full flex-col">
            {groups.map((group, index) => (
              <Group key={group.id} group={group} showDivider={index > 0} onSelect={() => setIsOpen(false)}>
                {showThemeToggle && group.id === "personal" && (
                  <div className="flex w-full items-center justify-between gap-2.5 rounded-2lg px-2 py-1">
                    <span className="flex min-w-0 flex-1 items-center gap-2">
                      <RiPaletteLine className="size-5 shrink-0 text-foreground-icon-secondary" aria-hidden />
                      <span className="truncate text-body-medium text-text-primary">Appearance</span>
                    </span>
                    <ThemeToggle appearance="segmented" size="small" className="shrink-0" />
                  </div>
                )}
              </Group>
            ))}
          </div>

          {/* Footer */}
          <div className="flex w-full items-center justify-between px-2 pt-1 pb-2">
            <span className="text-body-2-medium whitespace-nowrap text-text-tertiary">{product}</span>
            {version ? (
              <span className="inline-flex items-center justify-center rounded-sm bg-background-tertiary-default px-1 py-px text-body-2-medium whitespace-nowrap text-text-tertiary">
                {version}
              </span>
            ) : null}
          </div>
        </AriaDialog>
      </AriaPopover>
    </AriaDialogTrigger>
  ));
}

function Group({ group, showDivider, onSelect, children }: { group: MenuGroup; showDivider: boolean; onSelect: () => void; children?: ReactNode }) {
  const localize = useTemplateCopy();
  return localize((
    <>
      {showDivider && <div className="-mx-2.5 my-2.5 h-px bg-border-button-default" />}
      <div className={cx("flex w-full flex-col gap-1", group.label && "gap-1.5 pt-1")}>
        {group.label && (
          <span className="px-2 text-body-medium text-text-secondary">{group.label}</span>
        )}
        <div className="flex w-full flex-col gap-1">
          {group.items.map((item) => (
            <TeamMenuItem
              key={item.label}
              {...item}
              onSelect={() => {
                item.onSelect();
                onSelect();
              }}
            />
          ))}
          {children}
        </div>
      </div>
    </>
  ));
}
