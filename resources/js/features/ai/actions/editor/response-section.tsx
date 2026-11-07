/*
 * What of the API's answer the agent gets to read. Only the listed
 * fields reach the model, which keeps other customers' data, internal
 * notes and secrets out of the conversation; an empty list hands over
 * the whole answer and says so. After a test run, the paths of the real
 * answer are offered as one-click fields.
 */
import { RiAddLine, RiErrorWarningLine } from "@remixicon/react";

import { Section } from "@/features/settings/layout";

import { rowKey } from "../draft";
import { Block, PairRows } from "../parts";
import { fieldNameOf, responsePaths } from "../schema";
import type { Editor } from "./use-editor";

export function ResponseSection({ editor, sample }: { editor: Editor; sample: unknown }) {
  const { draft, update } = editor;
  const taken = new Set(draft.response.map((row) => row.value));
  const offered = sample === undefined ? [] : responsePaths(sample).filter((path) => !taken.has(path));

  return (
    <Section title="Response" description="The fields of the answer the agent reads. Everything else stays out.">
      <Block label="Fields to keep" hint="A name the agent reads, and where it sits in the answer, like data.0.status.">
        <PairRows
          label="Response field"
          rows={draft.response}
          onChange={(response) => update({ response })}
          namePlaceholder="status"
          valuePlaceholder="subscription.status"
          addLabel="Add field"
        />
        {draft.response.length === 0 ? (
          <p className="flex items-start gap-2 rounded-xl bg-status-yellow-background p-3 text-body-2-regular text-status-yellow-text">
            <RiErrorWarningLine aria-hidden className="mt-0.5 size-4 shrink-0" />
            Without fields, the agent reads the whole answer. Pick fields to keep data it does not need out of the
            conversation.
          </p>
        ) : null}
      </Block>
      {offered.length > 0 ? (
        <Block label="From the last test" hint="Click a path to keep it.">
          <ul className="flex flex-wrap gap-1.5">
            {offered.slice(0, 24).map((path) => (
              <li key={path}>
                <button
                  type="button"
                  dir="ltr"
                  onClick={() =>
                    update({ response: [...draft.response, { key: rowKey(), name: fieldNameOf(path), value: path }] })
                  }
                  className="flex cursor-pointer items-center gap-1 rounded-md bg-background-tertiary-default px-1.5 py-0.5 font-mono text-caption-1-medium text-text-secondary outline-none transition-colors duration-150 hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                >
                  <RiAddLine aria-hidden className="size-3" />
                  {path}
                </button>
              </li>
            ))}
          </ul>
        </Block>
      ) : null}
    </Section>
  );
}
