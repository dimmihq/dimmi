/*
 * Leaving the workspace in play. The last owner cannot, and the server's
 * answer says to hand it over first.
 */
import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Modal, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { callerQuery, leaveWorkspace } from "@/lib/api/workspace";

import { Danger } from "../controls";
import { Row, Section } from "../layout";

export function LeaveWorkspace({ id, name }: { id: string; name: string }) {
  const { data: caller } = useQuery(callerQuery(id));
  const [open, setOpen] = useState(false);
  const leave = useMutation({
    mutationFn: () => leaveWorkspace(caller?.member?.id ?? ""),
    onSuccess: () => window.location.assign("/d"),
  });

  if (!caller?.member) {
    return null;
  }

  const close = () => {
    leave.reset();
    setOpen(false);
  };

  return (
    <Section title="Workspace access">
      <Row label={`Leave ${name}`} description="You lose access to its conversations straight away.">
        <Danger onPress={() => setOpen(true)}>Leave workspace</Danger>
      </Row>
      <Modal size="sm" isOpen={open} onOpenChange={(next) => (next ? setOpen(true) : close())}>
        <ModalHeader
          tone="danger"
          title={`Leave ${name}?`}
          description={leave.error?.message ?? "Someone with access has to invite you again to come back."}
        />
        <ModalFooter className="mt-6">
          <Button variant="secondary" onClick={close}>
            Cancel
          </Button>
          <Button variant="danger" disabled={leave.isPending} onClick={() => leave.mutate()}>
            {leave.isPending ? "Leaving…" : "Leave workspace"}
          </Button>
        </ModalFooter>
      </Modal>
    </Section>
  );
}
