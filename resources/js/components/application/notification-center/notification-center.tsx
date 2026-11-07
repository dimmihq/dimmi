"use client";

import { useTemplateCopy } from "@/components/foundations/template-copy/template-copy";

import type { ComponentType, HTMLAttributes, ReactNode, Ref } from "react";
import { useMemo, useState } from "react";
import {
  RiCheckboxCircleFill,
  RiErrorWarningFill,
  RiInformationFill,
  RiNotification3Fill,
  RiNotificationOffLine,
} from "@remixicon/react";
import { Avatar, type AvatarProps } from "@/components/base/avatar/avatar";
import { Button, type ButtonProps } from "@/components/base/buttons/button";
import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/base/segmented-control/segmented-control";
import { cx, sortCx } from "@/utils/cx";

export type NotificationCenterTab = string;
export type NotificationCenterCategory = string;
export type NotificationCenterStatus = "neutral" | "information" | "success" | "error";

type IconComponent = ComponentType<{
  className?: string;
  "aria-hidden"?: boolean | "true" | "false";
}>;

export type NotificationCenterAction = {
  id: string;
  label: string;
  variant?: ButtonProps["variant"];
  loading?: boolean;
};

export type NotificationCenterItem = {
  id: string;
  category: NotificationCenterCategory;
  group: string;
  title: string;
  description: string;
  timestamp: string;
  unread?: boolean;
  status?: NotificationCenterStatus;
  icon?: IconComponent;
  avatar?: Pick<AvatarProps, "src" | "alt" | "initials" | "color">;
  actions?: NotificationCenterAction[];
  note?: string;
};

export type NotificationCenterTabSpec = { id: NotificationCenterTab; label: string; count?: number };

export interface NotificationCenterProps
  extends Omit<HTMLAttributes<HTMLDivElement>, "onChange"> {
  notifications: NotificationCenterItem[];
  defaultTab?: NotificationCenterTab;
  tab?: NotificationCenterTab;
  onTabChange?: (tab: NotificationCenterTab) => void;
  onAction?: (notificationId: string, actionId: string) => void;
  onOpen?: (notificationId: string) => void;
  onMarkAllRead?: () => void;
  tabs?: NotificationCenterTabSpec[];
  unreadCount?: number;
  footer?: ReactNode;
  title?: string;
  emptyMessage?: string;
  ref?: Ref<HTMLDivElement>;
}

const STATUS_ICON: Record<NotificationCenterStatus, IconComponent> = {
  neutral: RiNotification3Fill,
  information: RiInformationFill,
  success: RiCheckboxCircleFill,
  error: RiErrorWarningFill,
};

const styles = sortCx({
  status: {
    neutral: "bg-background-tertiary-default text-text-secondary",
    information:
      "bg-notification-information-background text-notification-information-foreground",
    success: "bg-notification-success-background text-notification-success-foreground",
    error: "bg-notification-error-background text-notification-error-foreground",
  },
});

const TABS: NotificationCenterTabSpec[] = [
  { id: "all", label: "All" },
  { id: "mentions", label: "Mentions" },
  { id: "system", label: "System" },
];

function NotificationVisual({ item }: { item: NotificationCenterItem }) {
  const localize = useTemplateCopy();
  if (item.avatar) {
    return localize(<Avatar {...item.avatar} size="lg" className="size-10" />);
  }

  const status = item.status ?? "neutral";
  const Icon = item.icon ?? STATUS_ICON[status];

  return localize((
    <span
      className={cx(
        "flex size-10 shrink-0 items-center justify-center rounded-full",
        styles.status[status],
      )}
    >
      <Icon className="size-5" aria-hidden />
    </span>
  ));
}

function TabLabel({
  label,
  count,
}: {
  label: string;
  count: number;
}) {
  const localize = useTemplateCopy();
  return localize((
    <span className="inline-flex items-center gap-1.5">
      {label}
      <span className="inline-flex min-w-5 items-center justify-center rounded-sm bg-badge-neutral-background px-1 py-px text-caption-1-medium text-text-secondary">
        {count}
      </span>
    </span>
  ));
}

export function NotificationCenter({
  notifications,
  defaultTab = "all",
  tab,
  onTabChange,
  onAction,
  onOpen,
  onMarkAllRead,
  tabs = TABS,
  unreadCount: unreadTotal,
  footer,
  title = "Notifications",
  emptyMessage = "You’re all caught up.",
  className,
  ref,
  ...props
}: NotificationCenterProps) {
  const localize = useTemplateCopy();
  const [internalTab, setInternalTab] = useState<NotificationCenterTab>(defaultTab);
  const [readIds, setReadIds] = useState<Set<string>>(() => new Set());
  const activeTab = tab ?? internalTab;
  const controlled = onMarkAllRead !== undefined;

  const setTab = (nextTab: NotificationCenterTab) => {
    if (tab === undefined) setInternalTab(nextTab);
    onTabChange?.(nextTab);
  };

  const isUnread = (item: NotificationCenterItem) =>
    item.unread === true && (controlled || !readIds.has(item.id));
  const unreadCount = unreadTotal ?? notifications.filter(isUnread).length;

  const tabCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const spec of tabs) {
      counts[spec.id] =
        spec.count ??
        (spec.id === "all"
          ? notifications.length
          : notifications.filter((item) => item.category === spec.id).length);
    }
    return counts;
  }, [notifications, tabs]);

  const visibleNotifications = useMemo(
    () =>
      controlled || activeTab === "all"
        ? notifications
        : notifications.filter((item) => item.category === activeTab),
    [activeTab, notifications, controlled],
  );

  const markAllRead = () => {
    if (onMarkAllRead) {
      onMarkAllRead();
      return;
    }
    setReadIds(new Set(notifications.filter((item) => item.unread).map((item) => item.id)));
  };

  return localize((
    <section
      ref={ref}
      aria-label={title}
      className={cx(
        "flex w-full max-w-[430px] flex-col overflow-hidden rounded-3xl border border-border-button-default bg-notification-center-background shadow-dropdown",
        className,
      )}
      {...props}
    >
      <div className="flex flex-col gap-3 p-4 pb-1.5">
        <div className="flex items-start justify-between gap-4">
          <div className="flex min-w-0 flex-col gap-0.5">
            <h2 className="text-title-3-medium text-text-primary">{title}</h2>
            <p className="text-body-regular text-text-secondary">
              {unreadCount === 0 ? "No unread notifications" : `${unreadCount} unread`}
            </p>
          </div>
          <Button
            variant="ghost"
            size="small"
            onClick={markAllRead}
            disabled={unreadCount === 0}
          >
            Mark all read
          </Button>
        </div>

        <SegmentedControl
          aria-label="Notification category"
          selectedKeys={[activeTab]}
          onSelectionChange={(keys) => {
            const next = [...(keys as Set<string>)][0] as NotificationCenterTab | undefined;
            if (next) setTab(next);
          }}
          className="flex w-full"
        >
          {tabs.map(({ id, label }) => (
            <SegmentedControlItem
              key={id}
              id={id}
              className="flex-1"
            >
              <TabLabel label={label} count={tabCounts[id] ?? 0} />
            </SegmentedControlItem>
          ))}
        </SegmentedControl>
      </div>

      <div className="flex min-h-0 flex-1 flex-col bg-notification-center-background p-1.5">
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl bg-background-secondary-default">
          <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain bg-background-secondary-default p-2">
            {visibleNotifications.length === 0 ? (
              <div className="flex min-h-64 flex-col items-center justify-center gap-2 rounded-2xl bg-background-primary-default px-6 text-center">
                <span className="flex size-11 items-center justify-center rounded-full bg-background-secondary-default text-foreground-icon-secondary">
                  <RiNotificationOffLine className="size-5" aria-hidden />
                </span>
                <p className="text-body-medium text-text-primary">{emptyMessage}</p>
                <p className="max-w-64 text-body-regular text-text-secondary">
                  New activity will appear here when it arrives.
                </p>
              </div>
            ) : (
              <div className="flex flex-col gap-2">
                {visibleNotifications.map((item) => {
                  const unread = isUnread(item);
                  return (
                    <article
                      key={item.id}
                      className="group/item relative flex gap-3 rounded-notification-card bg-background-primary-default px-3 py-3"
                    >
                      <NotificationVisual item={item} />
                      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                        <div className="flex min-w-0 items-start justify-between gap-3">
                          {onOpen ? (
                            <button
                              type="button"
                              onClick={() => onOpen(item.id)}
                              className="min-w-0 text-start text-body-medium text-text-primary outline-none after:absolute after:inset-0 after:rounded-notification-card focus-visible:after:ring-2 focus-visible:after:ring-border-focus-ring"
                            >
                              {item.title}
                            </button>
                          ) : (
                            <p className="min-w-0 text-body-medium text-text-primary">{item.title}</p>
                          )}
                          <div className="flex shrink-0 items-center gap-2">
                            <span className="text-caption-1-medium whitespace-nowrap text-text-tertiary">
                              {item.timestamp}
                            </span>
                            {unread ? (
                              <span
                                className="size-2 rounded-full bg-accent-500"
                                aria-label="Unread"
                              />
                            ) : null}
                          </div>
                        </div>
                        {item.description ? (
                          <p className="line-clamp-2 text-body-regular text-text-secondary">{item.description}</p>
                        ) : null}
                        {item.note ? (
                          <p className="text-caption-1-medium text-text-tertiary">{item.note}</p>
                        ) : null}
                        {item.actions?.length ? (
                          <div className="relative z-10 mt-1.5 flex flex-wrap items-center gap-2">
                            {item.actions.map((action) => (
                              <Button
                                key={action.id}
                                size="small"
                                variant={action.variant ?? "secondary"}
                                loading={action.loading}
                                onClick={() => {
                                  if (!controlled) setReadIds((current) => new Set(current).add(item.id));
                                  onAction?.(item.id, action.id);
                                }}
                              >
                                {action.label}
                              </Button>
                            ))}
                          </div>
                        ) : null}
                      </div>
                    </article>
                  );
                })}
              </div>
            )}
            {footer}
          </div>
        </div>
      </div>
    </section>
  ));
}
