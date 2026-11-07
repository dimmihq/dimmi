/*
 * The inbox's motion, on BoardUI's language. New things condense into
 * place with a fade, a small blur and BoardUI's ease-out; leaving is
 * faster and eases in. Views that swap inside a panel slide a few pixels
 * the way the person is heading, mirrored in right-to-left languages,
 * while the panel eases to its new height. Tiles scale in, and the ones beside them close the gap on BoardUI's
 * tight spring. Reduced motion turns all of it into plain appearance.
 */
import { AnimatePresence, motion, type Transition, useReducedMotion } from "motion/react";
import { type ReactNode, useLayoutEffect, useRef, useState } from "react";

import { useDirection } from "@/components/foundations/direction/direction";

const settle = [0.22, 1, 0.36, 1] as const;
const instant = { duration: 0 };

/*
 * useTiming returns BoardUI's timing for arriving, leaving and reflowing,
 * or no time at all when the person asked for reduced motion.
 */
function useTiming(arriving: number) {
  const still = useReducedMotion();
  if (still) {
    return { arrive: instant, leave: instant, reflow: instant } as const;
  }

  return {
    arrive: { duration: arriving, ease: settle } as Transition,
    leave: { duration: 0.15, ease: "easeIn" } as Transition,
    reflow: { type: "spring", stiffness: 520, damping: 42, mass: 0.7 } as Transition,
  };
}

/*
 * Arrive wraps a new item of a list, such as a chat message: it rises a
 * few pixels as it fades and sharpens in. Inside an AnimatePresence with
 * initial off, items present on first render do not animate.
 */
export function Arrive({ children }: { children: ReactNode }) {
  const timing = useTiming(0.3);

  return (
    <motion.li
      initial={{ opacity: 0, y: 4, filter: "blur(4px)" }}
      animate={{ opacity: 1, y: 0, filter: "blur(0px)", transition: timing.arrive }}
    >
      {children}
    </motion.li>
  );
}

/*
 * Swap shows one view of a panel at a time. When view changes, the old
 * one slides out and the new one slides in from the side the person is
 * heading: forward from the end, back from the start, sides that flip in
 * a right-to-left language. The direction is handed to the leaving view
 * as it leaves, so it always exits the opposite way. The className styles
 * each view's box, such as to let it fill a dialog.
 */
export function Swap({
  view,
  forward,
  className,
  children,
}: {
  view: string;
  forward: boolean;
  className?: string;
  children: ReactNode;
}) {
  const timing = useTiming(0.22);
  const ahead = useDirection() === "rtl" ? -12 : 12;
  const shift = forward ? ahead : -ahead;
  const moves = {
    enter: (by: number) => ({ opacity: 0, x: by, filter: "blur(2px)" }),
    shown: { opacity: 1, x: 0, filter: "blur(0px)", transition: timing.arrive },
    leave: (by: number) => ({ opacity: 0, x: -by, filter: "blur(2px)", transition: timing.leave }),
  };

  return (
    <AnimatePresence initial={false} mode="popLayout" custom={shift}>
      <motion.div
        key={view}
        custom={shift}
        variants={moves}
        initial="enter"
        animate="shown"
        exit="leave"
        className={className}
      >
        {children}
      </motion.div>
    </AnimatePresence>
  );
}

/*
 * Resize eases its height to whatever its content needs, so a panel
 * grows and shrinks smoothly as its view swaps.
 */
export function Resize({ children }: { children: ReactNode }) {
  const timing = useTiming(0.22);
  const content = useRef<HTMLDivElement>(null);
  const [height, setHeight] = useState<number | "auto">("auto");

  useLayoutEffect(() => {
    const element = content.current;
    if (!element) {
      return;
    }
    const watch = new ResizeObserver(([entry]) => setHeight(entry?.contentRect.height ?? "auto"));
    watch.observe(element);

    return () => watch.disconnect();
  }, []);

  return (
    <motion.div initial={false} animate={{ height }} transition={timing.arrive} className="relative overflow-hidden">
      <div ref={content}>{children}</div>
    </motion.div>
  );
}

/*
 * Pop wraps a tile: it scales in from slightly smaller with a small
 * blur, leaves the same way but faster, and its neighbours slide to
 * close the gap.
 */
export function Pop({ className, children }: { className?: string; children: ReactNode }) {
  const timing = useTiming(0.2);

  return (
    <motion.li
      layout="position"
      initial={{ opacity: 0, scale: 0.9, filter: "blur(2px)" }}
      animate={{ opacity: 1, scale: 1, filter: "blur(0px)", transition: timing.arrive }}
      exit={{ opacity: 0, scale: 0.9, filter: "blur(2px)", transition: timing.leave }}
      transition={{ layout: timing.reflow }}
      className={className}
    >
      {children}
    </motion.li>
  );
}
