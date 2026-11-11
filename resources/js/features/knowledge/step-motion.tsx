/*
 * Moving between the steps of adding a source, on BoardUI's motion: the
 * new step slides a few pixels in from the side it comes from, fading
 * and sharpening from a small blur, while the old one leaves the other
 * way. Going back reverses the direction, a right-to-left page mirrors
 * it, and reduced motion turns it into a plain swap.
 */
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import type { ReactNode } from "react";

import { useDirection } from "@/components/foundations/direction/direction";

const settle = [0.22, 1, 0.36, 1] as const;

/*
 * StepMotion shows the step named by view, coming in forward when
 * forward is true and backward otherwise.
 */
export function StepMotion({
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
  const still = useReducedMotion();
  const rtl = useDirection() === "rtl";
  const side = (forward ? 1 : -1) * (rtl ? -1 : 1);
  const shift = still ? 0 : 16;

  return (
    <AnimatePresence mode="wait" initial={false} custom={side}>
      <motion.div
        key={view}
        custom={side}
        variants={{
          enter: (s: number) => ({ opacity: 0, x: s * shift, filter: still ? "blur(0px)" : "blur(4px)" }),
          shown: { opacity: 1, x: 0, filter: "blur(0px)" },
          leave: (s: number) => ({ opacity: 0, x: -s * shift * 0.75, filter: still ? "blur(0px)" : "blur(4px)" }),
        }}
        initial="enter"
        animate="shown"
        exit="leave"
        transition={{ duration: still ? 0 : 0.24, ease: settle }}
        className={className}
      >
        {children}
      </motion.div>
    </AnimatePresence>
  );
}
