/*
 * A voice note's waveform, drawn in a thread's voice bubble and in the
 * composer while the visitor records.
 */
import { raw } from "widget-jsx/jsx-runtime";

import { levels } from "../text";

export function Waveform({ className = "wave" }: { className?: string }) {
  return <span className={className}>{raw(levels.map((level) => `<i data-height="${level}"></i>`).join(""))}</span>;
}
