/*
 * The dialogs of the feedback settings: a board's name, who sees it and
 * whether it holds posts for approval; a status's name, color, category
 * and place on the roadmap; and removing either, which first asks where
 * its requests go.
 */
import { useId, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import {
  type BoardChange,
  type BoardVisibility,
  categories,
  type FeedbackBoard,
  type FeedbackStatus,
  type StatusCategory,
  visibilities,
} from "@/lib/api/feedback";

import { Colour } from "./controls";

/*
 * BoardDialog adds a board, or changes the one given.
 */
export function BoardDialog({
  board,
  onSave,
  onClose,
}: {
  board: FeedbackBoard | null;
  onSave: (change: BoardChange) => Promise<unknown>;
  onClose: () => void;
}) {
  const formId = useId();
  const [name, setName] = useState(board?.name ?? "");
  const [description, setDescription] = useState(board?.description ?? "");
  const [visibility, setVisibility] = useState<BoardVisibility>(board?.visibility ?? "public");
  const [approval, setApproval] = useState(board?.approval_required ?? false);
  const [saving, setSaving] = useState(false);
  const submit = async () => {
    setSaving(true);
    try {
      await onSave({ name: name.trim(), description, visibility, approval_required: approval });
      onClose();
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={board ? `Edit ${board.name}` : "New board"}
        description="Boards split requests on your portal, such as feature requests and bugs."
      />
      <ModalBody className="flex flex-col gap-4 pb-4">
        <form
          id={formId}
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
          className="flex flex-col gap-4"
        >
          <Input label="Name" value={name} onChange={setName} maxLength={60} isRequired autoFocus />
          <Textarea label="Description" value={description} onChange={setDescription} rows={2} maxLength={500} />
        </form>
        <span className="-mb-2 text-body-2-medium text-text-secondary">Who sees it</span>
        <RadioGroup
          aria-label="Who sees it"
          value={visibility}
          onChange={(next) => setVisibility(next as BoardVisibility)}
          className="flex flex-col gap-2"
        >
          {visibilities.map((v) => (
            <RadioCard key={v.id} value={v.id} title={v.name} description={v.description} />
          ))}
        </RadioGroup>
        <Switch size="sm" isSelected={approval} onChange={setApproval} isDisabled={visibility === "internal"}>
          Approve customers' posts before they show
        </Switch>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button size="small" type="submit" form={formId} loading={saving} disabled={name.trim() === ""}>
          {board ? "Save board" : "Add board"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * StatusDialog adds a status, or changes the one given.
 */
export function StatusDialog({
  status,
  onSave,
  onClose,
}: {
  status: FeedbackStatus | null;
  onSave: (change: { name: string; color: string; category: StatusCategory; roadmap: boolean }) => Promise<unknown>;
  onClose: () => void;
}) {
  const formId = useId();
  const [name, setName] = useState(status?.name ?? "");
  const [color, setColor] = useState(status?.color ?? "#8b5cf6");
  const [category, setCategory] = useState<StatusCategory>(status?.category ?? "open");
  const [roadmap, setRoadmap] = useState(status?.roadmap ?? true);
  const [saving, setSaving] = useState(false);
  const submit = async () => {
    setSaving(true);
    try {
      await onSave({ name: name.trim(), color, category, roadmap: category !== "closed" && roadmap });
      onClose();
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={status ? `Edit ${status.name}` : "New status"}
        description="Requests keep their status when you rename or recolor it."
      />
      <ModalBody className="flex flex-col gap-4 pb-4">
        <form
          id={formId}
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
          className="flex items-end gap-3"
        >
          <Input label="Name" value={name} onChange={setName} maxLength={40} isRequired autoFocus className="flex-1" />
          <span className="flex h-10 items-center">
            <Colour label="Status color" value={color} onChange={setColor} />
          </span>
        </form>
        <span className="-mb-2 text-body-2-medium text-text-secondary">What it means</span>
        <RadioGroup
          aria-label="What it means"
          value={category}
          onChange={(next) => setCategory(next as StatusCategory)}
          className="flex flex-col gap-2"
        >
          {categories.map((c) => (
            <RadioCard key={c.id} value={c.id} title={c.name} description={c.description} />
          ))}
        </RadioGroup>
        <Switch
          size="sm"
          isSelected={category !== "closed" && roadmap}
          onChange={setRoadmap}
          isDisabled={category === "closed"}
        >
          Show as a column on the public roadmap
        </Switch>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button size="small" type="submit" form={formId} loading={saving} disabled={name.trim() === ""}>
          {status ? "Save status" : "Add status"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * MoveAndDelete removes a board or a status after asking which of the
 * others its requests move to.
 */
export function MoveAndDelete({
  noun,
  name,
  count,
  options,
  onConfirm,
  onClose,
}: {
  noun: "board" | "status";
  name: string;
  count?: number;
  options: { id: string; name: string }[];
  onConfirm: (target: string) => Promise<unknown>;
  onClose: () => void;
}) {
  const [target, setTarget] = useState(options[0]?.id ?? "");
  const requests = count === undefined ? "Its requests" : count === 1 ? "Its 1 request" : `Its ${count} requests`;

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      title={`Delete ${name}?`}
      description={`${requests} move to the ${noun} you pick. Nobody is emailed.`}
      confirmLabel={`Delete ${noun}`}
      isConfirmDisabled={target === ""}
      onConfirm={() => onConfirm(target)}
    >
      <Select label="Move requests to" selectedKey={target} onSelectionChange={(key) => setTarget(String(key))}>
        {options.map((o) => (
          <SelectItem key={o.id} id={o.id} textValue={o.name}>
            {o.name}
          </SelectItem>
        ))}
      </Select>
    </AlertDialog>
  );
}
