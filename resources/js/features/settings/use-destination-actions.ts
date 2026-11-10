/*
 * Testing a Slack destination and turning one back on after Slack
 * stopped it, each telling in a toast how it went, then refreshing the
 * list so its state shows.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { toast } from "@/components/base/toast/toast";
import {
  destinationsQuery,
  type NotificationDestination,
  testDestination,
  updateDestination,
} from "@/lib/api/notification-destinations";

/*
 * useDestinationActions tests a destination and turns it back on,
 * telling how each went.
 */
export function useDestinationActions(workspace: string) {
  const queryClient = useQueryClient();
  const refresh = () => queryClient.invalidateQueries({ queryKey: destinationsQuery(workspace).queryKey });

  const test = useMutation({
    mutationFn: (destination: NotificationDestination) => testDestination(workspace, destination.id),
    onSuccess: (result) =>
      result.status === "sent"
        ? toast.success("Test sent. Check Slack.")
        : toast.error(result.error ?? "Slack refused the test."),
    onError: (error) => toast.error(error.message),
    onSettled: refresh,
  });

  const enable = useMutation({
    mutationFn: (destination: NotificationDestination) =>
      updateDestination(workspace, destination.id, { enabled: true }),
    onSuccess: () => toast.success("Turned back on."),
    onError: (error) => toast.error(error.message),
    onSettled: refresh,
  });

  return { test, enable, refresh };
}
