/*
 * How a broadcast did once it went out, filling in live while it sends.
 * A paused broadcast says why on top, with a way to resume or stop it.
 * The headline numbers come next, each as a rate, so a small send and a
 * big one read the same way. Below them the drop from sent to
 * delivered, opened, clicked, replied and the goal; email opens are
 * estimated, since mail privacy proxies open some mail on their own and
 * many readers block images. Then who was left out or never got it and
 * why, the links people clicked, and, for B2B, how each account did,
 * which narrows the recipients below it to that account.
 */
import { RiPauseCircleLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { type Stat, StatCards } from "@/components/application/dashboard/stat-cards";
import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { ProgressTrack } from "@/components/base/progress/progress";
import { Row, Section } from "@/features/settings/layout";
import {
  type Broadcast,
  type BroadcastAction,
  type BroadcastCompany,
  type BroadcastCounts,
  broadcastReportQuery,
  type BroadcastReport as Report,
} from "@/lib/api/broadcasts";
import { formatNumber, formatTime } from "@/lib/i18n/format";
import { useRealtime } from "@/lib/realtime/context";

import { counted, pauseReasons, percent, reachNoun, reasonName } from "./looks";
import { type Account, Recipients } from "./report-recipients";
import { zoneFor } from "./schedule";
import { useWorkspaceZone } from "./sources";

type Step = [label: string, count: number, note: string];

function stepsOf(broadcast: Broadcast, counts: BroadcastCounts): Step[] {
  const channel = broadcast.channel;
  const steps: Step[] = [
    ["Sent", counts.sent, channel === "slack" ? "Posted into the channel" : "Left dimmi"],
    ["Delivered", counts.delivered, channel === "email" ? "Accepted by their mail server" : "Reached them"],
  ];
  if (channel === "email") {
    steps.push(["Opened", counts.opened, "Estimated: privacy proxies open some mail, image blockers hide others"]);
  } else if (channel !== "slack") {
    steps.push(["Seen", counts.opened, "Opened it in the messenger"]);
  }
  if (channel !== "slack") {
    steps.push(["Clicked", counts.clicked, "Clicked a link in it"]);
  }
  if (channel === "slack") {
    steps.push(["Reacted", counts.reacted, `${counted(counts.reactions, "reaction", "reactions")} in all`]);
  }
  steps.push(["Replied", counts.replied, "Each reply is a conversation in the inbox"]);
  if (broadcast.goal) {
    steps.push([
      "Reached the goal",
      counts.converted,
      `Did ${broadcast.goal.event} within ${broadcast.goal.days} days`,
    ]);
  }

  return steps;
}

function Funnel({ steps, whole }: { steps: Step[]; whole: number }) {
  return (
    <Section title="From sent to the goal">
      <div className="flex flex-col gap-4 py-4 pe-4">
        {steps.map(([label, count, note]) => (
          <div key={label} className="flex flex-col gap-1.5">
            <div className="flex items-baseline justify-between gap-4">
              <span className="flex min-w-0 flex-col">
                <span className="text-body-regular text-text-primary">{label}</span>
                <span className="truncate text-body-2-regular text-text-tertiary">{note}</span>
              </span>
              <span className="shrink-0 text-body-2-medium text-text-primary tabular-nums">
                {formatNumber(count)}
                <span className="ms-2 text-text-tertiary">{percent(count, whole)}</span>
              </span>
            </div>
            <ProgressTrack percentage={(count / Math.max(whole, 1)) * 100} size="sm" />
          </div>
        ))}
      </div>
    </Section>
  );
}

function Count({ value }: { value: number }) {
  return <span className="text-body-2-medium text-text-primary tabular-nums">{formatNumber(value)}</span>;
}

function Problems({ report }: { report: Report }) {
  const { counts, broadcast } = report;
  const reasons = report.reasons ?? [];
  const email = broadcast.channel === "email";

  return (
    <Section title="Didn't land" description="Bounced and spam-reporting addresses are skipped by every later send.">
      {email ? (
        <>
          <Row label="Unsubscribed" description="Left from this email">
            <Count value={counts.unsubscribed} />
          </Row>
          <Row label="Bounced" description="The address doesn't exist or refused the mail">
            <Count value={counts.bounced} />
          </Row>
          <Row label="Reported spam" description="Marked it as spam in their inbox">
            <Count value={counts.complained} />
          </Row>
        </>
      ) : null}
      <Row label="Failed" description="Could not be delivered">
        <Count value={counts.failed} />
      </Row>
      <Row label="Left out" description="Never sent, for the reasons below">
        <Count value={counts.skipped + counts.suppressed} />
      </Row>
      {reasons.map((reason) => (
        <Row key={reason.reason || "none"} label={reasonName(reason.reason || "unknown")}>
          <Count value={reason.count} />
        </Row>
      ))}
    </Section>
  );
}

function Links({ report }: { report: Report }) {
  const links = report.links ?? [];
  if (links.length === 0) {
    return null;
  }

  return (
    <Section title="Links" description="People who clicked each link, most clicked first.">
      {[...links]
        .sort((a, b) => b.clicks - a.clicks)
        .map((link) => (
          <Row
            key={link.n}
            label={
              <span dir="ltr" className="break-all">
                {link.url}
              </span>
            }
          >
            <Count value={link.clicks} />
          </Row>
        ))}
    </Section>
  );
}

function Companies({ companies, onPick }: { companies: BroadcastCompany[]; onPick: (account: Account) => void }) {
  if (companies.length === 0) {
    return null;
  }

  return (
    <Section title="Accounts" description="How each account did. Pick one to see its recipients.">
      {companies.map((company) => (
        <Row
          key={company.company_id}
          label={company.name || "Unnamed account"}
          description={`${formatNumber(company.delivered)} of ${formatNumber(company.total)} delivered · ${percent(company.opened, company.delivered)} opened · ${formatNumber(company.clicked)} clicked · ${formatNumber(company.replied)} replied${company.reactions ? ` · ${formatNumber(company.reactions)} reactions` : ""}${company.failed ? ` · ${formatNumber(company.failed)} failed` : ""}`}
        >
          <LinkButton
            variant="primary"
            size="xs"
            onClick={() => onPick({ id: company.company_id, name: company.name, total: company.total })}
          >
            Recipients
          </LinkButton>
        </Row>
      ))}
    </Section>
  );
}

/*
 * Paused says why the broadcast stopped going out and offers to carry
 * on or stop for good.
 */
function Paused({ broadcast, onAction }: { broadcast: Broadcast; onAction: (action: BroadcastAction) => void }) {
  const why = broadcast.pause_reason ? pauseReasons[broadcast.pause_reason] : "Sending is paused.";

  return (
    <div
      role="status"
      className="flex flex-wrap items-center gap-3 rounded-2xl border border-border-button-default bg-background-primary-default px-4 py-3"
    >
      <RiPauseCircleLine aria-hidden className="size-5 shrink-0 text-foreground-icon-secondary" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-body-medium text-text-primary">Paused</span>
        <span className="text-body-2-regular text-text-secondary">{why}</span>
      </span>
      <Button variant="secondary" size="small" onClick={() => onAction("stop")}>
        Stop
      </Button>
      <Button size="small" onClick={() => onAction("resume")}>
        Resume
      </Button>
    </div>
  );
}

function Headline({ broadcast, counts }: { broadcast: Broadcast; counts: BroadcastCounts }) {
  const whole = counts.delivered;
  const third: Stat =
    broadcast.channel === "email"
      ? {
          label: "Opened",
          hint: "Share of delivered email that was opened. Some mail apps open email on their own or block tracking, so this is an estimate.",
          value: percent(counts.opened, whole),
          caption: "Estimated",
        }
      : {
          label: "Replied",
          hint: "Share of delivered messages that got a reply. Each reply is a conversation in the inbox.",
          value: percent(counts.replied, whole),
          caption: counted(counts.replied, "reply", "replies"),
        };
  const last: Stat =
    broadcast.channel === "slack"
      ? {
          label: "Reactions",
          hint: "Emoji reactions on the posts, across every Slack channel it went to.",
          value: formatNumber(counts.reactions),
          caption: counted(counts.reacted, "channel", "channels"),
        }
      : {
          label: "Clicked",
          hint: "Share of delivered messages where someone clicked a link.",
          value: percent(counts.clicked, whole),
          caption: `${formatNumber(counts.clicked)} ${reachNoun(broadcast.channel, counts.clicked)}`,
        };

  return (
    <StatCards
      variant="compact"
      className="gap-2"
      stats={[
        {
          label: "Recipients",
          hint: "Everyone this broadcast is for, including those still waiting to be sent to.",
          value: formatNumber(counts.total),
          caption: `${formatNumber(counts.queued)} waiting`,
        },
        {
          label: "Delivered",
          hint: "Share of sent messages that reached the person. The number under it is how many.",
          value: percent(counts.delivered, counts.sent),
          caption: formatNumber(whole),
        },
        third,
        last,
      ]}
    />
  );
}

const waitingReasons: Record<string, string> = {
  warming_up:
    "This address is new, so it sends a little more each day while mail providers learn to trust it. The rest wait their turn.",
  provider_limit: "Your mail provider's sending limit is reached for now; the rest go out as it frees up.",
  rate_limited: "The provider asked dimmi to slow down; the rest go out shortly.",
};

/*
 * Waiting says why some recipients are still in the queue while the
 * broadcast sends: each person's own time not reached yet, an address
 * warming up, or the provider's limit.
 */
function Waiting({ workspace, report }: { workspace: string; report: Report }) {
  const zone = useWorkspaceZone(workspace);
  const { broadcast, counts } = report;
  const held = (report.waiting ?? []).filter((w) => w.count > 0);
  const theirTime = broadcast.schedule_kind === "local" && broadcast.scheduled_at !== null && counts.queued > 0;
  if (broadcast.status !== "sending" || (held.length === 0 && !theirTime)) {
    return null;
  }

  return (
    <div
      role="status"
      className="flex flex-col gap-1 rounded-2xl border border-border-button-default bg-background-primary-default px-4 py-3"
    >
      <span className="text-body-medium text-text-primary">
        {theirTime ? "Going out in each person's own time" : "Sending at a steady pace"}
      </span>
      {theirTime && broadcast.scheduled_at ? (
        <span className="text-body-2-regular text-text-secondary">
          Each person gets it at {formatTime(broadcast.scheduled_at, { timeZone: zoneFor("local", zone) })} where they
          are, and people without a time zone at that time in the workspace's. Stop it to keep it from anyone still
          waiting.
        </span>
      ) : null}
      {held.map((w) => (
        <span key={w.reason} className="text-body-2-regular text-text-secondary">
          {waitingReasons[w.reason] ?? reasonName(w.reason)}
        </span>
      ))}
    </div>
  );
}

export function BroadcastReport({
  workspace,
  broadcast,
  onAction,
}: {
  workspace: string;
  broadcast: Broadcast;
  onAction: (action: BroadcastAction) => void;
}) {
  const { state } = useRealtime();
  const going = broadcast.status === "sending";
  const { data: report } = useQuery({
    ...broadcastReportQuery(workspace, broadcast.id),
    refetchInterval: going && state !== "connected" ? 10_000 : false,
  });
  const [account, setAccount] = useState<Account | null>(null);
  const counts = report?.counts ?? broadcast.counts;

  return (
    <div className="mx-auto flex w-full max-w-[960px] flex-col gap-8 px-4 pt-6 pb-16 sm:px-6">
      {broadcast.status === "paused" ? <Paused broadcast={broadcast} onAction={onAction} /> : null}
      {report ? <Waiting workspace={workspace} report={report} /> : null}
      {counts ? <Headline broadcast={broadcast} counts={counts} /> : null}
      {counts ? <Funnel steps={stepsOf(broadcast, counts)} whole={Math.max(counts.sent, 1)} /> : null}
      {report ? <Problems report={report} /> : null}
      {report ? <Links report={report} /> : null}
      {report ? <Companies companies={report.companies ?? []} onPick={setAccount} /> : null}
      {counts ? (
        <Recipients
          workspace={workspace}
          broadcast={broadcast}
          counts={counts}
          account={account}
          onClearAccount={() => setAccount(null)}
        />
      ) : null}
    </div>
  );
}
