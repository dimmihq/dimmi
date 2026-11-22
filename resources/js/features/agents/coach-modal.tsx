/*
 * Coach: teach the agent from one of its real replies. It shows what
 * the customer wrote and what the agent said, then asks when the lesson
 * applies and what the agent should do next time: follow guidance in
 * your words, follow a procedure, answer from an article or hand over.
 * The lesson lands in the draft, and the chat can be kept as a
 * simulation so the mistake never comes back. Opened from a reply in
 * the inbox or from Analytics. Mockup: nothing saved.
 */
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { CheckboxCard } from "@/components/base/checkbox/checkbox-card";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { MessageBody } from "@/features/inbox/message-body";

import { Pick } from "../automation/pick";
import { fromCustomer } from "../automation/procedure-simulator/simulator";

export interface Coached {
  number: number;
  customer: string;
  reply: string;
}

const label = "text-body-2-medium text-text-primary";

function Behaviour({ kind }: { kind: string }) {
  const [procedure, setProcedure] = useState("qualify");
  const [article, setArticle] = useState("pricing");
  const [team, setTeam] = useState("sales");

  if (kind === "guidance") {
    return (
      <Textarea
        aria-label="Guidance"
        rows={2}
        autoResize
        defaultValue="Existing customers with a billing problem go to dimmi. Don't answer about plans."
      />
    );
  }
  if (kind === "procedure") {
    return (
      <Pick
        label="Procedure"
        choices={[
          ["qualify", "Qualify & book demo"],
          ["refund", "Refund request"],
        ]}
        value={procedure}
        onChange={setProcedure}
      />
    );
  }
  if (kind === "article") {
    return (
      <Pick
        label="Article"
        choices={[
          ["pricing", "Pricing & plans"],
          ["sso", "Single sign-on"],
        ]}
        value={article}
        onChange={setArticle}
      />
    );
  }

  return (
    <Pick
      label="Hand over to"
      choices={[
        ["sales", "Sales team"],
        ["support", "Support team"],
        ["dimmi", "dimmi (transfer)"],
      ]}
      value={team}
      onChange={setTeam}
    />
  );
}

export function CoachModal({
  agentName,
  coached,
  onClose,
}: {
  agentName: string;
  coached: Coached;
  onClose: () => void;
}) {
  const [when, setWhen] = useState("An existing customer reports a billing or export problem");
  const [kind, setKind] = useState("guidance");
  const [keep, setKeep] = useState(true);

  return (
    <Modal
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      size="md"
      className="h-[min(720px,calc(100dvh-3rem))]"
    >
      <ModalHeader
        title={`Coach ${agentName}`}
        description={`From conversation #${coached.number}. What you teach goes into the draft.`}
      />
      <ModalBody className="flex min-h-0 flex-col gap-5 overflow-y-auto">
        <div className="flex flex-col gap-3 rounded-2xl bg-background-secondary-default p-3">
          <p className={fromCustomer}>{coached.customer}</p>
          <MessageBody text={coached.reply} className="text-body-regular text-text-primary" />
        </div>
        <Input label="When" value={when} onChange={setWhen} />
        <div className="flex flex-col gap-2">
          <span className={label}>Next time</span>
          <RadioGroup aria-label="Next time" value={kind} onChange={setKind} className="grid grid-cols-2 gap-2">
            <RadioCard value="guidance" title="Follow guidance" description="In your words." />
            <RadioCard value="procedure" title="Follow a procedure" description="Step by step." />
            <RadioCard value="article" title="Answer from an article" description="And cite it." />
            <RadioCard value="handover" title="Hand over" description="To a team or an agent." />
          </RadioGroup>
          <Behaviour kind={kind} />
        </div>
        <CheckboxCard
          title="Keep this chat as a simulation"
          description="It runs with the others before you publish."
          isSelected={keep}
          onChange={setKeep}
        />
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button
          size="small"
          onClick={() => {
            toast.success(keep ? "Added to the draft, with a new simulation" : "Added to the draft");
            onClose();
          }}
        >
          Add to draft
        </Button>
      </ModalFooter>
    </Modal>
  );
}
