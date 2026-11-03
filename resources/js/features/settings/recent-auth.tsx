/*
 * Changes that could lock the owner out, such as adding or removing a
 * second factor, need a recent proof of who is at the keyboard. When the
 * server says the last one is too old, the person types their password
 * once and the change is sent again; closing the prompt drops it.
 */
import { useRef, useState } from "react";

import { ApiError } from "@/lib/api/client";

import { ConfirmPasswordDialog } from "./confirm-password-dialog";

/*
 * needsConfirmation reports whether the server refused a change only
 * because the last proof of identity is too old.
 */
export function needsConfirmation(error: unknown): boolean {
  return error instanceof ApiError && error.code === "password_confirmation_required";
}

interface Pending {
  action: () => Promise<unknown>;
  resolve: (done: boolean) => void;
  reject: (error: unknown) => void;
}

/*
 * useRecentAuth returns run, which sends a change and, when the server
 * asks, prompts for the password and sends it again, and the prompt to
 * render. run resolves true once the change went through and false when
 * the person closed the prompt.
 */
export function useRecentAuth(hasPassword: boolean) {
  const pending = useRef<Pending | null>(null);
  const [open, setOpen] = useState(false);

  const run = async (action: () => Promise<unknown>): Promise<boolean> => {
    try {
      await action();

      return true;
    } catch (error) {
      if (!needsConfirmation(error)) {
        throw error;
      }

      return new Promise<boolean>((resolve, reject) => {
        pending.current = { action, resolve, reject };
        setOpen(true);
      });
    }
  };

  const close = () => {
    pending.current?.resolve(false);
    pending.current = null;
    setOpen(false);
  };

  const confirmed = async () => {
    const current = pending.current;
    pending.current = null;
    setOpen(false);

    if (!current) {
      return;
    }

    try {
      await current.action();
      current.resolve(true);
    } catch (error) {
      current.reject(error);
    }
  };

  const prompt = (
    <ConfirmPasswordDialog isOpen={open} hasPassword={hasPassword} onCancel={close} onConfirmed={confirmed} />
  );

  return { run, prompt };
}
