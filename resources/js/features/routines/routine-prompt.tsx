/*
 * A routine's instructions, written in the shared prompt writer. Typing
 * @ offers Operator's tools, grouped, the ones the routine has first;
 * picking one from a group the routine lacks gives it that group.
 */
import { RiFlashlightLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { Section } from "@/features/settings/layout";
import { PromptWriter } from "@/features/writer/prompt-writer";
import { routineToolsQuery } from "@/lib/api/routines";

import type { RoutineDraft } from "./draft";
import { ToolToken } from "./tool-token";
import { findTools, type ToolOption, withNamedGroups } from "./tools";

const variables = ["{{workspace_name}}", "{{routine_name}}", "{{window_start}}", "{{window_end}}", "{{last_run_at}}"];

function ToolRow({ option }: { option: ToolOption }) {
  return (
    <span className="flex min-w-0 flex-1 flex-col">
      {option.heading ? (
        <span className="pb-1 text-caption-1-semibold text-text-tertiary uppercase">{option.heading}</span>
      ) : null}
      <span className="flex min-w-0 items-center gap-2">
        <RiFlashlightLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-body-medium text-text-primary">{option.label}</span>
          <span className="truncate text-body-2-regular text-text-tertiary">{option.hint}</span>
        </span>
      </span>
    </span>
  );
}

export function RoutinePrompt({
  workspace,
  draft,
  onChange,
}: {
  workspace: string;
  draft: RoutineDraft;
  onChange: (next: RoutineDraft) => void;
}) {
  const { data: tools } = useQuery(routineToolsQuery(workspace));
  const all = tools ?? [];

  return (
    <Section
      title="Instructions"
      description="Operator follows these on every run, reading what changed since the run before. Type @ to call a tool, / for headings and lists."
    >
      <div className="flex flex-col gap-2 py-3 pe-3">
        <PromptWriter
          markdown={draft.prompt}
          label="Instructions"
          chips={{
            node: ToolToken,
            label: "Tools",
            find: (query) => findTools(all, draft.toolGroups, query),
            insert: (editor, range, option) =>
              editor
                .chain()
                .focus()
                .deleteRange(range)
                .insertContent([
                  { type: "routineTool", attrs: { name: option.tool.name } },
                  { type: "text", text: " " },
                ])
                .run(),
            keyOf: (option) => option.tool.name,
            render: (option) => <ToolRow option={option} />,
          }}
          onChange={(prompt) => {
            const toolGroups = withNamedGroups(draft.toolGroups, prompt, all);
            const github =
              toolGroups.includes("github") && !draft.github
                ? { installationId: "", repositories: [], baseBranch: "" }
                : draft.github;
            onChange({ ...draft, prompt, toolGroups, github });
          }}
        />
        <p className="text-body-2-regular text-text-tertiary">
          Filled in on each run: <span className="font-mono">{variables.join(" ")}</span>
        </p>
      </div>
    </Section>
  );
}
