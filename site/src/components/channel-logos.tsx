/*
 * Real brand marks for every supported channel. Company logos keep their own
 * official colours in both light and dark mode instead of being tinted to the
 * site accent.
 */
import { Code } from "@phosphor-icons/react";
import { useId } from "react";

import { SlackMark } from "@/components/application/brand/slack-mark";
import { cx } from "@/utils/cx";

export type ChannelLogoProps = { className?: string };

export function MessengerBrandLogo({ className }: ChannelLogoProps) {
  return (
    <svg viewBox="0 0 28 28" aria-hidden className={className}>
      <rect width="28" height="28" rx="8" fill="#059669" />
      <path
        fill="#ffffff"
        d="M14 6.5c-4.3 0-7.75 3.1-7.75 6.95 0 2.05 1 3.9 2.6 5.15v2.9l2.65-1.45c.8.22 1.63.35 2.5.35 4.3 0 7.75-3.1 7.75-6.95S18.3 6.5 14 6.5Zm-2.75 8.1a1.15 1.15 0 1 1 0-2.3 1.15 1.15 0 0 1 0 2.3Zm2.75 0a1.15 1.15 0 1 1 0-2.3 1.15 1.15 0 0 1 0 2.3Zm2.75 0a1.15 1.15 0 1 1 0-2.3 1.15 1.15 0 0 1 0 2.3Z"
      />
    </svg>
  );
}

export function EmailBrandLogo({ className }: ChannelLogoProps) {
  return (
    <svg viewBox="0 0 28 28" aria-hidden className={className}>
      <path fill="#4285F4" d="M3.5 22.25h4.2V12.5L2 8.2v12.55c0 .83.67 1.5 1.5 1.5Z" />
      <path fill="#34A853" d="M20.3 22.25h4.2c.83 0 1.5-.67 1.5-1.5V8.2l-5.7 4.3v9.75Z" />
      <path fill="#FBBC04" d="M20.3 7.25v5.25L26 8.2V6.7c0-1.85-2.12-2.9-3.6-1.8l-2.1 2.35Z" />
      <path fill="#EA4335" d="M7.7 12.5V7.25L14 12l6.3-4.75v5.25L14 17.25 7.7 12.5Z" />
      <path fill="#C5221F" d="M2 6.7v1.5l5.7 4.3V7.25L5.6 4.9C4.12 3.8 2 4.85 2 6.7Z" />
    </svg>
  );
}

export function SlackBrandLogo({ className }: ChannelLogoProps) {
  return <SlackMark className={className} />;
}

export function WhatsAppBrandLogo({ className }: ChannelLogoProps) {
  return (
    <svg viewBox="0 0 28 28" aria-hidden className={className}>
      <path
        fill="#25D366"
        d="M14.04 2C7.42 2 2.04 7.36 2.04 13.96c0 2.11.55 4.17 1.6 5.99L2 26l6.23-1.63a11.96 11.96 0 0 0 5.81 1.48h.01c6.61 0 11.99-5.36 11.99-11.96A11.98 11.98 0 0 0 14.04 2Z"
      />
      <path
        fill="#ffffff"
        d="M19.86 16.92c-.3-.15-1.78-.88-2.06-.98-.28-.1-.48-.15-.68.15-.2.3-.78.98-.95 1.18-.18.2-.35.23-.65.08-.3-.15-1.27-.47-2.42-1.49-.9-.8-1.5-1.79-1.68-2.09-.18-.3-.02-.46.13-.61.14-.14.3-.35.45-.53.15-.18.2-.3.3-.5.1-.2.05-.38-.03-.53-.08-.15-.68-1.63-.93-2.23-.24-.59-.49-.5-.68-.51h-.58c-.2 0-.53.08-.8.38-.28.3-1.05 1.03-1.05 2.51s1.08 2.91 1.23 3.11c.15.2 2.12 3.24 5.14 4.54.72.31 1.28.5 1.72.64.72.23 1.38.2 1.9.12.58-.09 1.78-.73 2.03-1.43.25-.7.25-1.3.18-1.43-.08-.13-.28-.2-.58-.35Z"
      />
    </svg>
  );
}

export function InstagramBrandLogo({ className }: ChannelLogoProps) {
  const id = useId();
  return (
    <svg viewBox="0 0 28 28" aria-hidden className={className}>
      <defs>
        <radialGradient id={id} cx="28%" cy="105%" r="130%">
          <stop offset="0%" stopColor="#FEDA75" />
          <stop offset="25%" stopColor="#FA7E1E" />
          <stop offset="55%" stopColor="#D62976" />
          <stop offset="80%" stopColor="#962FBF" />
          <stop offset="100%" stopColor="#4F5BD5" />
        </radialGradient>
      </defs>
      <rect x="2" y="2" width="24" height="24" rx="6.5" fill={`url(#${id})`} />
      <rect x="6.5" y="6.5" width="15" height="15" rx="4.2" fill="none" stroke="#ffffff" strokeWidth="1.9" />
      <circle cx="14" cy="14" r="3.6" fill="none" stroke="#ffffff" strokeWidth="1.9" />
      <circle cx="18.3" cy="9.7" r="1.15" fill="#ffffff" />
    </svg>
  );
}

export function FacebookMessengerBrandLogo({ className }: ChannelLogoProps) {
  const id = useId();
  return (
    <svg viewBox="0 0 28 28" aria-hidden className={className}>
      <defs>
        <linearGradient id={id} x1="50%" y1="0%" x2="50%" y2="100%">
          <stop offset="0%" stopColor="#00B2FF" />
          <stop offset="100%" stopColor="#006AFF" />
        </linearGradient>
      </defs>
      <path
        fill={`url(#${id})`}
        d="M14 2C7.24 2 2 6.96 2 13.64c0 3.49 1.43 6.5 3.76 8.58.2.17.32.42.33.68l.07 2.13c.02.68.72 1.12 1.35.84l2.38-1.05c.2-.09.43-.11.65-.05 1.09.3 2.25.46 3.46.46 6.76 0 12-4.96 12-11.6C26 6.96 20.76 2 14 2Z"
      />
      <path
        fill="#ffffff"
        d="m21.2 10.67-3.52 5.58a1.8 1.8 0 0 1-2.6.48l-2.81-2.11a.72.72 0 0 0-.87 0l-3.8 2.88c-.51.39-1.17-.22-.83-.76l3.52-5.58a1.8 1.8 0 0 1 2.6-.48l2.81 2.11a.72.72 0 0 0 .87 0l3.8-2.88c.51-.39 1.17.22.83.76Z"
      />
    </svg>
  );
}

export function ApiBrandLogo({ className }: ChannelLogoProps) {
  return <Code weight="duotone" className={cx("text-accent-600 dark:text-accent-400", className)} aria-hidden />;
}
