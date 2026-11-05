/*
 * Picking the icon a home link shows: a small button with the icon chosen
 * now, opening a grid of every icon the messenger can draw. Each icon is
 * named for screen readers, and picking one closes the grid.
 */
import {
  RiBookOpenLine,
  RiCalendarLine,
  RiChat3Line,
  RiLink,
  RiMailLine,
  RiPhoneLine,
  RiQuestionLine,
  RiVideoOnLine,
} from "@remixicon/react";
import { Popover, PopoverTrigger } from "@/components/base/popover/popover";
import { ChevronDownSmall } from "@/components/foundations/icons/chevrons";
import type { HomeLink } from "@/lib/api/widget-settings";
import { cx } from "@/utils/cx";

const icons: { id: HomeLink["icon"]; name: string; icon: typeof RiLink }[] = [
  { id: "link", name: "Link", icon: RiLink },
  { id: "calendar", name: "Calendar", icon: RiCalendarLine },
  { id: "book", name: "Book", icon: RiBookOpenLine },
  { id: "mail", name: "Mail", icon: RiMailLine },
  { id: "phone", name: "Phone", icon: RiPhoneLine },
  { id: "chat", name: "Chat", icon: RiChat3Line },
  { id: "video", name: "Video", icon: RiVideoOnLine },
  { id: "help", name: "Help", icon: RiQuestionLine },
];

export function IconPick({
  label,
  value,
  disabled = false,
  onChange,
}: {
  label: string;
  value: HomeLink["icon"];
  disabled?: boolean;
  onChange: (icon: HomeLink["icon"]) => void;
}) {
  const current = icons.find((icon) => icon.id === value) ?? icons[0];
  const Current = current?.icon ?? RiLink;

  return (
    <PopoverTrigger>
      <button
        type="button"
        aria-label={`${label}: ${current?.name ?? "Link"}`}
        disabled={disabled}
        className={cx(
          "flex h-8 shrink-0 cursor-pointer items-center gap-1 rounded-2lg border border-border-button-default",
          "bg-background-primary-default px-[7px] text-foreground-icon-primary shadow-xs",
          "hover:border-border-button-hover hover:bg-background-primary-hover",
          "outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring focus-visible:ring-offset-2",
          "disabled:cursor-not-allowed disabled:bg-background-primary-disabled disabled:shadow-none",
        )}
      >
        <Current className="size-4" />
        <ChevronDownSmall className="size-3.5 text-text-secondary" />
      </button>
      <Popover aria-label={label} placement="bottom start" className="w-auto" dialogClassName="p-1">
        {({ close }) => (
          <div className="grid grid-cols-4 gap-1">
            {icons.map(({ id, name, icon: Icon }) => (
              <button
                key={id}
                type="button"
                aria-label={name}
                aria-pressed={id === value}
                onClick={() => {
                  onChange(id);
                  close();
                }}
                className={cx(
                  "flex size-9 cursor-pointer items-center justify-center rounded-xl text-foreground-icon-secondary",
                  "outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
                  id === value && "bg-background-secondary-default text-foreground-icon-primary",
                )}
              >
                <Icon className="size-[18px]" />
              </button>
            ))}
          </div>
        )}
      </Popover>
    </PopoverTrigger>
  );
}
