/*
 * The video and demo hosts an article can embed from, the same allow-list
 * the help center renders with (framework/markdown/embeds.go). A link is
 * read for its video or demo id, and the player address is built from a
 * fixed prefix, so the writer previews exactly what readers will see.
 * Both sides are tested against the same cases.
 */

export interface Embed {
  provider: string;
  player: string;
}

interface Provider {
  name: string;
  label: string;
  hosts: string[];
  player: string;
  id: (url: URL, parts: string[]) => string | null;
  suffix?: (url: URL, parts: string[]) => string;
}

const youtubeId = /^[A-Za-z0-9_-]{11}$/;
const digitsId = /^\d{1,15}$/;
const hexId = /^[a-f0-9]{32}$/;
const wistiaId = /^[a-z0-9]{10}$/;
const slugId = /^[A-Za-z0-9_-]{4,64}$/;
const vimeoHash = /^[a-f0-9]{6,20}$/;

function shaped(id: string | undefined | null, shape: RegExp): string | null {
  return id && shape.test(id) ? id : null;
}

function after(prefixes: string[], shape: RegExp) {
  return (_: URL, parts: string[]) =>
    parts.length >= 2 && prefixes.includes(parts[0] ?? "") ? shaped(parts[1], shape) : null;
}

const providers: Provider[] = [
  {
    name: "youtube",
    label: "YouTube",
    hosts: ["youtube.com", "www.youtube.com", "m.youtube.com", "youtu.be", "www.youtube-nocookie.com"],
    player: "https://www.youtube-nocookie.com/embed/",
    id: (url, parts) => {
      if (url.hostname === "youtu.be" && parts.length === 1) {
        return shaped(parts[0], youtubeId);
      }
      if (parts.length === 1 && parts[0] === "watch") {
        return shaped(url.searchParams.get("v"), youtubeId);
      }
      if (parts.length === 2 && ["embed", "shorts", "live"].includes(parts[0] ?? "")) {
        return shaped(parts[1], youtubeId);
      }
      return null;
    },
    suffix: (url) => {
      const t = (url.searchParams.get("t") ?? "").replace(/s$/, "") || (url.searchParams.get("start") ?? "");
      return /^\d+$/.test(t) && Number(t) > 0 ? `?start=${Number(t)}` : "";
    },
  },
  {
    name: "vimeo",
    label: "Vimeo",
    hosts: ["vimeo.com", "www.vimeo.com", "player.vimeo.com"],
    player: "https://player.vimeo.com/video/",
    id: (url, parts) => {
      if (url.hostname === "player.vimeo.com") {
        return parts.length === 2 && parts[0] === "video" ? shaped(parts[1], digitsId) : null;
      }
      return parts.length === 1 || parts.length === 2 ? shaped(parts[0], digitsId) : null;
    },
    suffix: (url, parts) => {
      let hash = url.searchParams.get("h") ?? "";
      if (!hash && parts.length === 2 && parts[0] !== "video") {
        hash = parts[1] ?? "";
      }
      return vimeoHash.test(hash) ? `?h=${hash}` : "";
    },
  },
  {
    name: "loom",
    label: "Loom",
    hosts: ["loom.com", "www.loom.com"],
    player: "https://www.loom.com/embed/",
    id: after(["share", "embed"], hexId),
  },
  {
    name: "wistia",
    label: "Wistia",
    hosts: ["fast.wistia.net", "fast.wistia.com", "*.wistia.com"],
    player: "https://fast.wistia.net/embed/iframe/",
    id: (_, parts) => {
      if (parts.length === 2 && parts[0] === "medias") {
        return shaped(parts[1], wistiaId);
      }
      if (parts.length === 3 && parts[0] === "embed" && ["iframe", "medias"].includes(parts[1] ?? "")) {
        return shaped(parts[2], wistiaId);
      }
      return null;
    },
  },
  {
    name: "vidyard",
    label: "Vidyard",
    hosts: ["share.vidyard.com", "play.vidyard.com", "video.vidyard.com"],
    player: "https://play.vidyard.com/",
    id: (_, parts) => {
      if (parts.length === 2 && parts[0] === "watch") {
        return shaped(parts[1], slugId);
      }
      return parts.length === 1 ? shaped(parts[0]?.replace(/\.html$/, ""), slugId) : null;
    },
  },
  {
    name: "arcade",
    label: "Arcade",
    hosts: ["app.arcade.software", "demo.arcade.software"],
    player: "https://demo.arcade.software/",
    id: (url, parts) => {
      if (parts.length === 2 && parts[0] === "share") {
        return shaped(parts[1], slugId);
      }
      return parts.length === 1 && url.hostname === "demo.arcade.software" ? shaped(parts[0], slugId) : null;
    },
    suffix: () => "?embed",
  },
  {
    name: "supademo",
    label: "Supademo",
    hosts: ["app.supademo.com"],
    player: "https://app.supademo.com/embed/",
    id: after(["demo", "embed"], slugId),
  },
  {
    name: "storylane",
    label: "Storylane",
    hosts: ["app.storylane.io"],
    player: "https://app.storylane.io/demo/",
    id: after(["share", "demo"], slugId),
  },
  {
    name: "tella",
    label: "Tella",
    hosts: ["tella.tv", "www.tella.tv"],
    player: "https://www.tella.tv/video/",
    id: after(["video"], slugId),
    suffix: () => "/embed",
  },
  {
    name: "navattic",
    label: "Navattic",
    hosts: ["capture.navattic.com"],
    player: "https://capture.navattic.com/",
    id: (_, parts) => (parts.length === 1 ? shaped(parts[0], slugId) : null),
  },
];

/*
 * embedProviders names the hosts, for the hint under the link field.
 */
export const embedProviders = providers.map((p) => p.label);

function matchesHost(hosts: string[], host: string): boolean {
  return hosts.some((h) => {
    if (h.startsWith("*.")) {
      const rest = h.slice(1);
      const sub = host.endsWith(rest) ? host.slice(0, -rest.length) : "";
      return sub !== "" && !sub.includes(".");
    }
    return host === h;
  });
}

/*
 * parseURL reads an address the way the server does, refusing what URL
 * would quietly fix, such as a ".." in the path or a stray quote.
 */
function parseURL(raw: string): URL | null {
  const text = raw.trim();
  if (!/^https?:\/\/[^/?#\s]+(?:[/?#]\S*)?$/i.test(text) || /\/\.\.?(?:\/|$)/.test(text)) {
    return null;
  }
  try {
    return new URL(text);
  } catch {
    return null;
  }
}

/*
 * resolveEmbed reads a link and returns its player, or null when it is
 * not from an allowed host or has no valid id.
 */
export function resolveEmbed(raw: string): Embed | null {
  const url = parseURL(raw);
  if (!url || url.username || url.password || url.port) {
    return null;
  }
  const host = url.hostname.toLowerCase();
  const provider = providers.find((p) => matchesHost(p.hosts, host));
  if (!provider) {
    return null;
  }
  const parts = url.pathname.split("/").filter(Boolean).map(decodeURIComponent);
  const id = provider.id(url, parts);
  if (!id) {
    return null;
  }

  return { provider: provider.name, player: provider.player + id + (provider.suffix?.(url, parts) ?? "") };
}
