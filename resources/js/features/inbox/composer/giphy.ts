/*
 * GIPHY's search, called from the browser as GIPHY's terms require:
 * never through our server, results only rated G, media addresses used
 * exactly as returned. Each teammate searches under an anonymous id
 * GIPHY hands out, kept in their browser, and a GIF that gets sent tells
 * GIPHY so through the address it gave for that.
 */

const api = "https://api.giphy.com/v1";

export interface Gif {
  id: string;
  title: string;
  preview: string;
  url: string;
  onSent: string | null;
}

interface Rendition {
  url: string;
}

interface Found {
  id: string;
  title: string;
  images: { fixed_width_small?: Rendition; fixed_width?: Rendition; downsized_medium?: Rendition };
  analytics?: { onsent?: { url: string } };
}

/*
 * anonymousId returns the id GIPHY gave this teammate, asking for one the
 * first time and keeping it in the browser when it may.
 */
export async function anonymousId(key: string, member: string): Promise<string> {
  const stored = `dimmi:giphy:${member}`;
  try {
    const kept = localStorage.getItem(stored);
    if (kept) {
      return kept;
    }
  } catch {
    // Storage may be blocked; a fresh id then serves this visit.
  }
  const response = await fetch(`${api}/randomid?api_key=${encodeURIComponent(key)}`);
  const body = (await response.json()) as { data?: { random_id?: string } };
  const id = body.data?.random_id ?? "";
  try {
    localStorage.setItem(stored, id);
  } catch {
    // Kept for this visit only.
  }

  return id;
}

export async function searchGifs(key: string, query: string, randomId: string, signal?: AbortSignal): Promise<Gif[]> {
  const params = new URLSearchParams({ api_key: key, limit: "24", rating: "g", random_id: randomId });
  const words = query.trim();
  if (words !== "") {
    params.set("q", words);
  }
  const response = await fetch(`${api}/gifs/${words === "" ? "trending" : "search"}?${params}`, { signal });
  if (!response.ok) {
    throw new Error("GIF search is not available right now.");
  }
  const body = (await response.json()) as { data: Found[] };

  return body.data.flatMap((found) => {
    const url = found.images.downsized_medium?.url ?? found.images.fixed_width?.url;
    const preview = found.images.fixed_width_small?.url ?? url;
    if (!url || !preview) {
      return [];
    }

    return [{ id: found.id, title: found.title || "GIF", preview, url, onSent: found.analytics?.onsent?.url ?? null }];
  });
}

/*
 * pingSent tells GIPHY a GIF was sent, as an image request so the page
 * reaches nothing beyond GIPHY's media hosts.
 */
export function pingSent(onSent: string, randomId: string) {
  const ping = new Image();
  ping.src = `${onSent}&random_id=${encodeURIComponent(randomId)}&ts=${Date.now()}`;
}
