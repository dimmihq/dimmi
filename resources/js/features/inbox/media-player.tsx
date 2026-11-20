/*
 * The thread's own players for audio and video, drawn from the design
 * system instead of the browser's. A voice note plays like one on a
 * phone: a round play button, its waveform filling as it plays, a drag
 * or a click anywhere on it to jump, and its time, all on one line. A video shows its first frame with a play button over it;
 * playing, its controls rest along the bottom and step back while the
 * pointer is away. A file the browser cannot play says so, and the link
 * to save it stays beside the player.
 */
import { RiFullscreenLine, RiPauseFill, RiPlayFill, RiVolumeMuteLine, RiVolumeUpLine } from "@remixicon/react";
import { type RefObject, useEffect, useMemo, useRef, useState } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { Slider } from "@/components/base/slider/slider";
import { cx } from "@/utils/cx";

/*
 * clock writes seconds as m:ss.
 */
function clock(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return "0:00";
  }

  const whole = Math.floor(seconds);
  return `${Math.floor(whole / 60)}:${String(whole % 60).padStart(2, "0")}`;
}

/*
 * useSteadySource keeps the link a player started with. The thread signs
 * its files' links afresh each time it reloads, and a player whose source
 * changed would start over; it moves to the newest link only after its
 * own stopped working.
 */
function useSteadySource(src: string) {
  const latest = useRef(src);
  latest.current = src;
  const [steady, setSteady] = useState(src);

  const recover = () => {
    if (latest.current === steady) {
      return false;
    }
    setSteady(latest.current);
    return true;
  };

  return { steady, recover };
}

/*
 * useMedia follows an audio or video element: whether it plays, where it
 * is, how long it is, and whether the browser could not play it. A file
 * that played to its end goes back to its start, ready to play again.
 */
function useMedia(ref: RefObject<HTMLMediaElement | null>, recover: () => boolean) {
  const [playing, setPlaying] = useState(false);
  const [current, setCurrent] = useState(0);
  const [duration, setDuration] = useState(0);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const media = ref.current;
    if (!media) {
      return;
    }

    const sync = () => {
      setPlaying(!media.paused && !media.ended);
      setCurrent(media.currentTime);
      if (Number.isFinite(media.duration)) {
        setDuration(media.duration);
      }
    };
    const fail = () => {
      if (!recover()) {
        setFailed(true);
      }
    };
    const rewind = () => {
      media.currentTime = 0;
      sync();
    };
    const events = ["play", "pause", "timeupdate", "loadedmetadata", "durationchange", "seeked"];

    for (const event of events) {
      media.addEventListener(event, sync);
    }
    media.addEventListener("error", fail);
    media.addEventListener("ended", rewind);

    return () => {
      for (const event of events) {
        media.removeEventListener(event, sync);
      }
      media.removeEventListener("error", fail);
      media.removeEventListener("ended", rewind);
    };
  }, [ref, recover]);

  const toggle = () => {
    const media = ref.current;
    if (!media) {
      return;
    }
    if (media.paused) {
      void media.play().catch(() => setFailed(true));
    } else {
      media.pause();
    }
  };

  const seek = (to: number) => {
    const media = ref.current;
    if (media && Number.isFinite(to)) {
      media.currentTime = to;
      setCurrent(to);
    }
  };

  return { playing, current, duration, failed, toggle, seek };
}

/*
 * waveform draws a voice note's bars, the same for the same file every
 * time, so the shape reads as its own without decoding the audio.
 */
function waveform(seed: string, count: number): number[] {
  let hash = 2166136261;
  for (const char of seed) {
    hash = Math.imul(hash ^ char.charCodeAt(0), 16777619);
  }

  return Array.from({ length: count }, (_, i) => {
    hash = Math.imul(hash ^ (i + 1), 16777619);
    const wave = Math.sin(i / 2.4) * 0.25 + 0.55;
    const noise = ((hash >>> 0) % 1000) / 1000;
    return Math.max(0.18, Math.min(1, wave * 0.6 + noise * 0.55));
  });
}

/*
 * VoicePlayer plays a voice note or other audio file in place.
 */
export function VoicePlayer({ src, label, seed }: { src: string; label: string; seed: string }) {
  const audio = useRef<HTMLAudioElement>(null);
  const { steady, recover } = useSteadySource(src);
  const { playing, current, duration, failed, toggle, seek } = useMedia(audio, recover);
  const bars = useMemo(() => waveform(seed, 36), [seed]);
  const progress = duration > 0 ? current / duration : 0;

  return (
    <div className="flex items-center gap-2.5">
      <audio ref={audio} preload="metadata" src={steady} className="hidden">
        <track kind="captions" />
      </audio>
      <IconButton
        variant="primary"
        size="small"
        icon={playing ? RiPauseFill : RiPlayFill}
        aria-label={playing ? `Pause ${label}` : `Play ${label}`}
        disabled={failed}
        onClick={toggle}
      />
      {failed ? (
        <span className="min-w-0 flex-1 text-body-2-regular text-text-tertiary">
          This browser cannot play this file.
        </span>
      ) : (
        <Slider
          size="tall"
          aria-label={`Position in ${label}`}
          minValue={0}
          maxValue={duration || 1}
          step={0.1}
          value={current}
          onChange={seek}
          className="min-w-0 flex-1"
          backdrop={
            <span aria-hidden className="absolute inset-0 flex items-center gap-[2px]">
              {bars.map((height, i) => (
                <span
                  // biome-ignore lint/suspicious/noArrayIndexKey: bars are a fixed drawing of the file
                  key={i}
                  className={cx(
                    "flex-1 rounded-full transition-colors duration-150",
                    i / bars.length < progress ? "bg-button-primary" : "bg-foreground-icon-quaternary",
                  )}
                  style={{ height: `${Math.round(height * 100)}%` }}
                />
              ))}
            </span>
          }
        />
      )}
      <span className="w-9 shrink-0 text-end text-caption-1-regular text-text-tertiary tabular-nums">
        {clock(playing || current > 0 ? current : duration)}
      </span>
    </div>
  );
}

/*
 * VideoPlayer plays a video in place, with controls over its bottom edge.
 */
export function VideoPlayer({ src, label }: { src: string; label: string }) {
  const frame = useRef<HTMLDivElement>(null);
  const video = useRef<HTMLVideoElement>(null);
  const { steady, recover } = useSteadySource(src);
  const { playing, current, duration, failed, toggle, seek } = useMedia(video, recover);
  const [muted, setMuted] = useState(false);

  useEffect(() => {
    if (video.current) {
      video.current.muted = muted;
    }
  }, [muted]);

  const fullscreen = () => {
    void frame.current?.requestFullscreen?.().catch(() => undefined);
  };

  if (failed) {
    return (
      <div className="flex h-40 w-72 max-w-full items-center justify-center rounded-xl bg-background-tertiary-default px-4 text-center text-body-2-regular text-text-tertiary">
        This browser cannot play this video. Save it to watch it.
      </div>
    );
  }

  return (
    <div ref={frame} className="group/video relative w-72 max-w-full overflow-hidden rounded-xl bg-black">
      <video
        ref={video}
        src={`${steady}#t=0.1`}
        preload="metadata"
        playsInline
        aria-label={label}
        onClick={toggle}
        className="block max-h-80 w-full cursor-pointer object-contain"
      >
        <track kind="captions" />
      </video>
      {playing ? null : (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <IconButton
            variant="glass"
            icon={RiPlayFill}
            aria-label={`Play ${label}`}
            onClick={toggle}
            className="pointer-events-auto size-12 [&>svg]:size-6"
          />
        </div>
      )}
      <div
        className={cx(
          "absolute inset-x-0 bottom-0 flex items-center gap-1.5 bg-gradient-to-t from-black/70 to-transparent px-2 pt-6 pb-1.5",
          "transition-opacity duration-200",
          playing ? "opacity-0 group-hover/video:opacity-100 group-focus-within/video:opacity-100" : "opacity-100",
        )}
      >
        <IconButton
          variant="glass"
          size="small"
          icon={playing ? RiPauseFill : RiPlayFill}
          aria-label={playing ? `Pause ${label}` : `Play ${label}`}
          onClick={toggle}
          className="size-7"
        />
        <Slider
          tone="onDark"
          aria-label={`Position in ${label}`}
          minValue={0}
          maxValue={duration || 1}
          step={0.1}
          value={current}
          onChange={seek}
          className="min-w-0 flex-1"
        />
        <span className="shrink-0 text-caption-1-regular text-text-white tabular-nums">
          {clock(current)} / {clock(duration)}
        </span>
        <IconButton
          variant="glass"
          size="small"
          icon={muted ? RiVolumeMuteLine : RiVolumeUpLine}
          aria-label={muted ? "Unmute" : "Mute"}
          onClick={() => setMuted((now) => !now)}
          className="size-7"
        />
        <IconButton
          variant="glass"
          size="small"
          icon={RiFullscreenLine}
          aria-label="Full screen"
          onClick={fullscreen}
          className="size-7"
        />
      </div>
    </div>
  );
}
