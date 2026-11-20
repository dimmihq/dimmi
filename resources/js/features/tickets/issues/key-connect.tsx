/*
 * Connecting an incident tool that takes an API key (incident.io,
 * Rootly, PagerDuty): the key is pasted once, checked with the tool,
 * and the integration's page opens next, where the webhook address and
 * signing secret are set up.
 */
import { RiKey2Line } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { integrationsQuery } from "@/lib/api/integrations";
import { connectWithKey } from "@/lib/api/ticket-issues";

import { toolLook } from "./tools";

/*
 * keyHelp says where each tool's key is made and what it needs.
 */
const keyHelp: Record<string, string> = {
  incident_io: "Create it in incident.io under Settings, API keys, with view, create and edit incidents.",
  rootly: "Create a global API key in Rootly under Account, API keys.",
  pagerduty: "Create a REST API key in PagerDuty under Integrations, API Access Keys.",
};

/*
 * KeyConnectDialog asks for the tool's key and connects it.
 */
export function KeyConnectDialog({
  workspace,
  provider,
  isOpen,
  onOpenChange,
}: {
  workspace: string;
  provider: string;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [key, setKey] = useState("");
  const label = toolLook(provider).label;

  const connect = useMutation({
    mutationFn: () => connectWithKey(workspace, provider, key.trim()),
    onSuccess: async (made) => {
      await queryClient.invalidateQueries({ queryKey: integrationsQuery(workspace).queryKey });
      toast.success(`${label} is connected.`);
      onOpenChange(false);
      await navigate({ to: "/d/settings/integrations/$integration", params: { integration: made.id } });
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange}>
      <ModalHeader icon={RiKey2Line} title={`Connect ${label}`} description={keyHelp[provider]} />
      <ModalBody>
        <Input label="API key" type="password" value={key} onChange={setKey} autoFocus />
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={() => onOpenChange(false)}>
          Cancel
        </Button>
        <Button disabled={key.trim() === ""} loading={connect.isPending} onClick={() => connect.mutate()}>
          Connect
        </Button>
      </ModalFooter>
    </Modal>
  );
}

export function KeyConnectButton({ workspace, provider }: { workspace: string; provider: string }) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button variant="secondary" size="small" onClick={() => setOpen(true)}>
        Connect
      </Button>
      <KeyConnectDialog workspace={workspace} provider={provider} isOpen={open} onOpenChange={setOpen} />
    </>
  );
}
