/*
 * Which ticket dialog a conversation has open, so the header menu and
 * the keyboard open the same ones.
 */
import { useState } from "react";

export type TicketDialog = "convert" | "escalate" | "tracker";

/*
 * useTicketDialogs holds which ticket dialog is open.
 */
export function useTicketDialogs() {
  const [open, setOpen] = useState<TicketDialog | null>(null);

  return { open, show: setOpen, close: () => setOpen(null) };
}

export type TicketDialogs = ReturnType<typeof useTicketDialogs>;
