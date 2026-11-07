/*
 * The basics of an action: the title teammates and customers see, the
 * tool name the agent calls (made from the title until someone types
 * their own), the description the agent reads to decide when to use it,
 * and whether it only reads or changes data. Changing data moves the
 * permissions to the safe default of the customer confirming first. A
 * tool synced from an MCP server stays marked as changing data until the
 * admin confirms it only reads, since servers can mislabel their tools.
 */
import { RiEyeLine, RiPencilLine } from "@remixicon/react";
import { useState } from "react";

import { Input } from "@/components/base/input/input";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Textarea } from "@/components/base/textarea/textarea";
import { Section } from "@/features/settings/layout";

import type { SideEffect } from "../api";
import { withSideEffect } from "../draft";
import { Block } from "../parts";
import { toSnake } from "../schema";
import type { Editor } from "./use-editor";

export function BasicsSection({ editor, fromMcp }: { editor: Editor; fromMcp: boolean }) {
  const { draft, update, problems } = editor;
  const [named, setNamed] = useState(draft.name !== "");

  return (
    <Section title="Basics" description="What the action is called and when dimmi should use it.">
      <Block label="Title" hint="Shown to teammates in the inbox and to customers on confirmation cards.">
        <Input
          aria-label="Title"
          placeholder="Refund an invoice"
          value={draft.title}
          onChange={(title) => update(named ? { title } : { title, name: toSnake(title) })}
          isInvalid={Boolean(problems.title)}
          errorMessage={problems.title}
        />
      </Block>
      <Block label="Tool name" hint="What the agent calls. Lowercase letters, digits and underscores.">
        <Input
          aria-label="Tool name"
          placeholder="refund_invoice"
          inputDir="ltr"
          value={draft.name}
          onChange={(name) => {
            setNamed(true);
            update({ name });
          }}
          isInvalid={Boolean(problems.name)}
          errorMessage={problems.name}
          fieldClassName="font-mono"
        />
      </Block>
      <Block
        label="Instructions for the agent"
        hint="When to use it, what it needs and what it returns. dimmi reads this to decide, so be specific."
        problem={problems.description}
      >
        <Textarea
          aria-label="Instructions for the agent"
          placeholder="Refund one of the signed-in customer's invoices. Use only when they ask for a refund of an annual plan paid in the last 30 days. Needs the invoice id; returns the refund id and amount."
          rows={3}
          autoResize
          maxRows={10}
          maxLength={1024}
          showCount
          value={draft.description}
          onChange={(description) => update({ description })}
          isInvalid={Boolean(problems.description)}
        />
      </Block>
      <Block
        label="What it does"
        hint={
          fromMcp && draft.side_effect === "write"
            ? "Synced from an MCP server. dimmi treats every tool as changing data until you confirm it only reads."
            : "Actions that change data always run with the permissions below."
        }
      >
        <RadioGroup
          aria-label="What it does"
          value={draft.side_effect}
          onChange={(side) => update((before) => withSideEffect(before, side as SideEffect))}
          className="grid gap-2 sm:grid-cols-2"
        >
          <RadioCard
            value="read"
            icon={RiEyeLine}
            title="Only reads data"
            description="Looks something up. Runs right away."
          />
          <RadioCard
            value="write"
            icon={RiPencilLine}
            title="Changes data"
            description="Refunds, cancels, updates. Asks first."
          />
        </RadioGroup>
      </Block>
    </Section>
  );
}
