/*
 * The whole of a received email, for when the thread shows only what
 * the customer wrote this time or had to cut a long body short. The
 * email's own layout is shown in a sandboxed frame that runs nothing and
 * loads nothing, its links opening in a new tab; its plain words are one
 * switch away. Someone allowed to read raw mail can also save the email
 * exactly as it arrived.
 */
import { RiDownloadLine, RiMailOpenLine } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Spinner } from "@/components/base/spinner/spinner";
import { downloadRawEmail, type MessageOriginal, messageOriginalQuery } from "@/lib/api/inbox";

const looks = { formatted: "formatted", text: "text" } as const;

type Look = (typeof looks)[keyof typeof looks];

/*
 * save hands the browser a file to keep under a name.
 */
function save(file: Blob, name: string) {
  const url = URL.createObjectURL(file);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  URL.revokeObjectURL(url);
}

function Shortened({ original }: { original: MessageOriginal }) {
  if (!original.text_truncated && !original.html_too_large) {
    return null;
  }

  return (
    <p className="text-body-2-regular text-text-tertiary">
      This email is longer than dimmi keeps.
      {original.raw_available ? " Download the original to read all of it." : ""}
    </p>
  );
}

function Content({ original, look }: { original: MessageOriginal; look: Look }) {
  if (look === "formatted" && original.html) {
    return (
      <iframe
        title="Email as it was formatted"
        sandbox="allow-popups allow-popups-to-escape-sandbox"
        srcDoc={original.html}
        className="h-[60vh] w-full rounded-xl border border-border-button-default"
      />
    );
  }

  return (
    <p className="whitespace-pre-wrap wrap-break-word rounded-xl bg-background-secondary-default p-3 text-body-regular text-text-primary">
      {original.text}
    </p>
  );
}

function Original({ original }: { original: MessageOriginal }) {
  const [look, setLook] = useState<Look>(original.html ? "formatted" : "text");

  return (
    <div className="flex flex-col gap-3">
      {original.html ? (
        <SegmentedControl
          aria-label="Show the email"
          selectedKeys={[look]}
          onSelectionChange={(selected) => {
            const next = [...selected][0];
            if (next === looks.formatted || next === looks.text) {
              setLook(next);
            }
          }}
          className="self-start"
        >
          <SegmentedControlItem id={looks.formatted}>Formatted</SegmentedControlItem>
          <SegmentedControlItem id={looks.text}>Plain text</SegmentedControlItem>
        </SegmentedControl>
      ) : null}
      <Content original={original} look={look} />
      <Shortened original={original} />
    </div>
  );
}

export function OriginalMessage({
  workspace,
  conversation,
  message,
  onClose,
}: {
  workspace: string;
  conversation: string;
  message: string;
  onClose: () => void;
}) {
  const { data: original, error } = useQuery(messageOriginalQuery(workspace, conversation, message));
  const download = useMutation({
    mutationFn: () => downloadRawEmail(workspace, conversation, message),
    onSuccess: (file) => save(file.blob, file.filename),
  });

  return (
    <Modal size="xl" isOpen onOpenChange={(open) => !open && onClose()}>
      <ModalHeader
        icon={RiMailOpenLine}
        title="Full message"
        description="Everything the customer sent, including the quoted thread and signature."
      />
      <ModalBody>
        {original ? <Original original={original} /> : null}
        {!original && !error ? <Spinner size="md" className="self-center" /> : null}
        {error ? <p className="text-body-regular text-text-error-primary">{error.message}</p> : null}
        {download.error ? (
          <p className="pt-2 text-body-2-regular text-text-error-primary">{download.error.message}</p>
        ) : null}
      </ModalBody>
      <ModalFooter>
        {original?.raw_available ? (
          <Button
            variant="secondary"
            leadingIcon={RiDownloadLine}
            loading={download.isPending}
            onClick={() => download.mutate()}
          >
            Download original
          </Button>
        ) : null}
        <Button onClick={onClose}>Done</Button>
      </ModalFooter>
    </Modal>
  );
}
