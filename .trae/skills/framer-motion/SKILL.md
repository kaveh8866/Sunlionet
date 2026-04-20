---
name: "framer-motion"
description: "Guides building accessible, performant React UI animations with Framer Motion. Invoke when adding page transitions, gestures, or animated wizards in the website."
---

# Framer Motion (Website UI)

Use this skill when implementing animations in the Next.js website (React) with Framer Motion.

## Principles

- Prefer subtle motion that clarifies navigation state (step changes, progress, validation).
- Respect reduced-motion preferences.
- Keep transitions fast and interruptible (users can tap quickly on mobile).
- Animate layout changes rather than manually calculating positions.

## Building Blocks

- **motion components**: Replace `div`/`button`/`section` with `motion.div` / `motion.button` / `motion.section`.
- **AnimatePresence**: Animate route/step transitions when content is conditionally rendered.
- **layout / layoutId**: Smoothly animate shared elements between steps.
- **Gestures**: Use `whileTap`, `whileHover` (guarded for touch), and optional `drag` where it improves usability.

## Accessibility and UX

- Keep focus visible and stable after transitions (focus the page heading on step change).
- Don’t rely on motion alone to communicate state; pair with text/labels.
- Use `useReducedMotion()` to disable or simplify transitions.

## Wizard Patterns (Recommended)

### Step transitions

- Wrap step content with `AnimatePresence mode="wait" initial={false}`.
- Use a consistent variant for enter/exit:
  - enter: `{ opacity: 0, y: 10 } → { opacity: 1, y: 0 }`
  - exit: `{ opacity: 0, y: -10 }`

### Progress + layout

- Animate the progress bar width with `motion.div` and a spring transition.
- Use `layout` on container cards so content expansion feels natural.

### Touch optimization

- Use `whileTap={{ scale: 0.98 }}` for primary buttons.
- Avoid `whileHover` on coarse pointers; make hover effects conditional.

## Examples

### Reduced motion

```tsx
import { motion, useReducedMotion } from "framer-motion";

export function MotionCard({ children }: { children: React.ReactNode }) {
  const reduced = useReducedMotion();
  return (
    <motion.div
      initial={reduced ? false : { opacity: 0, y: 8 }}
      animate={reduced ? { opacity: 1 } : { opacity: 1, y: 0 }}
      transition={reduced ? { duration: 0 } : { type: "spring", stiffness: 380, damping: 32 }}
    >
      {children}
    </motion.div>
  );
}
```

