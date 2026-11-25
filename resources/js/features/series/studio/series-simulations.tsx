/*
 * A series' simulation, written the way a chat with the agent is: who
 * travels it sits in the composer (a made-up person to start with, or
 * a real contact picked through the plus), and the text says how they
 * behave, such as "opens every email but never clicks". Sending walks
 * them through the draft or the live version at once: every wait is
 * skipped and nothing is sent. The walk reads as their journey: each
 * message they would get, what they did when the series asked, each
 * path it sent them down and what it would change, and how it ended.
 * The AI reads the scenario to answer each "did they open, click,
 * reply, do the event". Extract steps ask the AI about the person; run
 * action steps succeed with nothing and never run.
 */
import { RiSparkling2Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { toast } from "@/components/base/toast/toast";
import { KindMark } from "@/features/automation/parts";
import { madeUpPerson } from "@/features/automation/procedure-simulator/made-up";
import { PreviewAddMenu } from "@/features/automation/procedure-simulator/preview-add-menu";
import { Added, PreviewComposer } from "@/features/automation/procedure-simulator/preview-composer";
import { fromCustomer, hint } from "@/features/automation/procedure-simulator/simulator";
import type { OperatorTry } from "@/features/automation/studio/use-operator-tries";
import { MessageBody } from "@/features/inbox/message-body";
import { avatarColor } from "@/features/shell/avatar-color";
import { ScrollArea } from "@/features/shell/page";
import type { AnswerAs } from "@/lib/api/agents";
import type { Contact } from "@/lib/api/customers";
import type { SimPersona } from "@/lib/api/procedures";
import { seriesQuery } from "@/lib/api/series";
import { type SeriesSimulation, simulateSeries } from "@/lib/api/workflow-simulations";
import { cx } from "@/utils/cx";
import { nodeLook } from "../looks";

import { journey, type Moment } from "./series-sim-words";

const copies = [
  ["draft", "Draft"],
  ["live", "Live"],
] as const;

const examples = [
  "Opens every email but never clicks",
  "Clicks the first link, then goes quiet",
  "Replies to the first email asking for a call",
];

/* Traveler is who walks the series: a real contact, or a made-up person. */
type Traveler = { kind: "contact"; contact: Contact } | { kind: "made_up"; person: SimPersona };

/* Walk is one sent scenario: what was written, who walked it, how it went, and whether Operator sent it. */
interface Walk {
  key: string;
  scenario: string;
  name: string;
  result: SeriesSimulation | null;
  failed?: string;
  operator?: boolean;
}

/* walkOf reads one of Operator's tries as a walk. */
function walkOf(t: OperatorTry): Walk {
  return {
    key: t.id,
    scenario: t.scenario ?? "",
    name: firstName(t.name || "Someone"),
    result: t.walk ?? null,
    failed: t.failed,
    operator: true,
  };
}

function travelerName(t: Traveler): string {
  if (t.kind === "contact") {
    return t.contact.name || t.contact.email || "This contact";
  }

  return t.person.name || t.person.email || "Someone";
}

function firstName(name: string): string {
  return name.split(" ")[0] || name;
}

/* MomentLine draws one beat of the journey. */
function MomentLine({ moment }: { moment: Moment }) {
  if (moment.kind === "message") {
    const look = nodeLook[moment.node];

    return (
      <div className="flex flex-col gap-1.5">
        <span className={cx(hint, "flex items-center gap-1.5")}>
          <KindMark icon={look.icon} tint={look.tint} size="sm" />
          {look.name}
          {moment.to ? ` to ${moment.to}` : ""}
        </span>
        <div className="flex flex-col gap-1 rounded-2xl bg-background-primary-default px-3.5 py-3 shadow-card">
          {moment.subject ? <span className="text-body-medium text-text-primary">{moment.subject}</span> : null}
          <MessageBody text={moment.body} className="text-body-regular text-text-secondary" />
        </div>
      </div>
    );
  }
  if (moment.kind === "did") {
    return (
      <p className="flex justify-end">
        <span
          className={cx(
            "max-w-[85%] rounded-2xl px-2.5 py-1 text-body-2-medium",
            moment.did ? "bg-status-lime-background text-status-lime-text" : "bg-background-tertiary-default",
            moment.did ? null : "text-text-secondary",
          )}
        >
          {moment.text}
        </span>
      </p>
    );
  }
  const look = nodeLook[moment.node];

  return (
    <div className="flex flex-col items-center gap-1 px-4">
      <span
        className={cx(
          "flex max-w-full items-center gap-1.5 rounded-2xl bg-background-tertiary-default py-1 ps-1 pe-2.5 text-body-2-medium",
          moment.muted ? "text-text-tertiary" : "text-text-secondary",
        )}
      >
        <KindMark icon={look.icon} tint={look.tint} size="sm" />
        <span className="min-w-0 wrap-break-word">{moment.text}</span>
      </span>
      {moment.note ? <span className={cx(hint, "text-center")}>{moment.note}</span> : null}
    </div>
  );
}

/* WalkView is one sent scenario and the journey it drew. */
function WalkView({ walk, spec }: { walk: Walk; spec: Parameters<typeof journey>[2] }) {
  const { result } = walk;

  return (
    <li className="flex flex-col gap-4">
      <div className="ms-auto flex max-w-[85%] flex-col items-end gap-1">
        <span className={hint}>
          How {walk.name} behaves
        </span>
        <p className={cx(fromCustomer, "max-w-full")}>{walk.scenario || "Does nothing with the messages"}</p>
      </div>
      {walk.failed ? (
        <p className="text-body-2-regular text-text-error-primary">{walk.failed}</p>
      ) : result === null ? (
        <p className={cx(hint, "flex items-center gap-1.5")}>
          <RiSparkling2Line aria-hidden className="size-4 animate-pulse text-foreground-icon-tertiary" />
          Walking {walk.name} through it…
        </p>
      ) : !result.entered ? (
        <p className="text-body-2-regular text-text-error-primary">
          {walk.name} would not enter:{" "}
          {result.reason === "event_not_done" ? "they do not do the entry event" : "they do not match who enters"}.
        </p>
      ) : (
        <ol aria-label={`${walk.name}'s journey`} className="flex flex-col gap-4">
          <li className="flex justify-center">
            <span className={hint}>{walk.name} entered the series. Waits are skipped and nothing is sent.</span>
          </li>
          {journey(result, walk.name, spec).map((moment) => (
            <li key={moment.key}>
              <MomentLine moment={moment} />
            </li>
          ))}
        </ol>
      )}
    </li>
  );
}

function Blank({ onExample }: { onExample: (text: string) => void }) {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-4 px-6 py-10 text-center">
      <span className="flex flex-col gap-1">
        <span className="text-body-medium text-text-primary">Walk someone through it</span>
        <span className={hint}>
          Say how they behave below. Every wait is skipped and nothing is sent, so a month-long series runs in seconds.
        </span>
      </span>
      <span className="flex flex-col items-center gap-1.5">
        {examples.map((text) => (
          <button
            key={text}
            type="button"
            onClick={() => onExample(text)}
            className="cursor-pointer rounded-full bg-background-secondary-default px-3 py-1.5 text-body-2-medium text-text-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            {text}
          </button>
        ))}
      </span>
    </div>
  );
}

export function SeriesSimulations({
  workspace,
  series,
  tries = [],
}: {
  workspace: string;
  series: string;
  /* Operator's tries of this series, shown live with the walks sent here. */
  tries?: OperatorTry[];
}) {
  const row = useQuery(seriesQuery(workspace, series)).data;
  const [as, setAs] = useState<AnswerAs>("draft");
  const [traveler, setTraveler] = useState<Traveler>(() => ({ kind: "made_up", person: madeUpPerson() }));
  const [scenario, setScenario] = useState("");
  const [walks, setWalks] = useState<Walk[]>([]);
  const end = useRef<HTMLDivElement>(null);
  const hasLive = Boolean(row?.live_version_id);
  const copy = hasLive ? as : "draft";
  const busy = walks.some((w) => w.result === null);
  const shown = [...walks, ...tries.map(walkOf)];
  const name = travelerName(traveler);

  // biome-ignore lint/correctness/useExhaustiveDependencies: scrolls to the latest walk whenever one is added or lands.
  useEffect(() => {
    end.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [walks, tries]);

  const send = async () => {
    if (busy) {
      return;
    }
    const text = scenario.trim();
    const at = walks.length;
    const who = firstName(name);
    setWalks((list) => [...list, { key: crypto.randomUUID(), scenario: text, name: who, result: null }]);
    setScenario("");
    try {
      const result = await simulateSeries(workspace, series, {
        version: copy,
        ...(traveler.kind === "contact"
          ? { contact_id: traveler.contact.id }
          : {
              person: {
                name: traveler.person.name,
                email: traveler.person.email,
                attributes: traveler.person.attributes,
              },
            }),
        ...(text ? { scenario: text } : {}),
      });
      setWalks((list) => list.map((w, i) => (i === at ? { ...w, result } : w)));
    } catch (error) {
      setWalks((list) => list.filter((_, i) => i !== at));
      setScenario(text);
      toast.error(error instanceof Error ? error.message : "The simulation did not run.");
    }
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {hasLive ? (
        <div className="shrink-0 px-4 pt-3">
          <SegmentedControl
            aria-label="Run against"
            selectedKeys={new Set([copy])}
            onSelectionChange={(keys) => setAs([...keys][0] === "live" ? "live" : "draft")}
          >
            {copies.map(([id, label]) => (
              <SegmentedControlItem key={id} id={id}>
                {label}
              </SegmentedControlItem>
            ))}
          </SegmentedControl>
        </div>
      ) : null}
      {shown.length === 0 ? (
        <Blank onExample={setScenario} />
      ) : (
        <ScrollArea>
          <ol className="flex flex-col gap-10 px-4 pt-4 pb-6">
            {shown.map((walk) => (
              <WalkView key={walk.key} walk={walk} spec={copy === "draft" && !walk.operator ? row?.spec : undefined} />
            ))}
          </ol>
          <div ref={end} />
        </ScrollArea>
      )}
      <div className="shrink-0 px-4 pb-4">
        <PreviewComposer
          value={scenario}
          onValueChange={setScenario}
          onSubmit={() => void send()}
          onStop={() => undefined}
          busy={busy}
          placeholder={`How does ${firstName(name)} behave? Like: opens every email, never replies`}
          canSubmit
          added={
            <div className="flex flex-wrap gap-1.5 px-0.5 pt-0.5">
              <Added
                mark={<Avatar size="md" color={avatarColor(name)} initials={name.charAt(0).toUpperCase()} />}
                title={name}
                detail={traveler.kind === "contact" ? "A real contact" : "A made-up person"}
                onRemove={() => setTraveler({ kind: "made_up", person: madeUpPerson() })}
              />
            </div>
          }
          menu={
            <PreviewAddMenu
              workspace={workspace}
              replayable={false}
              onCustomer={(person) => setTraveler({ kind: "made_up", person })}
              onContact={(contact) => setTraveler({ kind: "contact", contact })}
              onConversation={() => undefined}
            />
          }
        />
      </div>
    </div>
  );
}
