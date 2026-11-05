/*
 * The person's name, as teammates and customers see it. It saves on the
 * button, so half-typed names never reach anyone.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { fieldError } from "@/features/auth/form-errors";
import { updateProfile } from "@/lib/api/account";

import { Action, Field } from "../controls";
import { Row } from "../layout";

export function NameRow({ name }: { name: string }) {
  const queryClient = useQueryClient();
  const [typed, setTyped] = useState(name);
  const save = useMutation({
    mutationFn: () => updateProfile({ name: typed }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["me"] }),
  });
  const failure = fieldError(save.error, "name", ["name"], "name");
  const cleaned = typed.trim();

  return (
    <Row label="Full name" description={failure ?? (save.isSuccess ? "Saved." : undefined)}>
      <Field
        label="Full name"
        value={name}
        onChange={(next) => {
          setTyped(next);
          save.reset();
        }}
      />
      <Action onPress={cleaned === "" || cleaned === name || save.isPending ? undefined : () => save.mutate()}>
        Save
      </Action>
    </Row>
  );
}
