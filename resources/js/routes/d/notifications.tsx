/*
 * The address emails and older links use for notifications. The
 * notifications live in a center beside the sidebar now, so this opens
 * it over the inbox.
 */
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";

import { useOpenNotifications } from "@/features/notifications/notification-center-context";

function OpenNotifications() {
  const open = useOpenNotifications();
  const navigate = useNavigate();
  useEffect(() => {
    open();
    void navigate({ to: "/d/inbox", replace: true });
  }, [open, navigate]);

  return null;
}

export const Route = createFileRoute("/d/notifications")({
  component: OpenNotifications,
});
