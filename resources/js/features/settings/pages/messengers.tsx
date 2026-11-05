/*
 * The workspace's messengers: one row each with the inbox its
 * conversations land in and the app id its snippet names, opening the
 * messenger's own page. A new messenger lands in the default inbox and
 * can be moved from its inbox's page.
 */
import { RiArrowRightSLine, RiChat3Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { inboxesQuery } from "@/lib/api/inboxes";
import { widgetSettingsQuery } from "@/lib/api/widget-settings";

import { usePermission } from "../inboxes";
import { Empty, Glyph, Item, Section, SettingsPage } from "../layout";
import { rowLink } from "../row-link";
import { useAccount } from "../use-account";
import { AddMessenger } from "./inbox-channels";

export function MessengersSettings() {
  const workspace = useAccount().workspace.id;
  const { data: widgets } = useQuery(widgetSettingsQuery(workspace));
  const { data: inboxes = [] } = useQuery(inboxesQuery(workspace));
  const canManage = usePermission(workspace, "inboxes:manage");
  const home = inboxes.find((inbox) => inbox.default) ?? inboxes[0];

  return (
    <SettingsPage title="Messengers" description="The chat on your site or in your app. Each has its own snippet.">
      <Section
        title="Messengers"
        action={canManage && home ? <AddMessenger workspace={workspace} inbox={home} /> : null}
        loading={!widgets}
        rows={2}
      >
        {widgets?.length === 0 ? <Empty>No messenger yet.</Empty> : null}
        {widgets?.map((widget) => (
          <Item
            key={widget.id}
            mark={<Glyph icon={RiChat3Line} />}
            title={
              <Link to="/d/settings/messenger/$widget" params={{ widget: widget.id }} className={rowLink}>
                {widget.name}
              </Link>
            }
            detail={`${widget.inbox.name} inbox · ${widget.id}`}
          >
            <RiArrowRightSLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
          </Item>
        ))}
      </Section>
    </SettingsPage>
  );
}
