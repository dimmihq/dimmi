/*
 * The shape of a message the series prototype previews with. Series are
 * not stored yet, so their steps are written here in the same terms the
 * broadcast preview reads: where it goes, its subject and preview line,
 * the Markdown body and who it comes from.
 */

export type Channel = "email" | "chat" | "post" | "banner";

/*
 * Broadcast is one message a series step sends, as the preview shows
 * it. when and edited say, in words, when it goes out and was changed.
 */
export interface Broadcast {
  id: string;
  name: string;
  channel: Channel;
  status: "draft" | "scheduled" | "sending" | "sent" | "stopped";
  audience: string;
  audienceSize: number;
  subject: string;
  preview: string;
  body: string;
  sender: string;
  when: string;
  edited: string;
}
