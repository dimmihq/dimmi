/*
 * The message composer: chips for attached files, the text box, and the
 * attach, emoji, voice and send buttons, with the recording bar that takes
 * their place while a voice note records, and a small picker of common
 * emoji above the text box, filled the first time it opens. The composer
 * controller brings it to life.
 */
import type { Options } from "../options";
import { ArrowUpIcon, MicIcon, PaperclipIcon, SmileIcon, TrashIcon } from "./icons";
import { Waveform } from "./thread";

/*
 * Chip is a file waiting to be sent, with a button to take it out again.
 */
export function Chip({ name, index }: { name: string; index: number }) {
  return (
    <span className="chip" data-index={index}>
      <span>{name}</span>
      <button type="button" data-action="click->unattach" aria-label={`Remove ${name}`}>
        ×
      </button>
    </span>
  );
}

export function Composer({ options }: { options: Options }) {
  const placeholder = options.texts.composer_placeholder;

  return (
    <form className="composer" data-controller="composer" data-action="submit->send">
      <fieldset className="emoji-picker" data-target="emoji" data-action="keydown->unpick" aria-label="Emoji" hidden />
      <div className="chips" data-target="chips" />
      <p className="composer-note" data-target="note" role="status" hidden />
      <textarea
        data-target="input"
        rows={1}
        placeholder={placeholder}
        aria-label={placeholder}
        data-action="input->resize keydown->enter"
      />
      <div className="actions">
        <label className="icon-button" aria-label="Attach files">
          <PaperclipIcon />
          <input type="file" multiple hidden data-action="change->attach" />
        </label>
        <button
          className="icon-button"
          type="button"
          data-action="click->emoji"
          data-target="emoji-button"
          aria-label="Add emoji"
          aria-expanded="false"
        >
          <SmileIcon />
        </button>
        <button className="icon-button" type="button" data-action="click->record" aria-label="Record a voice note">
          <MicIcon />
        </button>
        <button className="button primary round" type="submit" data-target="send" aria-label="Send" disabled>
          <ArrowUpIcon />
        </button>
      </div>
      <div className="recorder">
        <button className="icon-button" type="button" data-action="click->discard" aria-label="Discard voice note">
          <TrashIcon />
        </button>
        <span className="live-dot" />
        <time data-target="timer">0:00</time>
        <Waveform className="wave live" />
        <button className="button primary round" type="button" data-action="click->finish" aria-label="Send voice note">
          <ArrowUpIcon />
        </button>
      </div>
    </form>
  );
}
