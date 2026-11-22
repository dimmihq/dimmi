/*
 * Preview: chat with the agent as a customer would, answered by its
 * draft or its live version, the way a procedure previews. Each turn
 * shows what the agent did as it works, then its reply. Nothing reaches
 * a customer or your systems: actions answer with their mocks or their
 * last test. A chat worth keeping is saved as a simulation.
 */
import { RiChat3Line, RiRestartLine, RiTestTubeLine } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { subjectOf } from "@/features/inbox/conversations";
import { MessageBody } from "@/features/inbox/message-body";
import { ActivityLog } from "@/features/operator/activity-log";
import { avatarColor } from "@/features/shell/avatar-color";
import { ScrollArea } from "@/features/shell/page";
import { Reveal } from "@/features/shell/reveal";
import {
  type AgentPreview,
  type AgentPreviewInput,
  type AnswerAs,
  agentSimulationsQuery,
  createAgentSimulation,
  previewAgent,
} from "@/lib/api/agents";
import type { Conversation } from "@/lib/api/inbox";
import type { OperatorStep } from "@/lib/api/operator";
import type { Assertion, SimMessage, SimPersona } from "@/lib/api/procedures";
import { callerQuery } from "@/lib/api/workspace";
import type { Envelope } from "@/lib/realtime/bridge";
import { channels } from "@/lib/realtime/keys";
import { useChannel } from "@/lib/realtime/use-channel";

import { PreviewAddMenu } from "../automation/procedure-simulator/preview-add-menu";
import { Added, PreviewComposer } from "../automation/procedure-simulator/preview-composer";
import {
  actionName,
  card,
  fromCustomer,
  hint,
  type LiveStep,
  liveStep,
  words,
} from "../automation/procedure-simulator/simulator";
import type { ChipSources } from "../automation/procedure-writer/procedure-suggestions";

interface Turn {
  message: string;
  at: string;
  result: AgentPreview;
  steps: OperatorStep[];
}

interface Streamed {
  stream_id?: string;
  step?: LiveStep;
}

const madeUp: SimPersona = { verified: true, channel: "widget" };

function historyOf(turns: Turn[]): SimMessage[] {
  return turns.flatMap((t) => [
    ...(t.message ? [{ role: "customer" as const, text: t.message }] : []),
    ...(t.result.reply ? [{ role: "agent" as const, text: t.result.reply }] : []),
  ]);
}

/*
 * eventsOf reads a reply as the things the agent did, for when none of
 * it was streamed: the procedure it followed, its actions and a handover.
 */
function eventsOf(result: AgentPreview, before: AgentPreview | undefined, sources: ChipSources): OperatorStep[] {
  const trace = result.procedure_trace;
  const started: OperatorStep[] =
    trace && trace.procedure_id !== before?.procedure_trace?.procedure_id
      ? [{ tool: "follow_procedure", summary: `Followed ${trace.procedure_name}`, ok: true }]
      : [];
  const calls = result.action_calls.map((call): OperatorStep => {
    const name = actionName(sources, call.action_id || call.tool);

    return call.blocked
      ? { tool: "action_blocked", summary: `Refused ${name}`, ok: false }
      : { tool: "action_call", summary: `Called ${name}`, ok: call.succeeded };
  });
  const handover: OperatorStep[] = result.handoff
    ? [{ tool: "handoff", summary: `Handed over: ${words(result.handoff.reason).toLowerCase()}`, ok: true }]
    : [];

  return [...started, ...calls, ...handover];
}

/*
 * checksOf are what a chat saved as a simulation checks: every action
 * it called is called again, and it ends the same way, handed over or not.
 */
function checksOf(turns: AgentPreview[]): Assertion[] {
  const out: Assertion[] = [];
  const seen = new Set<string>();
  for (const turn of turns) {
    for (const call of turn.action_calls) {
      if (!call.blocked && !seen.has(call.tool)) {
        seen.add(call.tool);
        out.push({ type: "action_called", action: call.tool });
      }
    }
  }
  const handoff = turns.find((turn) => turn.handoff !== null)?.handoff;
  out.push(
    handoff?.team_id
      ? { type: "handed_off", handed_off: true, team_id: handoff.team_id }
      : { type: "handed_off", handed_off: handoff !== undefined },
  );

  return out;
}

function AddedRow({
  persona,
  replay,
  onNoCustomer,
  onNoReplay,
}: {
  persona: SimPersona;
  replay: Conversation | null;
  onNoCustomer: () => void;
  onNoReplay: () => void;
}) {
  const customer = persona.name || persona.email;
  if (!customer && !replay) {
    return null;
  }

  return (
    <div className="flex flex-wrap gap-1.5 px-0.5 pt-0.5">
      {customer ? (
        <Added
          mark={<Avatar size="md" color={avatarColor(customer)} initials={customer.charAt(0).toUpperCase()} />}
          title={customer}
          detail={persona.name && persona.email ? `Testing as ${persona.email}` : "Testing as this customer"}
          onRemove={onNoCustomer}
        />
      ) : null}
      {replay ? (
        <Added
          mark={
            <span className="flex size-8 items-center justify-center rounded-full bg-background-tertiary-default">
              <RiChat3Line aria-hidden className="size-4 text-foreground-icon-secondary" />
            </span>
          }
          title={subjectOf(replay)}
          detail={`Replaying conversation #${replay.number}`}
          onRemove={onNoReplay}
        />
      ) : null}
    </div>
  );
}

function Blank({ name, as }: { name: string; as: AnswerAs }) {
  return (
    <li className="flex flex-col items-center gap-3 text-center">
      <span className={`flex size-10 items-center justify-center rounded-full ${card}`}>
        <RiTestTubeLine aria-hidden className="size-5 text-foreground-icon-secondary" />
      </span>
      <div className="flex flex-col gap-1">
        <span className="text-body-medium text-text-primary">Write to {name} as a customer would</span>
        <span className={hint}>
          {as === "draft" ? "The draft answers." : "The live version answers."} Nothing reaches a customer or your
          systems.
        </span>
      </div>
    </li>
  );
}

export function AgentPreviewChat({
  workspace,
  agent,
  name,
  as,
  sources,
  flush,
  onSaved,
}: {
  workspace: string;
  agent: string;
  name: string;
  as: AnswerAs;
  sources: ChipSources;
  /* Saves the draft's pending changes before it answers. */
  flush: () => Promise<void>;
  onSaved: () => void;
}) {
  const queryClient = useQueryClient();
  const [persona, setPersona] = useState<SimPersona>(madeUp);
  const [replay, setReplay] = useState<Conversation | null>(null);
  const [turns, setTurns] = useState<Turn[]>([]);
  const [asked, setAsked] = useState<string | null>(null);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [saving, setSaving] = useState(false);
  const [title, setTitle] = useState("");
  const waiting = useRef(0);
  const stream = useRef("");
  const streamed = useRef<OperatorStep[]>([]);
  const [live, setLive] = useState<OperatorStep[]>([]);
  const seen = useRef(0);
  const end = useRef<HTMLDivElement>(null);
  const last = turns.at(-1)?.result;
  const { data: caller } = useQuery(callerQuery(workspace));
  const member = caller?.member?.id ?? null;

  useChannel(member ? channels.member(member) : null, {
    onEnvelope: (envelope: Envelope) => {
      const data = (envelope.data ?? {}) as Streamed;
      if (envelope.event !== "procedure.preview.step" || !data.step || data.stream_id !== stream.current) {
        return;
      }
      streamed.current = [...streamed.current, liveStep(data.step, sources)];
      setLive(streamed.current);
    },
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: the newest line is kept in view as the chat grows.
  useEffect(() => {
    end.current?.scrollIntoView({ block: "end" });
  }, [turns.length, asked, live.length]);

  const ask = async (input: AgentPreviewInput): Promise<AgentPreview | string> => {
    try {
      if (as === "draft") {
        await flush();
      }

      return await previewAgent(workspace, agent, {
        version: as,
        persona,
        history: input.source_conversation_id ? undefined : historyOf(turns),
        active_state: last?.active_state,
        stream_id: stream.current,
        ...input,
      });
    } catch (error) {
      return error instanceof Error ? error.message : "The preview did not run.";
    }
  };

  const send = async (input: AgentPreviewInput, shown: string) => {
    waiting.current += 1;
    const turn = waiting.current;
    stream.current = crypto.randomUUID();
    streamed.current = [];
    setLive([]);
    setBusy(true);
    setAsked(shown);
    setMessage("");
    const result = await ask(input);
    if (waiting.current !== turn) {
      return;
    }
    setBusy(false);
    setAsked(null);
    if (typeof result === "string") {
      setMessage(shown);
      toast.error(result);
      return;
    }
    setReplay(null);
    const steps = streamed.current.length > 0 ? streamed.current : eventsOf(result, last, sources);
    seen.current = turns.length;
    setTurns((before) => [...before, { message: shown, at: new Date().toISOString(), result, steps }]);
  };

  const submit = () => {
    if (busy) {
      return;
    }
    if (replay) {
      void send({ source_conversation_id: replay.id }, "");
      return;
    }
    const text = message.trim();
    if (text !== "") {
      void send({ message: text }, text);
    }
  };

  const save = async () => {
    const said = turns.find((t) => t.message)?.message ?? "";
    try {
      await createAgentSimulation(workspace, agent, {
        name: title.trim() || said.slice(0, 60) || "Simulation",
        persona,
        messages: turns.map((t) => (t.message ? { text: t.message } : { text: "", confirm: true })),
        assertions: checksOf(turns.map((t) => t.result)),
        simulated_at: turns[0]?.at ?? new Date().toISOString(),
      });
      void queryClient.invalidateQueries(agentSimulationsQuery(workspace, agent));
      setTitle("");
      toast.success("Saved as a simulation.");
      onSaved();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The simulation was not saved.");
    }
  };

  const stop = () => {
    waiting.current += 1;
    setBusy(false);
    setAsked(null);
  };

  const restart = () => {
    stop();
    seen.current = 0;
    setTurns([]);
  };

  const blank = turns.length === 0 && asked === null;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {turns.length > 0 ? (
        <div className="flex h-10 shrink-0 items-center justify-end gap-1 px-3">
          <Button variant="ghost" size="xs" disabled={busy} onClick={() => setSaving(true)}>
            Save as simulation
          </Button>
          <Button
            variant="ghost"
            size="xs"
            iconOnly
            leadingIcon={RiRestartLine}
            aria-label="Start over"
            onClick={restart}
          />
        </div>
      ) : null}
      <ScrollArea className="flex flex-col">
        <ol
          aria-label="Preview conversation"
          className={`flex flex-1 flex-col gap-5 px-4 pt-4 pb-6 ${blank ? "justify-center" : "justify-end"}`}
        >
          {blank ? <Blank name={name} as={as} /> : null}
          {turns.map((t, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: turns only grow.
            <li key={index} className="flex flex-col gap-5">
              {t.message ? <p className={fromCustomer}>{t.message}</p> : null}
              <div className="flex flex-col gap-3">
                <ActivityLog steps={t.steps} working={false} />
                <Reveal animate={index >= seen.current}>
                  <MessageBody
                    text={t.result.reply || words(t.result.outcome)}
                    className="text-body-regular text-text-primary"
                  />
                </Reveal>
              </div>
            </li>
          ))}
          {asked === null ? null : (
            <li className="flex flex-col gap-5">
              {asked ? (
                <Reveal>
                  <p className={fromCustomer}>{asked}</p>
                </Reveal>
              ) : null}
              <ActivityLog steps={live} working />
            </li>
          )}
          {busy
            ? null
            : last?.active_state.pending.map((p) => (
                <li key={p.action_id} className={`flex flex-col items-start gap-2 p-3 ${card}`}>
                  <span className="text-body-2-medium text-text-primary">{p.summary}</span>
                  <Button
                    variant="secondary"
                    size="small"
                    onClick={() => void send({ confirmed_action_ids: [p.action_id] }, "")}
                  >
                    {p.state === "awaiting_confirmation" ? "Confirm as the customer" : "Approve as a teammate"}
                  </Button>
                </li>
              ))}
        </ol>
        <div ref={end} />
      </ScrollArea>
      <div className="shrink-0 px-4 pb-4">
        <PreviewComposer
          value={message}
          onValueChange={setMessage}
          onSubmit={submit}
          onStop={stop}
          busy={busy}
          placeholder={replay ? "Press send to replay it" : "Write as the customer"}
          canSubmit={replay !== null}
          added={
            <AddedRow
              persona={persona}
              replay={replay}
              onNoCustomer={() => setPersona(madeUp)}
              onNoReplay={() => setReplay(null)}
            />
          }
          menu={
            <PreviewAddMenu
              workspace={workspace}
              replayable={turns.length === 0}
              onConversation={setReplay}
              onCustomer={setPersona}
            />
          }
        />
      </div>
      <AlertDialog
        isOpen={saving}
        onOpenChange={setSaving}
        variant="default"
        icon={false}
        title="Save as simulation"
        description={`${name} must answer this chat the same way every time the simulations run.`}
        confirmLabel="Save"
        onConfirm={save}
      >
        <Input aria-label="Simulation name" placeholder="Name it, or leave empty" value={title} onChange={setTitle} />
      </AlertDialog>
    </div>
  );
}
