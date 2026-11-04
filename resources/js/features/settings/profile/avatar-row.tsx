/*
 * The person's picture, or their initials without one. The server keeps
 * only a small square redrawn from the upload, so what shows here is what
 * teammates see.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { FileUpload } from "@/components/base/file-upload/file-upload";
import { Modal, ModalBody, ModalHeader } from "@/components/base/modal/modal";
import { avatarColor } from "@/features/shell/avatar-color";
import { removeAvatar, uploadAvatar } from "@/lib/api/account";

import { Action } from "../controls";
import { Row } from "../layout";

const accepted = ["png", "jpg", "jpeg", "webp", "gif"];
const maxBytes = 2 * 1024 * 1024;

export function AvatarRow({ name, avatarUrl }: { name: string; avatarUrl: string | null }) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["me"] });
  const upload = useMutation({
    mutationFn: uploadAvatar,
    onSuccess: async () => {
      setOpen(false);
      await refresh();
    },
  });
  const remove = useMutation({ mutationFn: removeAvatar, onSuccess: refresh });
  const failure = upload.error?.message ?? remove.error?.message;

  return (
    <Row label="Profile picture" description={failure ?? "PNG, JPEG, WebP or GIF, up to 2 MB."}>
      <Avatar
        size="md"
        color={avatarColor(name)}
        initials={name.charAt(0)}
        src={avatarUrl ?? undefined}
        alt={avatarUrl ? "Your profile picture" : undefined}
      />
      <Action
        onPress={() => {
          upload.reset();
          setOpen(true);
        }}
      >
        Upload
      </Action>
      {avatarUrl ? <Action onPress={() => remove.mutate()}>Remove</Action> : null}
      <Modal size="sm" isOpen={open} onOpenChange={setOpen}>
        <ModalHeader title="Upload a picture" description="It is cropped to a square from the middle." />
        <ModalBody>
          <FileUpload
            allowedExtensions={accepted}
            maxBytes={maxBytes}
            onUploadComplete={(file) => upload.mutate(file)}
          />
        </ModalBody>
      </Modal>
    </Row>
  );
}
