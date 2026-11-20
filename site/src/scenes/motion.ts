/* How scene pieces arrive: a fade, a small rise and a blur that clears, on BoardUI's ease-out. */
export const enter = {
  initial: { opacity: 0, y: 8, filter: "blur(3px)" },
  animate: { opacity: 1, y: 0, filter: "blur(0px)" },
  transition: { duration: 0.45, ease: [0.22, 1, 0.36, 1] as const },
};
