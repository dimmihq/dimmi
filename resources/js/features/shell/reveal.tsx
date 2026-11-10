/*
 * The motion BoardUI's agent blocks use when something lands: it softens
 * in from a blur and a short lift while its height opens, so the page
 * grows instead of jumping, and it folds away the same way. Reduced
 * motion shows and hides it at once.
 */
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import type { ReactNode } from "react";

import { cx } from "@/utils/cx";

const ease = [0.22, 1, 0.36, 1] as const;
const hidden = { opacity: 0, height: 0, y: 4, filter: "blur(6px)" };
const shown = { opacity: 1, height: "auto", y: 0, filter: "blur(0px)" };
const timing = {
  height: { duration: 0.38, ease },
  opacity: { duration: 0.42, ease },
  filter: { duration: 0.42, ease },
  y: { duration: 0.42, ease },
};

/*
 * Reveal shows its children landing when it mounts; animate false shows
 * them at once, for what was already there when the page opened.
 */
export function Reveal({
  animate = true,
  className,
  children,
}: {
  animate?: boolean;
  className?: string;
  children: ReactNode;
}) {
  const still = useReducedMotion() ?? false;

  return (
    <motion.div
      initial={animate && !still ? hidden : false}
      animate={shown}
      transition={timing}
      className={cx("overflow-hidden", className)}
    >
      {children}
    </motion.div>
  );
}

/*
 * Unfold opens and folds its children in place as open changes.
 */
export function Unfold({ open, className, children }: { open: boolean; className?: string; children: ReactNode }) {
  const still = useReducedMotion() ?? false;

  return (
    <AnimatePresence initial={false}>
      {open ? (
        <motion.div
          key="unfold"
          initial={still ? false : hidden}
          animate={shown}
          exit={still ? { opacity: 0 } : hidden}
          transition={timing}
          className={cx("overflow-hidden", className)}
        >
          {children}
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}
