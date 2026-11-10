/*
 * A teammate's own Slack direct messages as a place notifications reach
 * them. dimmi finds them in the workspace's Slack by their verified
 * email, so nothing has to be linked by hand. Which notifications go
 * there is the Slack column above. Slack stopping the messages shows
 * why, with a way to turn them back on. The section shows only while
 * Slack is connected to the workspace.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { integrationsQuery } from "@/lib/api/integrations";
import {
  createDirectDestination,
  deleteDestination,
  destinationsQuery,
  type NotificationDestination,
} from "@/lib/api/notification-destinations";

import { Action } from "../controls";
import { stopped } from "../destinations";
import { Row, Section } from "../layout";
import { useDestinationActions } from "../use-destination-actions";

/*
 * DirectRow is the teammate's direct messages once they are on.
 */
function DirectRow({ workspace, direct }: { workspace: string; direct: NotificationDestination }) {
  const { test, enable, refresh } = useDestinationActions(workspace);
  const [leaving, setLeaving] = useState(false);
  const reason = stopped(direct);

  const remove = async () => {
    try {
      await deleteDestination(workspace, direct.id);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Something went wrong. Try again.");
      throw error;
    }

    await refresh();
  };

  return (
    <Row
      label="Slack direct messages"
      description={
        reason ? (
          <span className="text-text-error-primary">{reason}</span>
        ) : (
          "Notifications with Slack turned on above arrive as direct messages from the app."
        )
      }
    >
      {reason ? (
        <Button variant="primary" size="small" loading={enable.isPending} onClick={() => enable.mutate(direct)}>
          Turn back on
        </Button>
      ) : null}
      <Action loading={test.isPending} onPress={() => test.mutate(direct)}>
        Send a test
      </Action>
      <Action onPress={() => setLeaving(true)}>Stop</Action>
      <AlertDialog
        isOpen={leaving}
        onOpenChange={setLeaving}
        title="Stop Slack direct messages?"
        description="Notifications stop reaching you in Slack. You can turn them on again any time."
        confirmLabel="Stop"
        onConfirm={remove}
      />
    </Row>
  );
}

export function SlackDirectSection({ workspace }: { workspace: string }) {
  const queryClient = useQueryClient();
  const { data: installations = [] } = useQuery(integrationsQuery(workspace));
  const { data: destinations = [], isPending } = useQuery(destinationsQuery(workspace));
  const slack = installations.find((i) => i.provider === "slack" && i.status === "active");
  const direct = destinations.find((d) => d.type === "slack_dm");

  const start = useMutation({
    mutationFn: () => createDirectDestination(workspace, slack?.id ?? ""),
    onSuccess: () => toast.success("Slack direct messages are on."),
    onError: (error) => toast.error(error.message),
    onSettled: () => queryClient.invalidateQueries({ queryKey: destinationsQuery(workspace).queryKey }),
  });

  if (!slack) {
    return null;
  }

  return (
    <Section
      title="Slack"
      description="Get notifications as direct messages in your team's Slack."
      loading={isPending}
      rows={1}
    >
      {direct ? (
        <DirectRow workspace={workspace} direct={direct} />
      ) : (
        <Row label="Send to my Slack DMs" description="dimmi finds you in Slack by your verified email address.">
          <Action loading={start.isPending} onPress={() => start.mutate()}>
            Send to my Slack DMs
          </Action>
        </Row>
      )}
    </Section>
  );
}
