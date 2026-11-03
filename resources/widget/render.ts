/*
 * Turns options and content into the messenger: its markup, and the brand
 * color and spacing as custom properties set through the CSSOM, which a
 * strict content security policy allows where inline styles are blocked.
 */
import type { Content } from "./content";
import { isLight, type Options } from "./options";
import { Messenger } from "./views/messenger";

export function style(host: HTMLElement, { appearance }: Options): void {
  host.dataset.theme = appearance.theme;
  host.dataset.brand = isLight(appearance.primary_color) ? "light" : "dark";
  host.style.setProperty("--brand", appearance.primary_color);
  host.style.setProperty("--x", `${appearance.padding_x}px`);
  host.style.setProperty("--y", `${appearance.padding_y}px`);
}

export function render(options: Options, content: Content): string {
  return String(Messenger({ options, content }));
}
