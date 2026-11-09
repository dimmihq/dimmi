/*
 * How wide a picture is and where it sits, kept in its address's
 * fragment, as in "/plans.png#width=60&align=center", so the Markdown
 * stays plain and the help center reads the same thing. Widths are a
 * share of the text column, rounded to steps of five; a picture with no
 * width shows at its own size, never wider than the column. Only a
 * fragment made of these two settings is read as a layout: any other
 * stays part of the address, as the help center does.
 */

export type ImageAlign = "start" | "center" | "end";

export interface ImageLayout {
  width: number | null;
  align: ImageAlign | null;
}

const aligns: ImageAlign[] = ["start", "center", "end"];

/*
 * roundWidth bounds a width to the column and rounds it to its step.
 */
export function roundWidth(width: number): number {
  return Math.round(Math.min(Math.max(width, 20), 100) / 5) * 5;
}

function layoutOf(fragment: string): ImageLayout | null {
  const values = new URLSearchParams(fragment);
  const layout: ImageLayout = { width: null, align: null };
  let any = false;
  for (const [key, value] of values) {
    any = true;
    if (key === "width" && /^\d+$/.test(value)) {
      layout.width = roundWidth(Number(value));
    } else if (key === "align" && aligns.includes(value as ImageAlign)) {
      layout.align = value as ImageAlign;
    } else {
      return null;
    }
  }

  return any ? layout : null;
}

/*
 * splitLayout separates a picture's address from the layout in its
 * fragment.
 */
export function splitLayout(href: string): { src: string } & ImageLayout {
  const at = href.lastIndexOf("#");
  const layout = at < 0 ? null : layoutOf(href.slice(at + 1));
  if (!layout) {
    return { src: href, width: null, align: null };
  }

  return { src: href.slice(0, at), ...layout };
}

/*
 * joinLayout puts the layout back into the address, leaving out
 * whatever is at its default: its own size and the start of the line.
 */
export function joinLayout(src: string, { width, align }: ImageLayout): string {
  const parts = [width === null ? "" : `width=${width}`, align ? `align=${align}` : ""].filter(Boolean);

  return parts.length === 0 ? src : `${src}#${parts.join("&")}`;
}

/*
 * layoutClasses are the classes the help center's stylesheet sizes and
 * places a picture by, the same ones readers' pages carry.
 */
export function layoutClasses({ width, align }: ImageLayout): string {
  return [width === null ? "" : `image-width-${width}`, align ? `image-align-${align}` : ""].filter(Boolean).join(" ");
}
