/*
 * Searches the help as the visitor types: every article the messenger
 * holds, by title, summary and its own words, matched regardless of case.
 * The collections and the most read articles make way for the matches,
 * and come back when the field is emptied.
 */
import { Controller } from "../lib/controller";

export class SearchController extends Controller {
  filter(event: Event): void {
    const query = (event.target as HTMLInputElement).value.trim().toLowerCase();
    const browsing = query === "";
    let shown = 0;

    for (const hit of this.element.querySelectorAll<HTMLElement>('[data-target="hit"]')) {
      const match = !browsing && (hit.dataset.words ?? "").includes(query);
      hit.hidden = !match;
      shown += Number(match);
    }

    for (const part of this.element.querySelectorAll<HTMLElement>('[data-target="browse"]')) {
      part.hidden = !browsing;
    }

    this.target("results")?.toggleAttribute("hidden", browsing || shown === 0);
    this.target("none")?.toggleAttribute("hidden", browsing || shown > 0);
  }
}
