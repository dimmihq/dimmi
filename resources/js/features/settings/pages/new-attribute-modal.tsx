/*
 * Adding an attribute in a modal opened from the top of the attributes
 * page: who it belongs to, the key code sends it under, how the team
 * reads it, and its type, which never changes once set. A list also
 * names the values it allows. The server's reason shows when it refuses.
 */
import { RiBuilding2Line, RiUserLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Label } from "@/components/base/input/label";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Select, SelectItem } from "@/components/base/select/select";
import {
  type AttributeOf,
  type AttributeType,
  attributeProblemsQuery,
  attributesQuery,
  defineAttribute,
} from "@/lib/api/attributes";

import { typeNames } from "../attributes";

function listOptions(text: string): string[] {
  return text
    .split(",")
    .map((o) => o.trim())
    .filter(Boolean);
}

export function NewAttributeModal({ workspace, onClose }: { workspace: string; onClose: () => void }) {
  const formId = useId();
  const client = useQueryClient();
  const [of, setOf] = useState<AttributeOf>("contact");
  const [key, setKey] = useState("");
  const [label, setLabel] = useState("");
  const [type, setType] = useState<AttributeType>("string");
  const [options, setOptions] = useState("");
  const define = useMutation({
    mutationFn: () =>
      defineAttribute(workspace, {
        object: of,
        key: key.trim(),
        label: label.trim() || key.trim(),
        data_type: type,
        options: type === "list" ? listOptions(options) : undefined,
      }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: attributesQuery(workspace).queryKey });
      void client.invalidateQueries({ queryKey: attributeProblemsQuery(workspace).queryKey });
      onClose();
    },
  });
  const ready = key.trim() !== "" && (type !== "list" || listOptions(options).length > 0);

  return (
    <Modal
      size="md"
      className="h-[min(640px,calc(100dvh-3rem))]"
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
    >
      <ModalHeader
        title="New attribute"
        description="Keys are how your code sends the value: plan, seats, trial_ends_at."
      />
      <ModalBody className="pb-4">
        <form
          id={formId}
          className="flex flex-col gap-4"
          onSubmit={(event) => {
            event.preventDefault();
            if (ready) {
              define.mutate();
            }
          }}
        >
          <div className="flex flex-col gap-1">
            <Label>Belongs to</Label>
            <RadioGroup aria-label="Belongs to" value={of} onChange={(next) => setOf(next as AttributeOf)}>
              <RadioCard value="contact" icon={RiUserLine} title="People" description="A value for each person." />
              <RadioCard
                value="company"
                icon={RiBuilding2Line}
                title="Companies"
                description="A value for each company."
              />
            </RadioGroup>
          </div>
          <Input label="Key" inputDir="ltr" placeholder="plan" autoFocus value={key} onChange={setKey} />
          <Input label="Label" placeholder="Plan" value={label} onChange={setLabel} />
          <Select
            label="Type"
            selectedKey={type}
            onSelectionChange={(next) => next !== null && setType(next as AttributeType)}
          >
            {(Object.entries(typeNames) as [AttributeType, string][]).map(([id, name]) => (
              <SelectItem key={id} id={id} textValue={name}>
                {name}
              </SelectItem>
            ))}
          </Select>
          <p className="-mt-2 text-body-2-regular text-text-tertiary">A type never changes once set.</p>
          {type === "list" ? (
            <Input
              label="Values"
              hint="Separate them with commas."
              placeholder="free, pro, team"
              value={options}
              onChange={setOptions}
            />
          ) : null}
          {define.error ? <p className="text-body-2-regular text-text-error-primary">{define.error.message}</p> : null}
        </form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" form={formId} loading={define.isPending} disabled={!ready}>
          Add attribute
        </Button>
      </ModalFooter>
    </Modal>
  );
}
