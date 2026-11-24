/*
 * The sounds a messenger plays: whether it plays any, and which built-in
 * sound marks the customer's message leaving and a reply arriving. Each
 * pick plays as it is chosen, and the button beside it plays it again;
 * the messenger draws the same sounds, so the preview is what customers
 * hear. Nothing saves until Save.
 */
import { RiPlayFill } from "@remixicon/react";

import { IconButton } from "@/components/base/buttons/icon-button";
import type { MessengerSound } from "@/lib/api/widget-settings";

import { play } from "../../../../widget/lib/sound";
import { Choice, Toggle } from "../controls";
import { Row, Section } from "../layout";
import { labelled, type TabProps } from "./parts";

const sounds = labelled<MessengerSound>({
  none: "None",
  pop: "Pop",
  chime: "Chime",
  bubble: "Bubble",
  swoosh: "Swoosh",
  tick: "Tick",
});

function SoundPick({
  label,
  value,
  disabled,
  onChange,
}: {
  label: string;
  value: MessengerSound;
  disabled: boolean;
  onChange: (sound: MessengerSound) => void;
}) {
  return (
    <>
      <Choice
        label={label}
        options={sounds.options}
        value={sounds.label(value)}
        disabled={disabled}
        onChange={(picked) => {
          const sound = sounds.value(picked) ?? "none";
          play(sound);
          onChange(sound);
        }}
      />
      <IconButton
        size="small"
        icon={RiPlayFill}
        aria-label={`Play ${label.toLowerCase()}`}
        disabled={value === "none"}
        onClick={() => play(value)}
      />
    </>
  );
}

export function SoundsSection({ value, set, locked }: TabProps) {
  const a = value.appearance;

  return (
    <Section title="Sounds">
      <Row label="Play sounds" description="The messenger also pops softly when it opens.">
        <Toggle
          label="Play sounds"
          on={a.sound}
          disabled={locked}
          onChange={(sound) => set({ appearance: { sound } })}
        />
      </Row>
      <Row label="Customer sends a message">
        <SoundPick
          label="Sent sound"
          value={a.sent_sound}
          disabled={locked || !a.sound}
          onChange={(sent_sound) => set({ appearance: { sent_sound } })}
        />
      </Row>
      <Row label="A reply arrives" description="From your team or the AI agent.">
        <SoundPick
          label="Reply sound"
          value={a.reply_sound}
          disabled={locked || !a.sound}
          onChange={(reply_sound) => set({ appearance: { reply_sound } })}
        />
      </Row>
    </Section>
  );
}
