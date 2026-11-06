/*
 * The composer's GIFs: whether the workspace may search them, and the
 * GIFs dropped into the message, so GIPHY hears about each one that is
 * still in the message when it is sent.
 */
import { useQuery } from "@tanstack/react-query";
import { useRef } from "react";

import { gifSearchQuery } from "@/lib/api/inbox";

import type { Gifs } from "../add-menu";
import type { PickedGif } from "./gif-picker";
import { pingSent } from "./giphy";

export function useGifs(workspace: string, member: string, insert: (text: string) => void) {
  const { data } = useQuery({ ...gifSearchQuery(workspace), enabled: workspace !== "" });
  const picked = useRef(new Map<string, PickedGif>());

  const gifs: Gifs | null =
    data?.enabled && data.api_key
      ? {
          apiKey: data.api_key,
          member,
          onPick: (gif) => {
            picked.current.set(gif.gif.url, gif);
            insert(`![${gif.gif.title.replaceAll(/[[\]]/g, "")}](${gif.gif.url})`);
          },
        }
      : null;

  const sent = (text: string) => {
    for (const [url, gif] of picked.current) {
      if (text.includes(url) && gif.gif.onSent) {
        pingSent(gif.gif.onSent, gif.randomId);
      }
    }
    picked.current.clear();
  };

  return { gifs, sent };
}
