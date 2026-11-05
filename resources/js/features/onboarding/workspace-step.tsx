/*
 * The first step names the workspace. Someone without a workspace creates
 * it here; a workspace made at sign-up, or one still being set up, is
 * renamed instead, so naming twice never makes two.
 */
import { RiBuildingLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Input } from "@/components/base/input/input";
import { fieldError } from "@/features/auth/form-errors";
import { rememberWorkspace } from "@/features/shell/workspace";
import { meQuery } from "@/lib/api/auth";
import { createWorkspace, updateWorkspace } from "@/lib/api/workspace";

import { StepActions, StepHeader } from "./frame";

export interface WorkspaceStepProps {
  workspace: string | null;
  name: string;
  label: string;
  onSaved: (workspace: string) => void;
  onNamed: (name: string) => void;
}

export function WorkspaceStep({ workspace, name: initial, label, onSaved, onNamed }: WorkspaceStepProps) {
  const queryClient = useQueryClient();
  const [name, setName] = useState(initial);
  const save = useMutation({
    mutationFn: async () => (workspace ? updateWorkspace(workspace, { name }) : createWorkspace(name)),
    onSuccess: async (saved) => {
      rememberWorkspace(saved.id);
      onNamed(saved.name);
      await queryClient.invalidateQueries({ queryKey: meQuery.queryKey });
      onSaved(saved.id);
    },
  });
  const nameError = fieldError(save.error, "name", ["name"], "name");

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    save.mutate();
  };

  return (
    <form onSubmit={submit} aria-busy={save.isPending}>
      <StepHeader
        label={label}
        title="Name your workspace"
        description="Your customers see this name in the messenger and in emails, and your team knows the workspace by it."
      />
      <Input
        className="mt-8"
        name="name"
        label="Workspace name"
        placeholder="Acme, Inc."
        leadingIcon={RiBuildingLine}
        autoFocus
        isRequired
        maxLength={80}
        validationBehavior="aria"
        value={name}
        onChange={(value) => {
          setName(value);
          save.reset();
        }}
        isInvalid={Boolean(nameError)}
        errorMessage={nameError}
      />
      <StepActions primary={() => save.mutate()} isPending={save.isPending} isDisabled={name.trim() === ""} />
    </form>
  );
}
