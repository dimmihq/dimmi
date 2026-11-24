/*
 * Agent Studio coming in as it opens: a fade and a short rise, on
 * BoardUI's ease-out. Reduced motion shows it at once.
 */
import { motion, useReducedMotion } from "motion/react";
import type { ReactNode } from "react";

const settle = [0.22, 1, 0.36, 1] as const;

export function StudioArrive({ children }: { children: ReactNode }) {
  const still = useReducedMotion() ?? false;

  return (
    <motion.div
      initial={still ? false : { opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.32, ease: settle }}
      className="flex h-full min-h-0 min-w-0 flex-1"
    >
      {children}
    </motion.div>
  );
}
