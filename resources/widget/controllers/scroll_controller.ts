/*
 * Marks a scrolling view as scrolled once its content has moved, so pinned
 * parts such as the home view's top row can take a background and keep
 * their buttons readable over whatever passes under them.
 */
import { Controller } from "../lib/controller";

export class ScrollController extends Controller {
  #mark = () => this.element.toggleAttribute("data-scrolled", this.element.scrollTop > 4);

  connect(): void {
    this.element.addEventListener("scroll", this.#mark, { passive: true });
  }

  disconnect(): void {
    this.element.removeEventListener("scroll", this.#mark);
  }
}
