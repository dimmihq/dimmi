/*
 * The composer writes either a reply the customer receives or a note
 * only teammates see. The modes also key the composer's mode switch.
 */
export const modes = { reply: "reply", note: "note" } as const;

export type Mode = keyof typeof modes;
