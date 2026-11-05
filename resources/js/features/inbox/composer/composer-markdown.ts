/*
 * The composer's document: only what Markdown can carry and every
 * channel can show. Paragraphs, line breaks, bold, italic, strike, inline
 * code, code blocks, links, lists, quotes, inline images and mentions; no headings,
 * rules, underline, sizes or colours, which a reply cannot keep. The
 * composer reads and writes Markdown only, so the API never changes.
 */

import Image from "@tiptap/extension-image";
import { Markdown } from "@tiptap/markdown";
import { type AnyExtension, Extension } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";

import { MentionToken } from "./mention-token";

/*
 * ComposerKeys adds the composer's own shortcuts: Cmd+Shift+X strikes
 * through, and Cmd+Shift+U asks for a link through onLink.
 */
function composerKeys(onLink: () => void) {
  return Extension.create({
    name: "composerKeys",
    addKeyboardShortcuts() {
      return {
        "Mod-Shift-x": () => this.editor.commands.toggleStrike(),
        "Mod-Shift-u": () => {
          onLink();
          return true;
        },
      };
    },
  });
}

/*
 * composerExtensions is the composer's schema with Markdown in and out,
 * plus any helpers, such as suggestions while typing.
 */
export function composerExtensions(onLink: () => void = () => undefined, helpers: AnyExtension[] = []) {
  return [
    StarterKit.configure({
      heading: false,
      horizontalRule: false,
      underline: false,
      link: { openOnClick: false, autolink: true, protocols: ["http", "https", "mailto"] },
    }),
    Image.configure({ inline: true, allowBase64: false }),
    MentionToken,
    Markdown,
    composerKeys(onLink),
    ...helpers,
  ];
}
