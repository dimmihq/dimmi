/*
 * The tags step: the first words the team sorts conversations by. A few
 * common ones are suggested; the team picks some, adds its own, and saves
 * them all at once.
 */
import { RiAddLine, RiCheckLine, RiCloseLine, RiPriceTag3Line } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { createTag } from "@/lib/api/tags";

import { StepActions, StepHeader } from "./frame";
import type { StepProps } from "./step-props";

const suggestions = ["Billing", "Bug", "Feedback", "Question", "Account", "Feature request"];

export function TagsStep({ workspace, label, onNext, onBack }: StepProps) {
  const queryClient = useQueryClient();
  const [chosen, setChosen] = useState<string[]>([]);
  const [draft, setDraft] = useState("");
  const save = useMutation({
    mutationFn: async () => {
      for (const name of chosen) {
        await createTag(workspace, name);
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
          {suggestions.map((name) => {
            const isChosen = chosen.includes(name);

            return (
              <Button
                key={name}
                variant="secondary"
                size="small"
                leadingIcon={RiPriceTag3Line}
                trailingIcon={isChosen ? RiCheckLine : RiAddLine}
                aria-pressed={isChosen}
                onClick={() => toggle(name)}
              >
                {name}
              </Button>
            );
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
                <Button variant="secondary" size="xs" trailingIcon={RiCloseLine} onClick={() => toggle(name)}>
                  {name}
                </Button>
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
