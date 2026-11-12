/*
 * The confirmation before a policy is archived. Archiving cancels every
 * clock the policy is running, so the dialog says so before anything
 * changes.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { archiveSLAPolicy, type SLAPolicy, slaPoliciesQuery } from "@/lib/api/sla";

export function ArchivePolicyDialog({
  workspace,
  policy,
  onClose,
  onArchived,
}: {
  workspace: string;
  policy: SLAPolicy;
  onClose: () => void;
  onArchived?: () => unknown;
}) {
  const { t } = useTranslation("sla");
  const client = useQueryClient();

  return (
    <AlertDialog
      isOpen
      onOpenChange={(next) => (next ? undefined : onClose())}
      title={t("archive.confirmTitle", { name: policy.name })}
      description={t("archive.confirmBody")}
      confirmLabel={t("archive.confirm")}
      onConfirm={async () => {
        try {
          await archiveSLAPolicy(workspace, policy.id);
          await client.invalidateQueries({ queryKey: slaPoliciesQuery(workspace).queryKey });
          toast.success(t("archive.done"));
          await onArchived?.();
          onClose();
        } catch (error) {
          toast.error((error as Error).message);
          throw error;
        }
      }}
    />
  );
}
