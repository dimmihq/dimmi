/*
 * Narrows the help collections as the visitor types, matching any part of
 * a title regardless of case, and says so when nothing is left.
 */
import { Controller } from "../lib/controller";

export class SearchController extends Controller {
  filter(event: Event): void {
    const query = (event.target as HTMLInputElement).value.trim().toLowerCase();
    let shown = 0;
    for (const row of this.element.querySelectorAll<HTMLElement>('[data-target="row"]')) {
      const match = (row.textContent ?? "").toLowerCase().includes(query);
      row.hidden = !match;
      shown += Number(match);
    }
    this.target("none")?.toggleAttribute("hidden", shown > 0);
  }
}
