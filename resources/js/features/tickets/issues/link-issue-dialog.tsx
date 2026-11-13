/*
 * Linking engineering work to a ticket, in two steps. First the tool,
 * picked from the workspace's connected trackers and incident tools
 * (skipped when there is only one). Then either a new issue, filed in a
 * team and project picked from lists pulled from the tool (an incident
 * tool asks for a severity, PagerDuty for the service to page), or an
 * existing one found by search or pasted as a URL or key. Nothing is
 * typed as an id.
 */
import { RiArrowLeftLine, RiCheckLine, RiSearchLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { StepMotion } from "@/features/knowledge/step-motion";
import type { Integration } from "@/lib/api/integrations";
import {
  type IssueFound,
  type IssueList,
  type IssueSettings,
  issueOptionsQuery,
  issueSearchQuery,
  issueSettingsQuery,
  linkTicketIssue,
  type NewIssue,
  refreshTicketKeys,
} from "@/lib/api/ticket-issues";
import { cx } from "@/utils/cx";

import { groupTint, toolLook } from "./tools";

type Mode = "create" | "link";

/*
 * Picker is a select fed by one of the tool's lists.
 */
function Picker({
  workspace,
  installation,
  list,
  parent = "",
  label,
  value,
  optional = false,
  onChange,
}: {
  workspace: string;
  installation: string;
  list: IssueList;
  parent?: string;
  label: string;
  value: string;
  optional?: boolean;
  onChange: (id: string) => void;
}) {
  const { data, isPending, isError } = useQuery(issueOptionsQuery(workspace, installation, list, parent));
  const options = data ?? [];

  return (
    <Select
      label={label}
      className="w-full"
      placeholder={isPending ? "Loading…" : isError ? "Could not load the list" : optional ? "None" : "Pick one"}
      isDisabled={isPending || isError}
      selectedKey={value || null}
      onSelectionChange={(key) => onChange(key ? String(key) : "")}
    >
      {options.map((o) => (
        <SelectItem key={o.id} id={o.id} textValue={o.name}>
          {o.name}
          {o.detail ? <span className="ms-2 text-text-tertiary">{o.detail}</span> : null}
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * CreateFields are the pickers a new issue or incident needs in the
 * tool.
 */
function CreateFields({
  workspace,
  tool,
  issue,
  onChange,
}: {
  workspace: string;
  tool: Integration;
  issue: NewIssue;
  onChange: (issue: NewIssue) => void;
}) {
  const look = toolLook(tool.provider);

  if (look.kind === "incidents") {
    return (
      <div className="flex flex-col gap-4">
        {tool.provider === "pagerduty" ? (
          <Picker
            workspace={workspace}
            installation={tool.id}
            list="services"
            label="Service to page"
            value={issue.service_id ?? ""}
            onChange={(id) => onChange({ ...issue, service_id: id })}
          />
        ) : null}
        <Picker
          workspace={workspace}
          installation={tool.id}
          list="severities"
          label="Severity"
          value={issue.severity_id ?? ""}
          optional
          onChange={(id) => onChange({ ...issue, severity_id: id })}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <Picker
        workspace={workspace}
        installation={tool.id}
        list="teams"
        label={look.team ?? "Team"}
        value={issue.team_id ?? ""}
        onChange={(id) => onChange({ ...issue, team_id: id, project_id: "" })}
      />
      {look.project && issue.team_id ? (
        <Picker
          workspace={workspace}
          installation={tool.id}
          list="projects"
          parent={issue.team_id}
          label={look.project}
          value={issue.project_id ?? ""}
          optional={tool.provider === "linear"}
          onChange={(id) => onChange({ ...issue, project_id: id })}
        />
      ) : null}
    </div>
  );
}

/*
 * LinkFields find an existing issue by search, or take a pasted URL.
 */
function LinkFields({
  workspace,
  tool,
  picked,
  onPick,
}: {
  workspace: string;
  tool: Integration;
  picked: string;
  onPick: (ref: string) => void;
}) {
  const [q, setQ] = useState("");
  const pasted = /^https?:\/\//.test(q.trim());
  const { data, isFetching } = useQuery({
    ...issueSearchQuery(workspace, tool.id, q.trim()),
    enabled: !pasted && q.trim().length > 1,
  });
  const found: IssueFound[] = data ?? [];

  return (
    <div className="flex flex-col gap-3">
      <Input
        label="Find the issue"
        placeholder={toolLook(tool.provider).search}
        leadingIcon={RiSearchLine}
        value={q}
        onChange={(value) => {
          setQ(value);
          onPick(/^https?:\/\//.test(value.trim()) ? value.trim() : "");
        }}
      />
      {pasted ? (
        <p className="text-caption-1-regular text-text-tertiary">dimmi reads this issue from the link.</p>
      ) : null}
      {!pasted && q.trim().length > 1 && !isFetching && found.length === 0 ? (
        <p className="text-body-2-regular text-text-tertiary">Nothing matches. Try other words or paste the link.</p>
      ) : null}
      <ul className="flex flex-col divide-y divide-separator-border overflow-hidden rounded-xl border border-border-primary">
        {found.map((issue) => {
          const ref = issue.url || issue.key;
          const selected = picked === ref;

          return (
            <li key={issue.external_id}>
              <button
                type="button"
                aria-pressed={selected}
                onClick={() => onPick(ref)}
                className={cx(
                  "flex w-full items-center gap-3 px-3 py-2.5 text-start outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring",
                  selected ? "bg-background-secondary-hover" : "hover:bg-background-secondary-default",
                )}
              >
                <span className="shrink-0 text-body-2-medium text-text-secondary">{issue.key}</span>
                <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{issue.title}</span>
                <Chip variant="caption" color={groupTint[issue.status_group] ?? "gray"}>
                  {issue.status}
                </Chip>
                {selected ? <RiCheckLine aria-hidden className="size-4 text-foreground-icon-primary" /> : null}
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/*
 * modeLabels name the two ways of linking for the tool.
 */
function modeLabels(tool: Integration): { id: Mode; label: string }[] {
  const look = toolLook(tool.provider);
  const create = look.kind === "incidents" ? `Declare in ${look.label}` : `New ${look.label} issue`;

  return [
    { id: "create", label: create },
    { id: "link", label: "Link an existing one" },
  ];
}

/*
 * isReady says whether the details are enough to send.
 */
function isReady(tool: Integration | undefined, mode: Mode, issue: NewIssue, ref: string): boolean {
  if (!tool) {
    return false;
  }
  if (mode === "link") {
    return ref !== "";
  }
  if (toolLook(tool.provider).kind === "incidents") {
    return tool.provider !== "pagerduty" || !!issue.service_id;
  }

  return !!issue.team_id && (issue.title ?? "").trim() !== "";
}

/*
 * withDefaults fills the details the person has not picked yet with the
 * tool's defaults from its settings: the team, project, service and
 * severity.
 */
function withDefaults(issue: NewIssue, settings: IssueSettings): NewIssue {
  const pick = (current: string | undefined, fallback: string) => (current ? current : fallback || undefined);

  return {
    ...issue,
    team_id: pick(issue.team_id, settings.team_id),
    project_id: issue.team_id ? issue.project_id : pick(issue.project_id, settings.project_id),
    service_id: pick(issue.service_id, settings.service_id),
    severity_id: pick(issue.severity_id, settings.severity_id),
  };
}

/*
 * Details is the second step: create in the tool, or link existing work.
 */
function Details({
  workspace,
  tool,
  mode,
  issue,
  picked,
  onMode,
  onIssue,
  onPick,
}: {
  workspace: string;
  tool: Integration;
  mode: Mode;
  issue: NewIssue;
  picked: string;
  onMode: (mode: Mode) => void;
  onIssue: (issue: NewIssue) => void;
  onPick: (ref: string) => void;
}) {
  return (
    <>
      <SegmentedControl
        aria-label="Create or link"
        selectedKeys={[mode]}
        onSelectionChange={(keys) => {
          const next = [...keys][0];
          if (next) {
            onMode(next as Mode);
          }
        }}
      >
        {modeLabels(tool).map((m) => (
          <SegmentedControlItem key={m.id} id={m.id}>
            {m.label}
          </SegmentedControlItem>
        ))}
      </SegmentedControl>
      {mode === "create" ? (
        <>
          <Input label="Title" value={issue.title ?? ""} onChange={(value) => onIssue({ ...issue, title: value })} />
          <CreateFields workspace={workspace} tool={tool} issue={issue} onChange={onIssue} />
          <p className="text-caption-1-regular text-text-tertiary">
            The description lists the customers, companies and revenue this reaches, with a link back here, and stays
            current as conversations are linked.
          </p>
        </>
      ) : (
        <LinkFields workspace={workspace} tool={tool} picked={picked} onPick={onPick} />
      )}
    </>
  );
}

export function LinkIssueDialog({
  workspace,
  ticket,
  title,
  tools,
  onClose,
}: {
  workspace: string;
  ticket: string;
  title: string;
  tools: Integration[];
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [toolId, setToolId] = useState(tools.length === 1 ? tools[0].id : "");
  const [step, setStep] = useState<"tool" | "details">(tools.length === 1 ? "details" : "tool");
  const [mode, setMode] = useState<Mode>("create");
  const [issue, setIssue] = useState<NewIssue>({ title });
  const [ref, setRef] = useState("");
  const [seeded, setSeeded] = useState("");
  const tool = tools.find((t) => t.id === toolId);
  const { data: settings } = useQuery({ ...issueSettingsQuery(workspace, toolId), enabled: toolId !== "" });

  if (settings && settings.installation_id === toolId && seeded !== toolId) {
    setSeeded(toolId);
    setIssue((current) => withDefaults(current, settings));
  }

  const look = tool ? toolLook(tool.provider) : undefined;
  const incidents = look?.kind === "incidents";

  const link = useMutation({
    mutationFn: () => {
      if (!tool) {
        throw new Error("Pick a tool first.");
      }
      const body: NewIssue =
        mode === "link" ? { installation_id: tool.id, url: ref } : { ...issue, installation_id: tool.id };

      return linkTicketIssue(workspace, ticket, body);
    },
    onSuccess: async (made) => {
      await Promise.all(
        refreshTicketKeys(workspace, ticket).map((queryKey) => queryClient.invalidateQueries({ queryKey })),
      );
      toast.success(`${look?.label ?? "The issue"} ${made.key} is linked.`);
      onClose();
    },
    onError: (error) => toast.error(error.message),
  });

  function pickTool(id: string) {
    setToolId(id);
    setIssue((current) => ({ title: current.title }));
    setSeeded("");
  }

  const submit = mode === "link" ? "Link" : incidents ? "Declare" : "Create issue";

  return (
    <Modal size="wide" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={incidents ? "Link an incident" : "Link engineering work"}
        description="The ticket stays dimmi's: customers see its states, never the tool's."
      />
      <ModalBody className="min-h-[22rem]">
        <StepMotion view={step} forward={step === "details"} className="flex flex-col gap-4">
          {step === "tool" ? (
            <RadioGroup aria-label="Tool" value={toolId} onChange={pickTool} className="flex flex-col gap-2">
              {tools.map((t) => (
                <RadioCard
                  key={t.id}
                  value={t.id}
                  title={toolLook(t.provider).label}
                  description={`${t.name} · ${toolLook(t.provider).kind === "incidents" ? "Incidents" : "Issues"}`}
                />
              ))}
            </RadioGroup>
          ) : null}
          {step === "details" && tool ? (
            <Details
              workspace={workspace}
              tool={tool}
              mode={mode}
              issue={issue}
              picked={ref}
              onMode={setMode}
              onIssue={setIssue}
              onPick={setRef}
            />
          ) : null}
        </StepMotion>
      </ModalBody>
      <ModalFooter>
        {step === "details" && tools.length > 1 ? (
          <Button variant="secondary" leadingIcon={RiArrowLeftLine} onClick={() => setStep("tool")}>
            Back
          </Button>
        ) : (
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
        )}
        {step === "tool" ? (
          <Button disabled={!toolId} onClick={() => setStep("details")}>
            Continue
          </Button>
        ) : (
          <Button disabled={!isReady(tool, mode, issue, ref)} loading={link.isPending} onClick={() => link.mutate()}>
            {submit}
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}
