/*
 * Turning a synced source into help center articles: pick the help
 * center, and every live document becomes an editable article there,
 * placed in collections the way it sat at the source, with its old
 * address redirecting to it. Doing it again updates those articles.
 */
import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { ApiError } from "@/lib/api/client";
import { helpCentersQuery } from "@/lib/api/help-centers";
import { convertSource, type KnowledgeSource } from "@/lib/api/knowledge-sources";

export function ConvertModal({
  workspace,
  source,
  onClose,
}: {
  workspace: string;
  source: KnowledgeSource;
  onClose: () => void;
}) {
  const centers = useQuery(helpCentersQuery(workspace));
  const [center, setCenter] = useState<string | null>(source.help_center_id);

  const convert = useMutation({
    mutationFn: () => convertSource(workspace, source.id, center ?? ""),
    onSuccess: () => {
      toast.success(`${source.document_count} documents are becoming articles.`);
      onClose();
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "That did not work. Try again."),
  });

  return (
    <Modal isOpen onOpenChange={(open) => !open && onClose()} size="md">
      <ModalHeader
        title="Turn into articles"
        description={`Each of the ${source.document_count} live documents becomes an article you can edit.`}
      />
      <ModalBody>
        <Select
          label="Help center"
          placeholder="Choose a help center"
          selectedKey={center}
          onSelectionChange={(key) => setCenter(key === null ? null : String(key))}
        >
          {(centers.data ?? []).map((c) => (
            <SelectItem key={c.id} id={c.id} textValue={c.name}>
              {c.name}
            </SelectItem>
          ))}
        </Select>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button disabled={!center} loading={convert.isPending} onClick={() => convert.mutate()}>
          Turn into articles
        </Button>
      </ModalFooter>
    </Modal>
  );
}
