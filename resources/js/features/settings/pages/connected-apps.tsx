/*
 * The apps the person connected to dimmi through OAuth, such as AI
 * agents: which workspace each works in, what it may do and when it was
 * last used. Disconnecting one ends its access at once; it has to ask
 * again to come back.
 */
import { RiRobot2Line } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { AlertDialog } from "@/components/base/modal/modal";
import { type ConnectedApp, connectedAppsQuery, disconnectApp } from "@/lib/api/connected-apps";
import { formatDate } from "@/lib/i18n/format";

import { Action } from "../controls";
import { Empty, Glyph, Item, RowsSkeleton, Section, SettingsPage } from "../layout";
import { abilitySummary } from "../token-format";

/*
 * appDetail is the line under an app's name: its workspace, what it may
 * do and when it last acted.
 */
function appDetail(app: ConnectedApp): string {
  const scopes = app.scopes.filter((scope) => scope !== "offline_access");

  return [
    app.workspace_name,
    abilitySummary(scopes),
    `Connected ${formatDate(app.created_at)}`,
    `Last used ${formatDate(app.last_used_at)}`,
  ].join(" · ");
}

export function ConnectedAppsSettings() {
  const queryClient = useQueryClient();
  const { data: apps = [], isPending } = useQuery(connectedAppsQuery);
  const [leaving, setLeaving] = useState<ConnectedApp | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  const disconnect = async (id: string) => {
    setFailure(null);

    try {
      await disconnectApp(id);
    } catch (error) {
      setFailure(error instanceof Error ? error.message : "Something went wrong. Try again.");
    }

    await queryClient.invalidateQueries({ queryKey: connectedAppsQuery.queryKey });
  };

  return (
    <SettingsPage
      title="Connected apps"
      description="Apps and AI agents you let work in dimmi as you. None can do more than your role allows."
    >
      <Section title="Apps" description="Each one works in the workspace you chose when you connected it.">
        {isPending ? <RowsSkeleton rows={2} /> : null}
        {!isPending && apps.length === 0 ? (
          <Empty>No apps yet. An app you connect, such as an AI agent using dimmi's MCP server, shows here.</Empty>
        ) : null}
        {apps.map((app) => (
          <Item key={app.id} mark={<Glyph icon={RiRobot2Line} />} title={app.name} detail={appDetail(app)}>
            <Action onPress={() => setLeaving(app)}>Disconnect</Action>
          </Item>
        ))}
        {failure ? (
          <p role="alert" className="py-2.5 pe-3 text-body-2-regular text-text-error-primary">
            {failure}
          </p>
        ) : null}
      </Section>
      <AlertDialog
        isOpen={leaving !== null}
        onOpenChange={(open) => !open && setLeaving(null)}
        title={`Disconnect ${leaving?.name ?? "this app"}?`}
        description="It stops working at once and has to ask you again to connect."
        confirmLabel="Disconnect"
        onConfirm={() => {
          if (leaving) {
            void disconnect(leaving.id);
          }
        }}
      />
    </SettingsPage>
  );
}
