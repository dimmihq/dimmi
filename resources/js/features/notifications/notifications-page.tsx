/*
 * The member's notifications on BoardUI's notification center: newest
 * change first, by tab, unread marked. Opening the page clears the
 * sidebar badge; opening a notification marks it read and goes to what
 * it is about. An action waiting for approval can be approved or
 * declined right here; once someone decides, the buttons give way to
 * how it ended. Live updates arrive without a refresh.
 */
import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";

import { NotificationCenter } from "@/components/application/notification-center/notification-center";
import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { decideActionCall } from "@/lib/api/ai-actions";
import { markNotifications, type NotificationFilter, notificationsQuery } from "@/lib/api/notifications";
import { realtimeKeys } from "@/lib/realtime/keys";

import { useAccount } from "../settings/use-account";
import { centerItem, pathOf } from "./notification-format";

const tabs: { id: NotificationFilter; label: string }[] = [
  { id: "all", label: "All" },
  { id: "unread", label: "Unread" },
  { id: "mentioned", label: "Mentions" },
  { id: "ai_action_approval_requested", label: "Approvals" },
];

export function NotificationsPage() {
  const workspace = useAccount().workspace.id;
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [filter, setFilter] = useState<NotificationFilter>("all");
  const [busy, setBusy] = useState<{ id: string; action: string } | null>(null);
  const list = useInfiniteQuery({ ...notificationsQuery(workspace, filter), enabled: workspace !== "" });
  const refresh = () => queryClient.invalidateQueries({ queryKey: realtimeKeys.notifications(workspace) });
  const mark = useMutation({
    mutationFn: (change: Parameters<typeof markNotifications>[1]) => markNotifications(workspace, change),
    onSettled: refresh,
  });
  const notifications = useMemo(() => list.data?.pages.flatMap((page) => page.data) ?? [], [list.data]);
  const meta = list.data?.pages[0]?.meta;

  useEffect(() => {
    if (meta && meta.unseen_count > 0) {
      mark.mutate({ seen: true });
    }
  }, [meta, mark.mutate]);

  const decide = async (id: string, action: string) => {
    const n = notifications.find((item) => item.id === id);
    if (!n?.action_call_id) {
      return;
    }
    setBusy({ id, action });
    try {
      await decideActionCall(workspace, n.action_call_id, { status: action === "approve" ? "approved" : "declined" });
      toast.success(action === "approve" ? "Approved" : "Declined");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "That did not work. Try again.");
    } finally {
      setBusy(null);
      void refresh();
    }
  };

  const open = (id: string) => {
    const n = notifications.find((item) => item.id === id);
    if (!n) {
      return;
    }
    if (!n.read) {
      mark.mutate({ ids: [id], read: true });
    }
    void navigate({ href: pathOf(n.link) });
  };

  const now = new Date();
  const items = notifications.map((n) => centerItem(n, now, busy?.id === n.id ? busy.action : undefined));

  return (
    <div className="flex min-h-0 flex-1 justify-center">
      <NotificationCenter
        className="h-full max-w-3xl"
        notifications={items}
        tab={filter}
        onTabChange={(next) => setFilter(next as NotificationFilter)}
        tabs={tabs.map((t) => ({ ...t, count: t.id === "unread" ? (meta?.unread_count ?? 0) : undefined }))}
        unreadCount={meta?.unread_count ?? 0}
        onMarkAllRead={() => mark.mutate({ read: true })}
        onOpen={open}
        onAction={(id, action) => void decide(id, action)}
        emptyMessage={list.isPending ? "Loading…" : "You’re all caught up."}
        footer={
          list.hasNextPage ? (
            <div className="flex justify-center pt-2">
              <Button
                variant="secondary"
                size="small"
                loading={list.isFetchingNextPage}
                onClick={() => void list.fetchNextPage()}
              >
                Load more
              </Button>
            </div>
          ) : null
        }
      />
    </div>
  );
}
