/*
 * Picking a GIF from the add menu. Searching sends the words to GIPHY,
 * and the picker says so; with no words it shows what is trending. A
 * click drops the GIF into the message as a picture, and GIPHY's mark
 * sits under the results as its terms ask.
 */
import { RiSearchLine } from "@remixicon/react";
import { useEffect, useState } from "react";

import { Input } from "@/components/base/input/input";

import { anonymousId, type Gif, searchGifs } from "./giphy";

const searchDelay = 300;

export interface PickedGif {
  gif: Gif;
  randomId: string;
}

/*
 * useGifSearch searches GIPHY a moment after the last letter typed.
 */
function useGifSearch(apiKey: string, member: string, query: string) {
  const [found, setFound] = useState<{ gifs: Gif[]; randomId: string; failed: boolean }>({
    gifs: [],
    randomId: "",
    failed: false,
  });
  useEffect(() => {
    const abort = new AbortController();
    const timer = setTimeout(async () => {
      try {
        const randomId = await anonymousId(apiKey, member);
        const gifs = await searchGifs(apiKey, query, randomId, abort.signal);
        setFound({ gifs, randomId, failed: false });
      } catch {
        if (!abort.signal.aborted) {
          setFound((before) => ({ ...before, failed: true }));
        }
      }
    }, searchDelay);

    return () => {
      clearTimeout(timer);
      abort.abort();
    };
  }, [apiKey, member, query]);

  return found;
}

export function GifPicker({
  apiKey,
  member,
  onPick,
}: {
  apiKey: string;
  member: string;
  onPick: (picked: PickedGif) => void;
}) {
  const [query, setQuery] = useState("");
  const { gifs, randomId, failed } = useGifSearch(apiKey, member, query);

  return (
    <div className="flex flex-col gap-2">
      <Input
        aria-label="Search GIFs"
        placeholder="Search GIPHY"
        leadingIcon={RiSearchLine}
        fieldClassName="bg-background-secondary-default"
        value={query}
        onChange={setQuery}
        autoFocus
      />
      <p className="px-1 text-body-2-regular text-text-tertiary">Searching GIFs sends your search to GIPHY.</p>
      <div className="grid max-h-64 grid-cols-3 gap-1 overflow-y-auto">
        {gifs.map((gif) => (
          <button
            key={gif.id}
            type="button"
            aria-label={gif.title}
            onClick={() => onPick({ gif, randomId })}
            className="cursor-pointer overflow-hidden rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <img src={gif.preview} alt="" className="h-20 w-full object-cover" />
          </button>
        ))}
      </div>
      {failed ? <p className="px-1 text-body-2-regular text-text-error-primary">GIF search is not available.</p> : null}
      <p className="px-1 text-end text-caption-1-semibold text-text-secondary">Powered by GIPHY</p>
    </div>
  );
}
