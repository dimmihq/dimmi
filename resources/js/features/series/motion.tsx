/*
 * The series builder's motion, on BoardUI's language. Numbers that come
 * and go with the results switch open and close their own height while
 * they fade and sharpen, so nothing beside them jumps. A new selection
 * in the inspector rises a few pixels into place. Reduced motion turns
 * all of it into plain appearance.
 */
import { AnimatePresence, motion, type Transition, useReducedMotion } from "motion/react";
import type { ReactNode } from "react";

const settle = [0.22, 1, 0.36, 1] as const;

function useTiming(): { arrive: Transition; leave: Transition } {
  const still = useReducedMotion();
  if (still) {
    return { arrive: { duration: 0 }, leave: { duration: 0 } };
  }

  return { arrive: { duration: 0.28, ease: settle }, leave: { duration: 0.18, ease: "easeIn" } };
}

/*
 * Unfold shows its children while shown is true, opening to their
 * height on the way in and closing on the way out.
 */
export function Unfold({ shown, className, children }: { shown: boolean; className?: string; children: ReactNode }) {
  const timing = useTiming();

  return (
    <AnimatePresence initial={false}>
      {shown ? (
        <motion.div
          initial={{ height: 0, opacity: 0, filter: "blur(4px)" }}
          animate={{ height: "auto", opacity: 1, filter: "blur(0px)", transition: timing.arrive }}
          exit={{ height: 0, opacity: 0, filter: "blur(4px)", transition: timing.leave }}
          className={className}
          style={{ overflow: "hidden" }}
        >
          {children}
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}

/*
 * Rise brings in what it wraps each time view changes: a fade, a few
 * pixels up and a small blur that sharpens.
 */
export function Rise({ view, className, children }: { view: string; className?: string; children: ReactNode }) {
  const timing = useTiming();

  return (
    <AnimatePresence initial={false} mode="wait">
      <motion.div
        key={view}
        initial={{ opacity: 0, y: 6, filter: "blur(3px)" }}
        animate={{ opacity: 1, y: 0, filter: "blur(0px)", transition: timing.arrive }}
        exit={{ opacity: 0, y: -4, filter: "blur(3px)", transition: timing.leave }}
        className={className}
      >
        {children}
      </motion.div>
    </AnimatePresence>
  );
}
