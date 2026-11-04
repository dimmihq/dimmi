import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Row, Section } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { createEmailAddress } from "@/lib/api/email";

import { Failure } from "./failure";

/*
 * AddAddress connects another public address.
 */
export function AddAddress() {
  const me = useAccount();
  const queryClient = useQueryClient();
  const [address, setAddress] = useState("");
  const add = useMutation({
    mutationFn: () => createEmailAddress(me.workspace.id, address.trim()),
    onSuccess: () => {
      setAddress("");
      return queryClient.invalidateQueries({ queryKey: ["workspace", me.workspace.id, "email-addresses"] });
    },
  });

  return (
    <Section title="Add an address" description="Connect another address your customers write to.">
      <Row label="Address">
        <Input
          aria-label="New email address"
          size="small"
          placeholder="support@yourcompany.com"
          value={address}
          onChange={setAddress}
          className="w-64"
        />
        <Button size="small" disabled={!address.trim() || add.isPending} onClick={() => add.mutate()}>
          Add address
        </Button>
      </Row>
      <Failure error={add.error} />
    </Section>
  );
}
