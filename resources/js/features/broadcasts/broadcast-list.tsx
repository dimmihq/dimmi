/*
 * The broadcasts page: every one-off message the team sent or is about
 * to send, beside a pane that narrows the list to one state or one
 * channel. A row says who it went to, where it stands, and how it did,
 * as rates of the people it reached, so two sends of very different
 * sizes compare at a glance. The last month's totals sit on top. A new
 * broadcast starts by picking its channel, since the composer and the
 * numbers that matter follow from it.
 */
import { RiAddLine, RiArrowRightSLine, RiListCheck2, RiStackLine } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { type ComponentType, useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { DataTable } from "@/components/application/data-table/data-table";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { KindMark } from "@/features/automation/parts";
import { useWorkspace } from "@/features/customers/names";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useWide } from "@/features/shell/use-wide";
import { audiencesQuery } from "@/lib/api/audiences";
import {
  type Broadcast,
  type BroadcastCounts,
  type BroadcastStatus,
  broadcastsQuery,
  type Channel,
  createBroadcast,
} from "@/lib/api/broadcasts";
import { formatNumber } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { useLiveBroadcasts } from "./live";
import { channelLook, channels, noteOf, percent, statuses, statusName } from "./looks";
import { BroadcastStatusChip, ChannelMark, Stat } from "./parts";
import { useWorkspaceZone } from "./sources";
import { TopicsModal } from "./topics-modal";
import { plainText } from "./writer/variables";

type View = { kind: "all" } | { kind: "status"; status: BroadcastStatus } | { kind: "channel"; channel: Channel };

const pane = { group: "dimmi:broadcasts", side: "views", list: "broadcasts" } as const;

const visible = { overflow: "visible" } as const;

function holds(view: View, broadcast: Broadcast): boolean {
  if (view.kind === "status") {
    return broadcast.status === view.status;
  }
  if (view.kind === "channel") {
    return broadcast.channel === view.channel;
  }

  return true;
}

function titleOf(view: View): string {
  if (view.kind === "status") {
    return statusName(view.status);
  }
  if (view.kind === "channel") {
    return channelLook[view.channel].name;
  }

  return "All broadcasts";
}

function keyOf(view: View): string {
  if (view.kind === "status") {
    return view.status;
  }
  if (view.kind === "channel") {
    return `channel:${view.channel}`;
  }

  return "all";
}

function viewOf(key: string): View {
  if (key.startsWith("channel:")) {
    return { kind: "channel", channel: key.slice(8) as Channel };
  }
  if ((statuses as string[]).includes(key)) {
    return { kind: "status", status: key as BroadcastStatus };
  }

  return { kind: "all" };
}

function rowClass(current: boolean): string {
  return cx(
    "flex min-h-8 w-full items-center gap-2 rounded-xl ps-3 pe-2 text-text-primary outline-none",
    "transition-[background-color,box-shadow] duration-150 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
    current
      ? "bg-background-primary-default text-body-2-medium shadow-xs ring-1 ring-inset ring-border-button-default"
      : "text-body-2-regular hover:bg-background-secondary-hover",
  );
}

function Pick({
  label,
  icon: Icon,
  count,
  current,
  onPress,
}: {
  label: string;
  icon?: ComponentType<{ className?: string }>;
  count: number;
  current: boolean;
  onPress: () => void;
}) {
  return (
    <button type="button" aria-current={current ? "page" : undefined} onClick={onPress} className={rowClass(current)}>
      {Icon ? <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary opacity-50" /> : null}
      <span className="min-w-0 flex-1 truncate text-start">{label}</span>
      {count ? <span className="text-caption-1-regular text-text-tertiary tabular-nums">{count}</span> : null}
    </button>
  );
}

function Views({ broadcasts, view, onView }: { broadcasts: Broadcast[]; view: View; onView: (view: View) => void }) {
  const current = keyOf(view);
  const workspace = useWorkspace();
  const [topics, setTopics] = useState(false);
  const count = (next: View) => broadcasts.filter((broadcast) => holds(next, broadcast)).length;

  return (
    <Panel label="Broadcast views" className="h-full w-full">
      <header className="flex h-14 shrink-0 items-center gap-2 ps-4 pe-2.5">
        <h2 className="truncate text-body-medium text-text-primary">Broadcasts</h2>
      </header>
      <ScrollArea>
        <nav aria-label="Broadcast views" className="flex flex-col gap-5 px-2 pb-4">
          <div className="flex flex-col gap-0.5">
            <Pick
              label="All broadcasts"
              icon={RiStackLine}
              count={broadcasts.length}
              current={current === "all"}
              onPress={() => onView({ kind: "all" })}
            />
            {statuses.map((status) => (
              <Pick
                key={status}
                label={statusName(status)}
                count={count({ kind: "status", status })}
                current={current === status}
                onPress={() => onView({ kind: "status", status })}
              />
            ))}
          </div>
          <div className="flex flex-col gap-0.5">
            <span className="px-3 pb-1 text-caption-1-medium text-text-tertiary">Channels</span>
            {channels.map((channel) => (
              <Pick
                key={channel}
                label={channelLook[channel].name}
                icon={channelLook[channel].icon}
                count={count({ kind: "channel", channel })}
                current={current === `channel:${channel}`}
                onPress={() => onView({ kind: "channel", channel })}
              />
            ))}
          </div>
          <div className="flex flex-col gap-0.5">
            <span className="px-3 pb-1 text-caption-1-medium text-text-tertiary">Subscriptions</span>
            <Pick label="Topics" icon={RiListCheck2} count={0} current={false} onPress={() => setTopics(true)} />
          </div>
        </nav>
      </ScrollArea>
      {topics ? <TopicsModal workspace={workspace} onClose={() => setTopics(false)} /> : null}
    </Panel>
  );
}

function NameCell({ broadcast }: { broadcast: Broadcast }) {
  return (
    <span className="flex min-w-0 max-w-80 items-center gap-3">
      <ChannelMark channel={broadcast.channel} />
      <span className="flex min-w-0 flex-col">
        <span className="truncate text-body-medium text-text-primary">{broadcast.name}</span>
        <span className="truncate text-body-2-regular text-text-tertiary">{plainText(broadcast.subject)}</span>
      </span>
    </span>
  );
}

/*
 * Results shows how a sent broadcast did, as rates of the people it
 * reached: the number that matters most for its channel first, the next
 * one under it.
 */
function Results({ broadcast }: { broadcast: Broadcast }) {
  const counts = broadcast.counts;
  if (!counts || broadcast.status === "draft" || broadcast.status === "scheduled") {
    return <span className="text-body-2-regular text-text-tertiary">Not sent yet</span>;
  }
  const base = counts.delivered;
  if (broadcast.channel === "slack") {
    return (
      <span className="flex min-w-0 flex-col">
        <span className="text-body-2-medium text-text-primary tabular-nums">
          {formatNumber(counts.reactions)} reactions
        </span>
        <span className="text-caption-1-regular text-text-tertiary tabular-nums">
          {formatNumber(counts.replied)} replied
        </span>
      </span>
    );
  }
  const second =
    broadcast.channel === "chat"
      ? `${percent(counts.replied, base)} replied`
      : `${percent(counts.clicked, base)} clicked`;

  return (
    <span className="flex min-w-0 flex-col">
      <span className="text-body-2-medium text-text-primary tabular-nums">
        {percent(counts.opened, base)} {broadcast.channel === "email" ? "opened" : "saw it"}
      </span>
      <span className="text-caption-1-regular text-text-tertiary tabular-nums">{second}</span>
    </span>
  );
}

function reachOf(broadcast: Broadcast): string {
  const total = broadcast.counts?.total ?? 0;
  if (total === 0) {
    return "Counted when it sends";
  }

  return `${formatNumber(total)} ${broadcast.channel === "slack" ? "channels" : "people"}`;
}

function columnsOf(audiences: Map<string, string>, zone: string | undefined): ColumnDef<Broadcast>[] {
  return [
    { id: "broadcast", header: "Broadcast", cell: ({ row }) => <NameCell broadcast={row.original} /> },
    {
      id: "audience",
      header: "Audience",
      cell: ({ row }) => (
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-body-2-regular text-text-primary">{audienceOf(row.original, audiences)}</span>
          <span className="text-caption-1-regular text-text-tertiary tabular-nums">{reachOf(row.original)}</span>
        </span>
      ),
    },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) => (
        <span className="flex min-w-0 flex-col items-start gap-0.5">
          <BroadcastStatusChip status={row.original.status} />
          <span className="truncate text-caption-1-regular text-text-tertiary">{noteOf(row.original, zone)}</span>
        </span>
      ),
    },
    { id: "results", header: "Results", cell: ({ row }) => <Results broadcast={row.original} /> },
  ];
}

function audienceOf(broadcast: Broadcast, audiences: Map<string, string>): string {
  const named = broadcast.audience_id ? audiences.get(broadcast.audience_id) : undefined;
  if (broadcast.channel === "slack") {
    return named ? `Slack channels of ${named}` : "Customer Slack channels";
  }
  if (named) {
    return named;
  }

  return broadcast.rules ? "Its own rule" : "No audience yet";
}

const widths = { broadcast: "w-[36%]", audience: "w-[22%]", status: "w-[24%]", results: "w-[18%]" };

const month = 30 * 24 * 60 * 60 * 1000;

/*
 * MonthStats adds up the broadcasts that started going out in the last
 * thirty days.
 */
function MonthStats({ broadcasts }: { broadcasts: Broadcast[] }) {
  const since = Date.now() - month;
  const sent = broadcasts.flatMap((broadcast) =>
    broadcast.counts && broadcast.started_at && Date.parse(broadcast.started_at) >= since ? [broadcast.counts] : [],
  );
  const sum = (pick: (counts: BroadcastCounts) => number) => sent.reduce((total, c) => total + pick(c), 0);
  const delivered = sum((c) => c.delivered);

  return (
    <div className="grid grid-cols-2 gap-2 lg:grid-cols-4">
      <Stat label="Delivered" value={formatNumber(delivered)} note="Last 30 days" />
      <Stat
        label="Opened"
        value={percent(
          sum((c) => c.opened),
          delivered,
        )}
        note="Email opens are estimated"
      />
      <Stat
        label="Clicked"
        value={percent(
          sum((c) => c.clicked),
          delivered,
        )}
        note="Of delivered"
      />
      <Stat label="Replies" value={formatNumber(sum((c) => c.replied))} note="Landed in the inbox" />
    </div>
  );
}

/*
 * ChannelPicker is the first step of a new broadcast: where it goes.
 */
function ChannelPicker({
  isOpen,
  onOpenChange,
  onPick,
}: {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onPick: (channel: Channel) => void;
}) {
  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange} size="md">
      <ModalHeader title="New broadcast" description="Where should it reach people? You can't change this later." />
      <ModalBody>
        <div className="flex flex-col gap-0.5 rounded-2xl bg-background-secondary-default p-1">
          {channels.map((channel) => {
            const look = channelLook[channel];

            return (
              <button
                key={channel}
                type="button"
                onClick={() => onPick(channel)}
                className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
              >
                <KindMark icon={look.icon} tint={look.tint} />
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="text-body-regular text-text-primary">{look.name}</span>
                  <span className="text-body-2-regular text-text-secondary">{look.where}</span>
                </span>
                <RiArrowRightSLine
                  aria-hidden
                  className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100"
                />
              </button>
            );
          })}
        </div>
      </ModalBody>
    </Modal>
  );
}

function NarrowViews({ view, onView }: { view: View; onView: (view: View) => void }) {
  const choices: [string, string][] = [
    ["all", "All broadcasts"],
    ...statuses.map((status): [string, string] => [status, statusName(status)]),
    ...channels.map((channel): [string, string] => [`channel:${channel}`, channelLook[channel].name]),
  ];

  return (
    <div className="px-4 pb-3">
      <Select
        aria-label="View"
        className="w-full"
        selectedKey={keyOf(view)}
        onSelectionChange={(key) => onView(viewOf(String(key ?? "all")))}
      >
        {choices.map(([id, name]) => (
          <SelectItem key={id} id={id} textValue={name}>
            {name}
          </SelectItem>
        ))}
      </Select>
    </div>
  );
}

export function BroadcastList() {
  const workspace = useWorkspace();
  useLiveBroadcasts(workspace);
  const { data: broadcasts = [], isPending } = useQuery(broadcastsQuery(workspace));
  const audienceList = useQuery(audiencesQuery(workspace)).data ?? [];
  const audiences = new Map(audienceList.map((audience) => [audience.id, audience.name]));
  const zone = useWorkspaceZone(workspace);
  const navigate = useNavigate();
  const wide = useWide(1024);
  const [view, setView] = useState<View>({ kind: "all" });
  const [query, setQuery] = useState("");
  const [picking, setPicking] = useState(false);
  const needle = query.trim().toLowerCase();
  const rows = broadcasts.filter(
    (broadcast) =>
      holds(view, broadcast) &&
      (needle === "" || `${broadcast.name} ${broadcast.subject}`.toLowerCase().includes(needle)),
  );
  const open = (id: string) => void navigate({ to: "/d/broadcasts/$broadcast", params: { broadcast: id } });
  const start = useMutation({
    mutationFn: (channel: Channel) =>
      createBroadcast(workspace, `Untitled ${channelLook[channel].name.toLowerCase()}`, channel),
    onSuccess: (broadcast) => open(broadcast.id),
    onError: (error) => toast.error(error.message),
  });

  const main = (
    <Panel label="Broadcasts" ground className="h-full w-full flex-1">
      <PageHeader title={wide ? titleOf(view) : "Broadcasts"}>
        <Button size="small" leadingIcon={RiAddLine} disabled={start.isPending} onClick={() => setPicking(true)}>
          New broadcast
        </Button>
      </PageHeader>
      <ChannelPicker
        isOpen={picking}
        onOpenChange={setPicking}
        onPick={(channel) => {
          setPicking(false);
          start.mutate(channel);
        }}
      />
      <ScrollArea ground>
        {wide ? null : <NarrowViews view={view} onView={setView} />}
        <div key={keyOf(view)} className="flex h-full flex-col gap-4 px-4 pb-4 animate-page-in">
          <MonthStats broadcasts={broadcasts} />
          <div className="min-h-[420px] flex-1">
            <DataTable
              aria-label="Broadcasts"
              data={rows}
              columns={columnsOf(audiences, zone)}
              getRowId={(broadcast) => broadcast.id}
              total={`${formatNumber(rows.length)} ${rows.length === 1 ? "broadcast" : "broadcasts"}`}
              toolbar={
                <Input
                  aria-label="Search broadcasts"
                  placeholder="Search broadcasts"
                  value={query}
                  onChange={setQuery}
                  className="w-64"
                />
              }
              page={1}
              pageCount={1}
              onPageChange={() => undefined}
              onRowAction={open}
              loading={isPending}
              widths={widths}
              empty={needle ? "No broadcast matches." : "Nothing here yet. Send your first broadcast."}
            />
          </div>
        </div>
      </ScrollArea>
    </Panel>
  );

  if (!wide) {
    return main;
  }

  return (
    <PaneGroup id={pane.group} panelIds={[pane.side, pane.list]}>
      <Pane
        id={pane.side}
        defaultSize={240}
        minSize={200}
        maxSize={360}
        groupResizeBehavior="preserve-pixel-size"
        style={visible}
        className="flex h-full"
      >
        <Views broadcasts={broadcasts} view={view} onView={setView} />
      </Pane>
      <PaneHandle />
      <Pane id={pane.list} minSize={560} style={visible} className="flex h-full">
        {main}
      </Pane>
    </PaneGroup>
  );
}
