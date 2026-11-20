/*
 * Scenes are product moments told as a short list of steps. The
 * prerendered HTML shows the last step, the finished picture, so search
 * engines, the first paint and anyone who asked for reduced motion see a
 * complete, readable mock. In the browser the scene rewinds to its first
 * step when it scrolls into view and plays forward, one step per tick,
 * then holds or starts again. It pauses while offscreen.
 */
import { useEffect, useRef, useState } from "react";

export function useScene(steps: number, options: { interval?: number | number[]; loop?: boolean; hold?: number } = {}) {
  const { interval = 1100, loop = true, hold = 3200 } = options;
  const ref = useRef<HTMLDivElement>(null);
  const [step, setStep] = useState(steps - 1);
  const [playing, setPlaying] = useState(false);
  const started = useRef(false);

  useEffect(() => {
    const el = ref.current;
    if (!el || window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      return;
    }
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting && !started.current) {
          started.current = true;
          setStep(0);
        }
        setPlaying(entry.isIntersecting);
      },
      { threshold: 0.35 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (!playing || !started.current) {
      return;
    }
    const last = step >= steps - 1;
    if (last && !loop) {
      return;
    }
    const wait = last ? hold : Array.isArray(interval) ? (interval[step] ?? 1100) : interval;
    const timer = setTimeout(() => setStep(last ? 0 : step + 1), wait);
    return () => clearTimeout(timer);
  }, [playing, step, steps, loop, hold, interval]);

  // Restarting only rewinds a scene that is already playing; a scene that
  // has not been reached, or one held still for reduced motion, keeps its
  // finished picture.
  const restart = () => {
    if (started.current) {
      setStep(0);
    }
  };

  // Jumping moves to any step and plays on from there, as when a visitor
  // picks a tab; a scene held still for reduced motion stays on that step.
  const goTo = (target: number) => {
    started.current = true;
    setStep(Math.max(0, Math.min(steps - 1, target)));
  };

  return { ref, step, restart, goTo };
}

/*
 * useStartInView reports when an element first scrolls into view and
 * whether it is in view now. Scenes start from their finished picture
 * and only rewind once started is true; reduced motion never starts.
 */
export function useStartInView<T extends HTMLElement = HTMLDivElement>(threshold = 0.35) {
  const ref = useRef<T>(null);
  const [started, setStarted] = useState(false);
  const [inView, setInView] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el || window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      return;
    }
    const observer = new IntersectionObserver(
      ([entry]) => {
        setInView(entry.isIntersecting);
        if (entry.isIntersecting) {
          setStarted(true);
        }
      },
      { threshold },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [threshold]);

  return { ref, started, inView };
}
