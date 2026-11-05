/*
 * The tags step: the first words the team sorts conversations by. A few
 * common ones are suggested; the team picks some, adds its own, and saves
 * them all at once.
 */
import { RiAddLine, RiCheckLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { tagTint } from "@/features/inbox/tags/tag-colors";
import { tagIcon } from "@/features/inbox/tags/tag-icons";
import { createTag, type Tag } from "@/lib/api/tags";
import { cx } from "@/utils/cx";

import { StepActions, StepHeader } from "./frame";
import type { StepProps } from "./step-props";

const suggestions: Record<string, { color: string; icon: string }> = {
  Billing: { color: "#16a34a", icon: "bank-card" },
  Bug: { color: "#dc2626", icon: "bug" },
  Feedback: { color: "#9333ea", icon: "chat-smile-2" },
  Question: { color: "#2563eb", icon: "question" },
  Account: { color: "#ea580c", icon: "user" },
  "Feature request": { color: "#0891b2", icon: "sparkling" },
};

/*
 * preview is the chip a chosen name will become. A name the team typed
 * itself gets its color and icon from the server once saved, so it shows
 * neutral until then.
 */
function preview(name: string): Tag {
  const suggested = suggestions[name];

  return { id: name, object: "tag", name, color: suggested?.color ?? null, icon: suggested?.icon ?? null };
}

/*
 * Suggestion is a suggested tag in its own color and icon, so the team
 * sees the tag it will get; pressing it adds or drops the tag.
 */
function Suggestion({ tag, isChosen, onToggle }: { tag: Tag; isChosen: boolean; onToggle: () => void }) {
  const Icon = tagIcon(tag.icon);
  const Mark = isChosen ? RiCheckLine : RiAddLine;
  const tint = tagTint(tag.color);

  return (
    <button
      type="button"
      aria-pressed={isChosen}
      onClick={onToggle}
      style={tint.style}
      className={cx(
        "inline-flex h-7 cursor-pointer items-center gap-1 rounded-lg px-2 text-body-medium outline-none",
        "transition-[filter,box-shadow] duration-150 ease-out hover:brightness-95",
        "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
        tint.className,
        isChosen && "ring-1 ring-current/40 ring-inset",
      )}
    >
      <Icon aria-hidden className="size-4 shrink-0" />
      {tag.name}
      <Mark aria-hidden className="size-4 shrink-0 opacity-70" />
    </button>
  );
}

export function TagsStep({ workspace, label, onNext, onBack }: StepProps) {
  const queryClient = useQueryClient();
  const [chosen, setChosen] = useState<string[]>([]);
  const [draft, setDraft] = useState("");
  const save = useMutation({
    mutationFn: async () => {
      for (const name of chosen) {
        await createTag(workspace, name, suggestions[name] ?? {});
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace] });
      onNext();
    },
  });

  const toggle = (name: string) =>
    setChosen((current) => (current.includes(name) ? current.filter((tag) => tag !== name) : [...current, name]));

  const addOwn = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = draft.trim();
    if (name && !chosen.includes(name)) {
      setChosen((current) => [...current, name]);
    }
    setDraft("");
  };

  return (
    <div>
      <StepHeader
        label={label}
        title="Create your first tags"
        description="Tags sort and filter conversations. Pick a few to start with or add your own. You can change them any time."
      />
      <div className="mt-8 flex flex-col gap-3">
        <h2 className="text-body-semibold text-text-primary">Suggestions</h2>
        <div className="flex flex-wrap gap-2">
          {Object.keys(suggestions).map((name) => {
            const isChosen = chosen.includes(name);

            return <Suggestion key={name} tag={preview(name)} isChosen={isChosen} onToggle={() => toggle(name)} />;
          })}
        </div>
      </div>
      <div className="mt-8 flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h2 className="text-body-semibold text-text-primary">Your tags</h2>
          <span className="text-body-2-regular text-text-tertiary">{chosen.length} chosen</span>
        </div>
        <form onSubmit={addOwn} className="flex items-end gap-2">
          <Input
            className="flex-1"
            aria-label="Add your own tag"
            placeholder="Add your own…"
            maxLength={50}
            value={draft}
            onChange={setDraft}
          />
          <Button size="small" type="submit" variant="secondary" leadingIcon={RiAddLine} disabled={draft.trim() === ""}>
            Add
          </Button>
        </form>
        {chosen.length > 0 ? (
          <ul className="flex flex-wrap gap-2">
            {chosen.map((name) => (
              <li key={name}>
                <TagChip tag={preview(name)} onRemove={() => toggle(name)} />
              </li>
            ))}
          </ul>
        ) : null}
      </div>
      <StepActions
        primary={() => save.mutate()}
        isPending={save.isPending}
        isDisabled={chosen.length === 0}
        skipLabel="Continue without tags"
        onSkip={onNext}
        onBack={onBack}
      />
    </div>
  );
}
