/*
 * The sounds a messenger plays: whether it plays any, and whether a
 * message coming in and a message going out each make their sound. The
 * button beside each plays it; the messenger draws the same sounds, so
 * the preview is what customers hear. Turning one on plays it. Nothing
 * saves until Save.
 */
import { RiPlayFill } from "@remixicon/react";

import { IconButton } from "@/components/base/buttons/icon-button";

import { play, type Sound } from "../../../../widget/lib/sound";
import { Toggle } from "../controls";
import { Row, Section } from "../layout";
import type { TabProps } from "./parts";

/*
 * SoundSwitch turns one of the messenger's sounds on or off, with a
 * button that plays it.
 */
function SoundSwitch({
  label,
  sound,
  on,
  disabled,
  onChange,
}: {
  label: string;
  sound: Sound;
  on: boolean;
  disabled: boolean;
  onChange: (on: boolean) => void;
}) {
  return (
    <>
      <IconButton
        size="small"
        icon={RiPlayFill}
        aria-label={`Play ${label.toLowerCase()}`}
        onClick={() => play(sound)}
      />
      <Toggle
        label={label}
        on={on}
        disabled={disabled}
        onChange={(next) => {
          if (next) {
            play(sound);
          }
          onChange(next);
        }}
      />
    </>
  );
}

export function SoundsSection({ value, set, locked }: TabProps) {
  const a = value.appearance;

  return (
    <Section title="Sounds">
      <Row label="Play sounds">
        <Toggle
          label="Play sounds"
          on={a.sound}
          disabled={locked}
          onChange={(sound) => set({ appearance: { sound } })}
        />
      </Row>
      <Row label="Message comes in" description="From your team or the AI agent.">
        <SoundSwitch
          label="Incoming sound"
          sound="in"
          on={a.reply_sound !== "none"}
          disabled={locked || !a.sound}
          onChange={(on) => set({ appearance: { reply_sound: on ? "pop" : "none" } })}
        />
      </Row>
      <Row label="Message goes out" description="When the customer sends one.">
        <SoundSwitch
          label="Outgoing sound"
          sound="out"
          on={a.sent_sound !== "none"}
          disabled={locked || !a.sound}
          onChange={(on) => set({ appearance: { sent_sound: on ? "pop" : "none" } })}
        />
      </Row>
    </Section>
  );
}
