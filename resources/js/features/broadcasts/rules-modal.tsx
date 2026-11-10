/*
 * A rule written for one broadcast, without saving it as an audience:
 * the same builder as Customers, with a live count of who matches now.
 * Saving puts the rule on the draft and clears any saved audience.
 */
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RuleGroups } from "@/features/audiences/rule-groups";
import { type Form, toForm, toRules } from "@/features/audiences/rules";
import { useLookups } from "@/features/audiences/use-lookups";
import { type AudienceRules, audienceFieldsQuery, previewAudience } from "@/lib/api/audiences";
import { formatNumber } from "@/lib/i18n/format";

const blank: Form = { visitors: "everyone", match: "all", items: [] };

function Count({ workspace, form }: { workspace: string; form: Form }) {
  const rules = toRules(form);
  const { data, isFetching } = useQuery({
    queryKey: ["workspace", workspace, "audience-previews", JSON.stringify(rules)],
    queryFn: () => previewAudience(workspace, rules, 0),
    staleTime: 10_000,
  });
  if (!data) {
    return <span className="text-body-2-regular text-text-secondary">{isFetching ? "Counting…" : ""}</span>;
  }

  return (
    <span className="text-body-2-regular text-text-secondary">
      {formatNumber(data.count)}
      {data.approximate ? "+" : ""} {data.count === 1 ? "person matches" : "people match"} now
    </span>
  );
}

export function RulesModal({
  workspace,
  rules,
  onSave,
  onClose,
}: {
  workspace: string;
  rules: AudienceRules | null;
  onSave: (rules: AudienceRules) => void;
  onClose: () => void;
}) {
  const [form, setForm] = useState<Form>(() => toForm(rules ?? undefined) ?? blank);
  const fields = useQuery(audienceFieldsQuery(workspace)).data ?? [];
  const { lookups } = useLookups(workspace, "");

  return (
    <Modal size="wide" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader title="Who gets this broadcast" />
      <ModalBody className="pb-2">
        <RuleGroups form={form} fields={fields} lookups={lookups} onChange={setForm} />
      </ModalBody>
      <ModalFooter className="sm:justify-between">
        <Count workspace={workspace} form={form} />
        <span className="flex flex-col-reverse gap-2 sm:flex-row">
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={form.items.length === 0 && form.visitors === "everyone"}
            onClick={() => {
              onSave(toRules(form));
              onClose();
            }}
          >
            Use this rule
          </Button>
        </span>
      </ModalFooter>
    </Modal>
  );
}
