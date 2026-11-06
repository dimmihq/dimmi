/*
 * The team step: inviting the people who answer customers. Each address
 * gets a mail with a link to join; nobody has to be invited now.
 */
import { useMutation } from "@tanstack/react-query";
import { useState } from "react";

import { HintText } from "@/components/base/input/hint-text";
import { Textarea } from "@/components/base/textarea/textarea";
import { fieldError } from "@/features/auth/form-errors";
import { sendInvitations } from "@/lib/api/invitations";

import { emailsIn } from "./emails";
import { StepActions, StepHeader } from "./frame";
import type { StepProps } from "./step-props";

export function TeamStep({ workspace, label, onNext, onBack }: StepProps) {
  const [emails, setEmails] = useState("");
  const invite = useMutation({
    mutationFn: () => sendInvitations(workspace, emailsIn(emails), "member"),
    onSuccess: onNext,
  });
  const error = fieldError(invite.error, "invitations", ["invitations"], "invitations");

  return (
    <div>
      <StepHeader
        label={label}
        title="Invite your team"
        description="Each person gets a mail with a link to join this workspace. You can invite more people later."
      />
      <Textarea
        className="mt-8"
        name="emails"
        label="Email addresses"
        placeholder="grace@acme.com, alan@acme.com"
        hint="Separate addresses with commas or new lines."
        rows={3}
        value={emails}
        onChange={(value) => {
          setEmails(value);
          invite.reset();
        }}
        isInvalid={Boolean(error)}
      />
      {error ? (
        <HintText isInvalid role="alert" className="mt-2">
          {error}
        </HintText>
      ) : null}
      <StepActions
        primary={() => invite.mutate()}
        primaryLabel="Send invitations"
        isPending={invite.isPending}
        isDisabled={emailsIn(emails).length === 0}
        skipLabel="Continue without inviting"
        onSkip={onNext}
        onBack={onBack}
      />
    </div>
  );
}
