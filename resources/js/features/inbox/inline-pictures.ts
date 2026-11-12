/*
 * An email's HTML shows some of its pictures in place, naming each by its
 * content id as cid:<id>. The thread draws those inside the text from
 * the stored file, and leaves them out of the files listed below it so
 * no picture shows twice. A picture the text never names, or one not
 * readable yet, stays in the list.
 */
import type { Attachment } from "@/lib/api/inbox";

const cidScheme = "cid:";

/*
 * inlinePictures splits a message's files into the pictures its text
 * shows in place, by content id, and the files left to list.
 */
export function inlinePictures(
  text: string,
  files: Attachment[],
): { pictures: Record<string, string>; listed: Attachment[] } {
  const pictures: Record<string, string> = {};
  const listed: Attachment[] = [];

  for (const file of files) {
    const id = file.content_id;
    if (id && file.url && !file.blocked && text.includes(cidScheme + id)) {
      pictures[id] = file.url;
      continue;
    }
    listed.push(file);
  }

  return { pictures, listed };
}

/*
 * pictureFor returns the address of the picture a cid: link names, or
 * null when the link is not one or names no picture of the message.
 */
export function pictureFor(url: string, pictures: Record<string, string>): string | null {
  if (!url.startsWith(cidScheme)) {
    return null;
  }

  return pictures[url.slice(cidScheme.length)] ?? "";
}
