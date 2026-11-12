/*
 * Where a step saves what it finds: a variable of the run, which later
 * steps read as {{run.vars.name}}, or an attribute of the contact, kept
 * after the run ends. The kind is picked and the name typed; until both
 * are there, nothing is saved.
 */
import { useState } from "react";

import { Field } from "@/features/settings/controls";

import { choicesOf } from "./choices";
import { Pick } from "./pick";
import { splitTarget, targetKinds } from "./step-settings";

const nowhere = "none";

/*
 * SaveTarget picks a target's kind and types its name, telling onChange
 * the whole target once both are there, and undefined until then.
 */
export function SaveTarget({
  label,
  value,
  nothing,
  onChange,
}: {
  label: string;
  value: string;
  nothing?: string;
  onChange: (target: string | undefined) => void;
}) {
  const [saved, savedName] = splitTarget(value);
  const [kind, setKind] = useState(saved || (nothing ? nowhere : "run.vars."));
  const [name, setName] = useState(savedName);
  const tell = (nextKind: string, nextName: string) => {
    const key = nextName.trim();
    onChange(nextKind !== nowhere && key ? nextKind + key : undefined);
  };
  const choices = nothing ? [[nowhere, nothing] as const, ...choicesOf(targetKinds)] : choicesOf(targetKinds);

  return (
    <span className="flex flex-wrap items-center gap-2">
      <Pick
        label={`${label} kind`}
        choices={choices}
        value={kind}
        onChange={(next) => {
          setKind(next);
          tell(next, name);
        }}
      />
      {kind !== nowhere ? (
        <Field
          label={`${label} name`}
          placeholder="name"
          value={name}
          width="w-32"
          onChange={(next) => {
            setName(next);
            tell(kind, next);
          }}
        />
      ) : null}
    </span>
  );
}
