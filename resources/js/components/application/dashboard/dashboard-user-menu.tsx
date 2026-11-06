"use client";

import { useTemplateCopy } from "@/components/foundations/template-copy/template-copy";

import { useDirection } from "@/components/foundations/direction/direction";
import { useEffect, useState, type ComponentType, type ReactNode } from "react";
import { RiCheckLine } from "@remixicon/react";
import {
  Button as AriaButton,
  Dialog as AriaDialog,
  DialogTrigger as AriaDialogTrigger,
  Popover as AriaPopover,
} from "react-aria-components";
import { Avatar } from "@/components/base/avatar/avatar";
import { ChevronUpDownSmall } from "@/components/foundations/icons/chevrons";
import { cx } from "@/utils/cx";

/**
 * Figma source: Board UI → sidebar account menu (node 3828:3895).
 *
 * Dropdown opened from the sidebar's workspace switcher (avatar + name +
 * chevron) at the top. Floats to the right of the sidebar with the same panel
 * surface and appear animation as the team menu: white, 1px border, radius 16,
 * p 10, shadow/dropdown. The trigger shows whatever the host passes (Dimmi:
 * the current workspace) and the panel renders the host's own content
 * through `children`, which gets a `close` callback.
 */

export type AvatarColor = "neutral" | "blue" | "lime" | "pink";

export type UserRow = {
  initials: string;
  color: AvatarColor;
  name: string;
  /** Second line under the name, e.g. the person's role in the workspace. */
  detail?: string;
  isSelected?: boolean;
};

/** Label slot on the trigger: blurs + fades away as the rail collapses. */
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

export function UserMenuItem({ initials, color, name, detail, isSelected, onSelect }: UserRow & { onSelect: () => void }) {
  const localize = useTemplateCopy();
  return localize((
    <AriaButton
      type="button"
      aria-current={isSelected ? "page" : undefined}
      onPress={onSelect}
      className={cx(
        "flex w-full cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-start outline-none transition-colors",
        "hover:bg-background-primary-hover focus-visible:bg-background-primary-hover",
        isSelected && "bg-background-primary-hover",
      )}
    >
      <Avatar size="xs" color={color} initials={initials} />
      <span className="min-w-0 truncate text-body-2-medium text-text-primary">{name}</span>
      {isSelected ? <RiCheckLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" /> : null}
      {detail ? (
        <span className="ms-auto shrink-0 rounded-full bg-background-tertiary-default px-1.5 py-px text-caption-1-medium text-text-tertiary">
          {detail}
        </span>
      ) : null}
    </AriaButton>
  ));
}

/** The panel's header: who or what the menu is about, larger than the rows below it. */
export function MenuHeader({ initials, color, name, detail }: Omit<UserRow, "isSelected">) {
  const localize = useTemplateCopy();
  return localize((
    <div className="flex items-center gap-2 px-2 py-1.5">
      <Avatar size="xs" color={color} initials={initials} />
      <span className="min-w-0 flex-1 truncate text-body-2-medium text-text-primary">{name}</span>
      {detail ? (
        <span className="shrink-0 rounded-full bg-background-tertiary-default px-1.5 py-px text-caption-1-medium text-text-tertiary">
          {detail}
        </span>
      ) : null}
    </div>
  ));
}

/** An action at the foot of the panel: an icon and a label, like the sidebar's rows. */
export function MenuActionRow({
  icon: Icon,
  label,
  onSelect,
}: {
  icon: ComponentType<{ className?: string; "aria-hidden"?: boolean | "true" | "false" }>;
  label: string;
  onSelect: () => void;
}) {
  const localize = useTemplateCopy();
  return localize((
    <AriaButton
      type="button"
      onPress={onSelect}
      className={cx(
        "flex w-full cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-start outline-none transition-colors",
        "hover:bg-background-primary-hover focus-visible:bg-background-primary-hover",
      )}
    >
      <Icon className="size-4 shrink-0 text-foreground-icon-secondary" aria-hidden />
      <span className="truncate text-body-2-medium text-text-primary">{label}</span>
    </AriaButton>
  ));
}

/** A titled list of rows for the panel, e.g. the workspaces a user can switch to. */
export function MenuRowList({
  title,
  rows,
  onSelect,
}: {
  title: string;
  rows: (UserRow & { key: string })[];
  onSelect: (key: string) => void;
}) {
  const localize = useTemplateCopy();
  return localize((
    <div className="flex w-full flex-col gap-0.5">
      <span className="px-2 pt-1 pb-0.5 text-caption-1-medium text-text-tertiary">{title}</span>
      <div className="flex w-full flex-col gap-0.5">
        {rows.map(({ key, ...row }) => (
          <UserMenuItem key={key} {...row} onSelect={() => onSelect(key)} />
        ))}
      </div>
    </div>
  ));
}

/** Full-bleed divider centered in the panel, between a list and its actions. */
export function MenuDivider() {
  return <div className="-mx-1.5 my-1.5 h-px bg-border-button-default" />;
}

export function DashboardUserMenu({
  collapsed = false,
  suppressHover = false,
  onHoverSuppressionEnd,
  avatarClassName,
  label,
  initials,
  color = "neutral",
  menuLabel,
  children,
}: {
  collapsed?: boolean;
  /** Text on the trigger, e.g. the workspace name. */
  label: string;
  initials: string;
  color?: AvatarColor;
  /** Accessible name of the panel. */
  menuLabel: string;
  /** The panel's contents; call `close` after acting on a row. */
  children: (close: () => void) => ReactNode;
  /** Prevents expansion from creating a hover state under a stationary pointer. */
  suppressHover?: boolean;
  /** Re-arms hover after the pointer fully leaves the trigger. */
  onHoverSuppressionEnd?: () => void;
  avatarClassName?: string;
}) {
  const localize = useTemplateCopy();
  const direction = useDirection();
  const [isOpen, setIsOpen] = useState(false);
  // "right" placement assumes room to the sidebar's right (true in-flow on
  // desktop) — on mobile the sidebar can span the full viewport, so the
  // 265px panel would render off-screen. Below sm, drop into a plain
  // dropdown under the trigger instead.
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const mq = window.matchMedia("(max-width: 639px)");
    setIsMobile(mq.matches);
    const onChange = (e: MediaQueryListEvent) => setIsMobile(e.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  return localize((
    <AriaDialogTrigger isOpen={isOpen} onOpenChange={setIsOpen}>
      <AriaButton
        aria-label={label}
        onPointerLeave={() => {
          if (suppressHover) onHoverSuppressionEnd?.();
        }}
        className={cx(
          "relative flex min-w-0 cursor-pointer items-center gap-2 rounded-full outline-none",
          "focus-visible:ring-2 focus-visible:ring-border-focus-ring focus-visible:ring-offset-2",
          // Hover pill (Figma node 3829:4063): a fully-rounded 2px border/button/hover
          // outline drawn via a pseudo-element so it never shifts the layout.
          "before:pointer-events-none before:absolute before:-inset-x-1.5 before:-inset-y-[5px] before:rounded-full before:border-2 before:border-transparent before:transition-colors before:duration-150",
          !suppressHover && "hover:before:border-border-sidebar-profile-hover",
          // Collapsed, the trigger takes the rail's own 36px column and centres
          // the 32px avatar in it, instead of sizing to the avatar plus a gap
          // held open for a label that's shrunk to nothing. That gap made the
          // button 42px wide in a 36px rail, which pushed its hover pill
          // off-centre and into the rail's clip.
          //
          // The pill's insets go square too: 36×32 plus the expanded 6/5 reach
          // is a 48×42 stadium, not the circle the avatar wants. 3/5 lands it
          // on 42×42.
          collapsed && "w-9 justify-center gap-0 before:-inset-x-[3px]",
        )}
      >
        <Avatar size="md" color={color} initials={initials} className={avatarClassName} />
        <Collapsible collapsed={collapsed}>
          <span className="flex items-center gap-0.5">
            <span className="text-body-medium whitespace-nowrap text-text-primary">{label}</span>
            <ChevronUpDownSmall className="size-4 shrink-0 text-foreground-icon-tertiary" />
          </span>
        </Collapsible>
      </AriaButton>

      <AriaPopover
        placement={isMobile ? "bottom start" : direction === "rtl" ? "left top" : "right top"}
        offset={8}
        className={cx(
          "w-60 max-w-[calc(100vw-32px)] origin-top-left overflow-y-auto",
          "rounded-2xl border border-border-button-default bg-background-primary-default p-1.5 shadow-dropdown",
          "transition duration-150 ease-out",
          "data-[entering]:opacity-0 data-[entering]:scale-95 data-[entering]:blur-[2px]",
          "data-[exiting]:opacity-0 data-[exiting]:scale-95 data-[exiting]:blur-[2px]",
        )}
      >
        <AriaDialog aria-label={menuLabel} className="flex flex-col outline-none">
          {children(() => setIsOpen(false))}
        </AriaDialog>
      </AriaPopover>
    </AriaDialogTrigger>
  ));
}
