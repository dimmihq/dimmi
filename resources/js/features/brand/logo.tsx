/*
 * The Dimmi logo as React components: the icon, a simpler icon for sizes
 * below 24px, the wordmark, and the icon and wordmark together. Every part
 * is filled with the current text color and has no background, so one
 * component serves light and dark mode: give it a text color, such as
 * text-text-primary, and a height, and the width follows.
 */
import type { ReactNode, SVGProps } from "react";

type LogoProps = Omit<SVGProps<SVGSVGElement>, "viewBox" | "children"> & { title?: string };

const iconPaths = [
  "M4,24 A20,20 0 0 1 17.714,5.014 L42.986,30.286 A20,20 0 0 1 24,44 L11,44 A7,7 0 0 1 4,37 Z",
  "M19.756,4.456 A20,20 0 0 1 24.713,4.013 L43.987,23.287 A20,20 0 0 1 43.544,28.244 Z",
  "M27.633,4.333 A20,20 0 0 1 34.99,7.29 L40.71,13.01 A20,20 0 0 1 43.667,20.367 Z",
];

const smallIconPaths = [
  "M4,24 A20,20 0 0 1 18.869,4.669 L43.331,29.131 A20,20 0 0 1 24,44 L11,44 A7,7 0 0 1 4,37 Z",
  "M21.91,4.11 A20,20 0 0 1 30.178,4.978 L43.022,17.822 A20,20 0 0 1 43.89,26.09 Z",
];

const letterD =
  "M315,0 L440,0 L440,640 L315,640 L315,585 C292.091,625 246.273,652 189,652 C74.455,652 0,570 0,440 " +
  "C0,332 68.727,240 189,240 C246.273,240 286.364,264 315,312 Z M315,432 C315,377 277,340 226.333,340 " +
  "C163,340 125,382 125,410 C125,510 163,552 226.333,552 C277,552 315,515 315,460 Z";

const letterI = "M0,0 H125 V118 H0 Z M0,240 H125 V640 H0 Z";

const letterM =
  "M0,640 L0,240 L125,240 L125,285 C153.111,250 198.089,228 259.933,228 C321.778,228 361.133,250 378,285 " +
  "C406.111,250 451.089,228 512.933,228 C574.778,228 631,270 631,350 L631,640 L506,640 L506,370 " +
  "C506,335 486.8,320 454.8,320 C416.4,320 378,345 378,385 L378,640 L253,640 L253,370 C253,335 233.8,320 201.8,320 " +
  "C163.4,320 125,345 125,385 L125,640 Z";

const letters: [string, string, number][] = [
  ["d", letterD, 0],
  ["i1", letterI, 478],
  ["m1", letterM, 641],
  ["m2", letterM, 1310],
  ["i2", letterI, 1979],
];

function Mark({ title = "Dimmi", viewBox, children, ...props }: LogoProps & { viewBox: string; children: ReactNode }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox={viewBox} role="img" fill="currentColor" {...props}>
      <title>{title}</title>
      <g fillRule="evenodd">{children}</g>
    </svg>
  );
}

function IconPaths({ paths }: { paths: string[] }) {
  return paths.map((d) => <path key={d} d={d} />);
}

function WordmarkPaths() {
  return letters.map(([key, d, x]) => <path key={key} d={d} transform={x ? `translate(${x},0)` : undefined} />);
}

/*
 * DimmiIcon is the speech bubble on its own, for 24px and up: app icons,
 * avatars and anywhere the name already shows nearby.
 */
export function DimmiIcon(props: LogoProps) {
  return (
    <Mark viewBox="0 0 40 40" {...props}>
      <g transform="translate(-4,-4)">
        <IconPaths paths={iconPaths} />
      </g>
    </Mark>
  );
}

/*
 * DimmiIconSmall draws the bubble with two thicker cuts, for sizes below
 * 24px, where the three thin cuts of the regular icon blur together.
 */
export function DimmiIconSmall(props: LogoProps) {
  return (
    <Mark viewBox="0 0 40 40" {...props}>
      <g transform="translate(-4,-4)">
        <IconPaths paths={smallIconPaths} />
      </g>
    </Mark>
  );
}

/*
 * DimmiWordmark is the hand-drawn name without the icon.
 */
export function DimmiWordmark(props: LogoProps) {
  return (
    <Mark viewBox="0 0 2104 652" {...props}>
      <WordmarkPaths />
    </Mark>
  );
}

/*
 * DimmiLogo is the icon followed by the wordmark, the primary form of the
 * logo.
 */
export function DimmiLogo(props: LogoProps) {
  return (
    <Mark viewBox="0 0 2880.632 656.632" {...props}>
      <g transform="translate(-63.333,-40.033) scale(15.8333)">
        <IconPaths paths={iconPaths} />
      </g>
      <g transform="translate(776.632,0)">
        <WordmarkPaths />
      </g>
    </Mark>
  );
}
