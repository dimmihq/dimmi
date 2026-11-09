/*
 * Posting a request for the team: what a customer asked for, or the
 * team's own idea, counted as that customer's vote when one is named.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { type FormEvent, useId, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { createPost, feedbackKeys } from "@/lib/api/feedback";

import { CustomerPicker, type Picked } from "./customer-picker";

export function NewRequestDialog({
  workspace,
  helpCenter,
  onClose,
}: {
  workspace: string;
  helpCenter: string;
  onClose: () => void;
}) {
  const formId = useId();
  const navigate = useNavigate();
  const client = useQueryClient();
  const [title, setTitle] = useState("");
  const [details, setDetails] = useState("");
  const [customer, setCustomer] = useState<Picked | null>(null);
  const create = useMutation({
    mutationFn: () =>
      createPost(workspace, helpCenter, {
        title,
        body_markdown: details,
        ...(customer ? { contact_id: customer.id } : {}),
      }),
    onSuccess: (post) => {
      void client.invalidateQueries({ queryKey: feedbackKeys.posts(workspace, helpCenter) });
      void navigate({ to: "/d/feedback/$post", params: { post: post.id } });
    },
    onError: (error) => toast.error(error.message),
  });
  const submit = (event: FormEvent) => {
    event.preventDefault();
    create.mutate();
  };

  return (
    <Modal
      size="md"
      className="h-[min(640px,calc(100dvh-3rem))]"
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
    >
      <ModalHeader title="New request" description="Post what a customer asked for, or an idea of the team's." />
      <ModalBody className="pb-4">
        <form id={formId} onSubmit={submit} className="flex flex-col gap-4">
          <Input label="Title" value={title} onChange={setTitle} maxLength={150} isRequired autoFocus />
          <Textarea label="Details" value={details} onChange={setDetails} rows={5} />
          <CustomerPicker workspace={workspace} picked={customer} onPick={setCustomer} />
        </form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" form={formId} loading={create.isPending} disabled={title.trim() === ""}>
          Post request
        </Button>
      </ModalFooter>
    </Modal>
  );
}
