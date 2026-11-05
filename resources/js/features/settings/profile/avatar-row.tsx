/*
 * The person's picture, or their initials without one. Upload opens the
 * file picker directly. The server keeps only a small square redrawn
 * from the upload, so what shows here is what teammates see.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { avatarColor } from "@/features/shell/avatar-color";
import { removeAvatar, uploadAvatar } from "@/lib/api/account";

import { Action } from "../controls";
import { Row } from "../layout";
import { PicturePicker } from "../picture-picker";

const tooLarge = "The picture must be 2 MB or smaller.";

export function AvatarRow({ name, avatarUrl }: { name: string; avatarUrl: string | null }) {
  const queryClient = useQueryClient();
  const [refused, setRefused] = useState<string | null>(null);
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["me"] });
  const upload = useMutation({ mutationFn: uploadAvatar, onSuccess: refresh });
  const remove = useMutation({ mutationFn: removeAvatar, onSuccess: refresh });
  const failure = refused ?? upload.error?.message ?? remove.error?.message;

  return (
    <Row label="Profile picture" description={failure ?? "PNG, JPEG, WebP or GIF, up to 2 MB."}>
      <Avatar
        size="md"
        color={avatarColor(name)}
        initials={name.charAt(0)}
        src={avatarUrl ?? undefined}
        alt={avatarUrl ? "Your profile picture" : undefined}
      />
      <PicturePicker
        isPending={upload.isPending}
        onTooLarge={() => setRefused(tooLarge)}
        onPick={(file) => {
          setRefused(null);
          upload.mutate(file);
        }}
      >
        Upload
      </PicturePicker>
      {avatarUrl ? (
        <Action loading={remove.isPending} onPress={() => remove.mutate()}>
          Remove
        </Action>
      ) : null}
    </Row>
  );
}
