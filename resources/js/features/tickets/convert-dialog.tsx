/*
 * Turning a conversation into a ticket, opened from its header or with
 * T. First the teammate picks one of the customer types on radio cards;
 * then the AI reads the conversation and fills what it can, each value
 * marked as the AI's, and the teammate checks the title and fields, the
 * required ones marked, before it is made. The two steps slide into each
 * other and the dialog eases to each one's height.
 */
import { RiArrowLeftLine, RiSparkling2Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { toast } from "@/components/base/toast/toast";
import { Resize, Swap } from "@/features/inbox/motion";
import { convertConversation, ticketKeys, ticketSuggestions, ticketTypesQuery } from "@/lib/api/tickets";

import { FieldInput } from "./field-input";
import { missingFor, shownFields } from "./field-rules";
import { typeIcon } from "./labels";
import { missingKeys } from "./use-ticket-save";

export function ConvertDialog({
  workspace,
  conversation,
  subject,
  onClose,
}: {
  workspace: string;
  conversation: string;
  subject: string;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const { data: types = [] } = useQuery(ticketTypesQuery(workspace));
  const customer = types.filter((t) => t.kind === "customer" && !t.archived_at);
  const [typeId, setTypeId] = useState<string | null>(null);
  const [step, setStep] = useState<"type" | "fields">("type");
  const [title, setTitle] = useState(subject);
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [errors, setErrors] = useState<string[]>([]);
  const chosen = customer.find((t) => t.id === (typeId ?? customer[0]?.id));
  const suggested = useQuery({
    queryKey: ["workspace", workspace, "tickets", "suggestions", conversation, chosen?.id],
    queryFn: () => ticketSuggestions(workspace, conversation, chosen?.id ?? ""),
    enabled: step === "fields" && chosen !== undefined,
    staleTime: Number.POSITIVE_INFINITY,
    retry: false,
  });
  const suggestions = suggested.data ?? {};
  const merged = { ...suggestions, ...values };
  const fields = shownFields(chosen?.fields ?? [], merged);
  const missing = missingFor(chosen?.fields ?? [], merged, "create");
  const convert = useMutation({
    mutationFn: () =>
      convertConversation(workspace, conversation, {
        type_id: chosen?.id ?? "",
        title: title.trim() || undefined,
        fields: Object.fromEntries(Object.entries(merged).filter(([, v]) => v !== null && v !== undefined)),
      }),
    onSuccess: async (ticket) => {
      await client.invalidateQueries({ queryKey: ticketKeys.all(workspace) });
      toast.success(`${ticket.reference} created`);
      onClose();
    },
    onError: (error) => {
      setErrors(missingKeys(error));
      toast.error((error as Error).message);
    },
  });

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={step === "type" ? "Convert to ticket" : (chosen?.name ?? "Ticket")}
        description={
          step === "type"
            ? "Pick what kind of request this is. The customer sees its state from now on."
            : "The AI filled what it could from the conversation. Check it before you create the ticket."
        }
      />
      <ModalBody className="pb-4">
        <Resize>
          <Swap view={step} forward={step === "fields"}>
            {step === "type" ? (
              <RadioGroup
                aria-label="Ticket type"
                value={chosen?.id ?? null}
                onChange={setTypeId}
                className="flex flex-col gap-2"
              >
                {customer.map((t) => (
                  <RadioCard
                    key={t.id}
                    value={t.id}
                    title={t.name}
                    description={t.description || undefined}
                    icon={typeIcon(t.icon, t.kind)}
                  />
                ))}
              </RadioGroup>
            ) : (
              <div className="flex flex-col gap-3">
                <Input label="Title" size="small" value={title} onChange={setTitle} maxLength={200} isRequired />
                {suggested.isFetching ? (
                  <span className="flex items-center gap-1.5 text-body-2-regular text-text-secondary">
                    <RiSparkling2Line aria-hidden className="size-4" /> Reading the conversation…
                  </span>
                ) : null}
                {fields.map((field) => (
                  <FieldInput
                    key={field.id}
                    workspace={workspace}
                    field={field}
                    purpose="create"
                    value={merged[field.key]}
                    suggestion={values[field.key] === undefined ? undefined : suggestions[field.key]}
                    error={errors.includes(field.key) ? "Fill this in first." : undefined}
                    onChange={(value) => setValues((was) => ({ ...was, [field.key]: value }))}
                    onAccept={() => setValues(({ [field.key]: _, ...rest }) => rest)}
                  />
                ))}
              </div>
            )}
          </Swap>
        </Resize>
      </ModalBody>
      <ModalFooter>
        {step === "type" ? (
          <>
            <Button variant="secondary" size="small" onClick={onClose}>
              Cancel
            </Button>
            <Button size="small" disabled={!chosen} onClick={() => setStep("fields")}>
              Continue
            </Button>
          </>
        ) : (
          <>
            <Button variant="secondary" size="small" leadingIcon={RiArrowLeftLine} onClick={() => setStep("type")}>
              Back
            </Button>
            <Button
              size="small"
              loading={convert.isPending}
              disabled={missing.length > 0 || title.trim() === ""}
              onClick={() => convert.mutate()}
            >
              Create ticket
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}
