/*
 * Controllers in the Stimulus style: each one is a small class bound to an
 * element that names it in data-controller. It finds its parts through
 * data-target and answers events named in data-action, such as
 * data-action="click->toggle". Markup says what happens; the class says how.
 * An action goes to the nearest controller around it that handles it.
 * One listener per event type on the root handles every action, so nothing
 * is bound per element and controllers can be swapped in and out freely.
 */

export type ControllerClass = new (element: HTMLElement, root: ParentNode) => Controller;

export class Controller<E extends HTMLElement = HTMLElement> {
  readonly element: E;
  readonly root: ParentNode;

  constructor(element: E, root: ParentNode) {
    this.element = element;
    this.root = root;
  }

  connect?(): void;

  disconnect?(): void;

  target<T extends HTMLElement = HTMLElement>(name: string): T | null {
    return this.element.querySelector<T>(`[data-target="${name}"]`);
  }

  dispatch(name: string, detail?: unknown): void {
    this.element.dispatchEvent(new CustomEvent(`dimmi:${name}`, { detail, bubbles: true, composed: true }));
  }
}

/*
 * The registry of controllers inside one root, a document or a shadow root.
 * start binds every data-controller element found, and the actions they
 * declare reach them through the root's delegated listeners.
 */
export class Application {
  readonly #root: ParentNode;
  readonly #definitions = new Map<string, ControllerClass>();
  readonly #controllers = new Map<HTMLElement, Map<string, Controller>>();

  constructor(root: ParentNode) {
    this.#root = root;
  }

  register(name: string, definition: ControllerClass): this {
    this.#definitions.set(name, definition);
    return this;
  }

  start(events: string[] = ["click", "input", "change", "submit", "keydown"]): void {
    for (const type of events) {
      this.#root.addEventListener(type, (event) => this.#handle(event));
    }
    this.connect(this.#root);
  }

  connect(scope: ParentNode): void {
    for (const element of scope.querySelectorAll<HTMLElement>("[data-controller]")) {
      for (const name of (element.dataset.controller ?? "").split(" ")) {
        this.#bind(element, name);
      }
    }
  }

  stop(): void {
    for (const controllers of this.#controllers.values()) {
      for (const controller of controllers.values()) {
        controller.disconnect?.();
      }
    }
    this.#controllers.clear();
  }

  controller<T extends Controller>(element: HTMLElement, name: string): T | undefined {
    return this.#controllers.get(element)?.get(name) as T | undefined;
  }

  #bind(element: HTMLElement, name: string): void {
    const definition = this.#definitions.get(name);
    const bound = this.#controllers.get(element) ?? new Map<string, Controller>();
    if (!definition || bound.has(name)) {
      return;
    }

    const controller = new definition(element, this.#root);
    bound.set(name, controller);
    this.#controllers.set(element, bound);
    controller.connect?.();
  }

  /*
   * handle runs the actions of the nearest element around the event's
   * target that answers this kind of event, so a key pressed on a button
   * that only answers clicks still reaches the panel around it.
   */
  #handle(event: Event): void {
    let source = event.target instanceof Element ? event.target.closest<HTMLElement>("[data-action]") : null;
    while (source) {
      const methods = (source.dataset.action ?? "")
        .split(" ")
        .map((descriptor) => descriptor.split("->"))
        .filter(([type, method]) => type === event.type && method)
        .map(([, method]) => method as string);
      if (methods.length > 0) {
        for (const method of methods) {
          this.#invoke(source, method, event);
        }
        return;
      }
      source = source.parentElement?.closest<HTMLElement>("[data-action]") ?? null;
    }
  }

  /*
   * invoke calls the action on the nearest controller around the element
   * that has it, so an action inside a nested controller still reaches an
   * outer one when the inner one does not handle it.
   */
  #invoke(source: HTMLElement, method: string, event: Event): void {
    let host = source.closest<HTMLElement>("[data-controller]");
    while (host) {
      for (const controller of this.#controllers.get(host)?.values() ?? []) {
        const action = (controller as unknown as Record<string, unknown>)[method];
        if (typeof action === "function") {
          action.call(controller, event);
          return;
        }
      }
      host = host.parentElement?.closest<HTMLElement>("[data-controller]") ?? null;
    }
  }
}
