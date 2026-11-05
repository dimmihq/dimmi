/*
 * A tag's chip in its own color, the way BoardUI tints its chips: a pale
 * tint of the color behind dark text of the same hue in light mode, and a
 * deep tint behind bright text in dark mode. A tag's color is anything a
 * person picks, so the text is pushed further toward black or white until
 * it reads against its tint, and every tag stays legible.
 */

import type { CSSProperties } from "react";

type Rgb = [number, number, number];

const white: Rgb = [255, 255, 255];
const black: Rgb = [0, 0, 0];
const lightSurface: Rgb = [255, 255, 255];
const darkSurface: Rgb = [23, 23, 23];
const readable = 4.5;

function parse(hex: string): Rgb | null {
  const match = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
  if (!match) {
    return null;
  }
  const value = Number.parseInt(match[1], 16);

  return [(value >> 16) & 255, (value >> 8) & 255, value & 255];
}

function mix(color: Rgb, toward: Rgb, amount: number): Rgb {
  return color.map((channel, i) => Math.round(channel + (toward[i] - channel) * amount)) as Rgb;
}

function luminance([r, g, b]: Rgb): number {
  const linear = (channel: number) => {
    const c = channel / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  };

  return 0.2126 * linear(r) + 0.7152 * linear(g) + 0.0722 * linear(b);
}

/*
 * contrast is the WCAG contrast ratio of two colors.
 */
export function contrast(a: Rgb, b: Rgb): number {
  const [light, dark] = [luminance(a), luminance(b)].sort((x, y) => y - x);

  return (light + 0.05) / (dark + 0.05);
}

function css([r, g, b]: Rgb, alpha = 1): string {
  return alpha === 1 ? `rgb(${r} ${g} ${b})` : `rgb(${r} ${g} ${b} / ${alpha})`;
}

/*
 * readableOn walks the color toward black or white until it reaches a
 * readable contrast on the background.
 */
function readableOn(color: Rgb, background: Rgb, toward: Rgb): Rgb {
  for (let amount = 0; amount <= 1; amount += 0.05) {
    const candidate = mix(color, toward, amount);
    if (contrast(candidate, background) >= readable) {
      return candidate;
    }
  }

  return toward;
}

export interface TagTint {
  background: string;
  text: string;
}

export interface TagColors {
  light: TagTint;
  dark: TagTint;
}

/*
 * tagColors is the chip's background and text in light and dark mode for
 * a tag's color; a tag without a usable color gets null and keeps the
 * neutral chip.
 */
export function tagColors(hex: string | null): TagColors | null {
  const color = hex ? parse(hex) : null;
  if (!color) {
    return null;
  }

  const lightBackground = mix(color, white, 0.8);
  const darkTint = mix(color, black, 0.8);
  const darkAlpha = 0.6;
  const darkBackground = mix(darkSurface, darkTint, darkAlpha);

  return {
    light: { background: css(lightBackground), text: css(readableOn(mix(color, black, 0.45), lightBackground, black)) },
    dark: {
      background: css(darkTint, darkAlpha),
      text: css(readableOn(mix(color, white, 0.15), darkBackground, white)),
    },
  };
}

function tintStyle(color: string | null): CSSProperties | undefined {
  const colors = tagColors(color);
  if (!colors) {
    return undefined;
  }

  return {
    "--tag-light-background": colors.light.background,
    "--tag-light-text": colors.light.text,
    "--tag-dark-background": colors.dark.background,
    "--tag-dark-text": colors.dark.text,
  } as CSSProperties;
}

/*
 * tagTint is the style and classes that paint an element in a tag's
 * color, for anything shaped like a tag: the chip, its icon and the
 * suggestions that become tags.
 */
export function tagTint(color: string | null): { style?: CSSProperties; className: string } {
  return { style: tintStyle(color), className: color ? tinted : neutral };
}

const tinted = [
  "bg-(--tag-light-background) text-(--tag-light-text) dark:bg-(--tag-dark-background) dark:text-(--tag-dark-text)",
  "ring-1 ring-current/30 ring-inset",
].join(" ");
const neutral = "bg-background-tertiary-default text-text-primary ring-1 ring-border-button-default ring-inset";

/*
 * tagPalette is the set of colours offered first, the same the server
 * hands out to a tag created without one.
 */
export const tagPalette = [
  "#2563eb",
  "#16a34a",
  "#dc2626",
  "#9333ea",
  "#ea580c",
  "#0891b2",
  "#db2777",
  "#ca8a04",
  "#4f46e5",
  "#0d9488",
  "#65a30d",
  "#c026d3",
];

export const testing = { parse, mix, darkSurface, lightSurface };
