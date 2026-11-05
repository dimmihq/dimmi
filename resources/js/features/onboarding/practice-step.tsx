/*
 * The last step offers five example conversations from made-up
 * customers, each with a different kind of request, so the team tries
 * replying, tagging and closing before a real customer writes. They are
 * listed one row each, the way the channels step lists chat apps.
 */
import { useMutation } from "@tanstack/react-query";

import { Avatar, type AvatarColor } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { createTestConversations } from "@/lib/api/workspace";

import { StepActions, StepHeader } from "./frame";
import type { StepProps } from "./step-props";

const customers: { name: string; subject: string; color: AvatarColor }[] = [
  { name: "Emily Johnson", subject: "Can't sign in", color: "pink" },
  { name: "David Smith", subject: "API requests failing with 401", color: "blue" },
  { name: "Maria Garcia", subject: "Changing my username", color: "lime" },
  { name: "James Wilson", subject: "Charged twice this month", color: "neutral" },
  { name: "Sophia Martinez", subject: "Upgrading our plan", color: "blue" },
];

export function PracticeStep({ workspace, label, onNext, onBack, onCreated }: StepProps & { onCreated: () => void }) {
  const create = useMutation({
    mutationFn: () => createTestConversations(workspace),
    onSuccess: () => {
      onCreated();
      onNext();
    },
  });

  return (
    <div>
      <StepHeader
        label={label}
        title="Try dimmi with example conversations"
        description="Five made-up customers write in, each with a different kind of request, so you can practice replying, tagging and closing before your first real conversation."
      />
      <ul className="mt-8 flex flex-col gap-2">
        {customers.map((customer) => (
          <li
            key={customer.name}
            className="flex min-h-[52px] items-center gap-3 rounded-2xl bg-background-secondary-default px-3 py-2.5"
          >
            <Avatar size="xs" color={customer.color} initials={customer.name.charAt(0)} />
            <span className="w-32 shrink-0 truncate text-body-2-medium text-text-primary">{customer.name}</span>
            <Chip color="yellow" variant="caption">
              Example
            </Chip>
            <span className="min-w-0 flex-1 truncate text-body-2-regular text-text-secondary">{customer.subject}</span>
          </li>
        ))}
      </ul>
      <p className="mt-3 text-body-2-regular text-text-tertiary">Example conversations never reach a real customer.</p>
      <StepActions
        primary={() => create.mutate()}
        primaryLabel="Add 5 example conversations"
        isPending={create.isPending}
        skipLabel="Continue without example conversations"
        onSkip={onNext}
        onBack={onBack}
      />
    </div>
  );
}
