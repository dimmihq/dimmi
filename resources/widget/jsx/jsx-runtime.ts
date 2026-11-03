/*
 * The widget's JSX runtime. Components are plain functions that return
 * HTML, so the markup reads like the page it makes while the widget stays
 * a few kilobytes of vanilla JavaScript: there is no virtual DOM and no
 * re-rendering, only strings the messenger hands to innerHTML once.
 *
 * Every value placed in markup is escaped, text and attributes alike, so
 * customer text can never become markup. Only another element, or markup
 * wrapped on purpose with raw(), passes through unescaped. Attributes set
 * to false, null or undefined are left out and true writes the bare name.
 * Attributes are written the way React spells them, className, htmlFor
 * and strokeWidth, so the usual JSX linting applies; they come out as the
 * HTML names class, for and stroke-width.
 * There are no inline styles: a strict content security policy would
 * block them, so styling goes through classes and the stylesheet.
 */

export class Html {
  readonly value: string;

  constructor(value: string) {
    this.value = value;
  }

  toString(): string {
    return this.value;
  }
}

export type Child = Html | string | number | boolean | null | undefined | Child[];

type Props = Record<string, unknown> & { children?: Child };

type Component = (props: Props) => Html;

const voids = new Set(["area", "br", "col", "hr", "img", "input", "meta", "source", "wbr"]);

const entities: Record<string, string> = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };

export function escapeHtml(text: string): string {
  return text.replace(/[&<>"']/g, (char) => entities[char] ?? char);
}

/*
 * raw marks markup the widget wrote itself, such as an icon's SVG, as safe
 * to place without escaping. Never pass it text that came from a customer.
 */
export function raw(markup: string): Html {
  return new Html(markup);
}

function children(child: Child): string {
  if (child instanceof Html) {
    return child.value;
  }
  if (Array.isArray(child)) {
    return child.map(children).join("");
  }
  if (child === null || child === undefined || typeof child === "boolean") {
    return "";
  }

  return escapeHtml(String(child));
}

const renamed: Record<string, string> = { className: "class", htmlFor: "for" };

/*
 * name turns a React attribute name into the HTML one: className and
 * htmlFor, and SVG presentation attributes such as strokeWidth, which
 * HTML spells with a dash.
 */
function name(prop: string): string {
  return (
    renamed[prop] ??
    prop.replace(
      /^(stroke|fill|clip|stop|font)([A-Z])/,
      (_, head: string, next: string) => `${head}-${next.toLowerCase()}`,
    )
  );
}

function attributes(props: Props): string {
  let out = "";
  for (const [prop, value] of Object.entries(props)) {
    if (prop === "children" || prop === "key" || value === false || value === null || value === undefined) {
      continue;
    }
    out += value === true ? ` ${name(prop)}` : ` ${name(prop)}="${escapeHtml(String(value))}"`;
  }

  return out;
}

export function jsx(type: string | Component, props: Props): Html {
  if (typeof type === "function") {
    return type(props);
  }

  const open = `<${type}${attributes(props)}>`;

  return new Html(voids.has(type) ? open : `${open}${children(props.children)}</${type}>`);
}

export const jsxs = jsx;

export const jsxDEV = jsx;

export function Fragment({ children: content }: Props): Html {
  return new Html(children(content));
}

/*
 * The types TypeScript checks JSX against: any element with any
 * attribute, each element producing Html, and a key on anything rendered
 * from a list, which the runtime ignores.
 */
export namespace JSX {
  export type Element = Html;
  export interface IntrinsicElements {
    [name: string]: Record<string, unknown>;
  }
  export interface ElementChildrenAttribute {
    children: unknown;
  }
  export interface IntrinsicAttributes {
    key?: string | number;
  }
}
