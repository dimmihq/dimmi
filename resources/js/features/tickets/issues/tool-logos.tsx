/*
 * The marks of the tools a ticket links to, drawn inline so they never
 * load from elsewhere. Linear gets its own mark in its own indigo; a tool
 * without a mark here gets a neutral one. ToolMark sits beside an issue
 * key in a row; ToolTile is the mark in white on the tool's colour, the
 * same tile the integrations page draws. Linear, Jira, incident.io and
 * Rootly draw their own marks in their own colours.
 */
import { RiGithubFill, RiPlugLine } from "@remixicon/react";
import type { ComponentType } from "react";

import { cx } from "@/utils/cx";

type Mark = ComponentType<{ className?: string }>;

/*
 * LinearMark is Linear's logo in the current text colour.
 */
export function LinearMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={className} fill="currentColor">
      <path d="M2.886 4.18A11.982 11.982 0 0 1 11.99 0C18.624 0 24 5.376 24 12.009c0 3.64-1.62 6.903-4.18 9.105L2.887 4.18ZM1.817 5.626l16.556 16.556c-.524.33-1.075.62-1.65.866L.951 7.277c.247-.575.537-1.126.866-1.65ZM.322 9.163l14.515 14.515c-.71.172-1.443.282-2.195.322L0 11.358a12 12 0 0 1 .322-2.195Zm-.17 4.862 9.823 9.824a12.02 12.02 0 0 1-9.824-9.824Z" />
    </svg>
  );
}

/*
 * IntercomMark is Intercom's logo in the current text colour.
 */
export function IntercomMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={className} fill="currentColor">
      <path d="M21 0H3C1.343 0 0 1.343 0 3v18c0 1.658 1.343 3 3 3h18c1.658 0 3-1.342 3-3V3c0-1.657-1.342-3-3-3zm-5.801 4.399c0-.44.36-.8.802-.8.44 0 .8.36.8.8v10.688c0 .442-.36.801-.8.801-.443 0-.802-.359-.802-.801V4.399zM11.2 3.994c0-.44.357-.799.8-.799s.8.359.8.799v11.602c0 .44-.357.8-.8.8s-.8-.36-.8-.8V3.994zm-4 .405c0-.44.359-.8.799-.8.443 0 .802.36.802.8v10.688c0 .442-.36.801-.802.801-.44 0-.799-.359-.799-.801V4.399zM3.199 6c0-.442.36-.8.802-.8.44 0 .799.358.799.8v7.195c0 .441-.359.8-.799.8-.443 0-.802-.36-.802-.8V6zM20.52 18.202c-.123.105-3.086 2.593-8.52 2.593-5.433 0-8.397-2.486-8.521-2.593-.335-.288-.375-.792-.086-1.128.285-.334.79-.375 1.125-.09.047.041 2.693 2.211 7.481 2.211 4.848 0 7.456-2.186 7.479-2.207.334-.289.839-.25 1.128.086.289.336.25.84-.086 1.128zm.281-5.007c0 .441-.36.8-.801.8-.441 0-.801-.36-.801-.8V6c0-.442.361-.8.801-.8.441 0 .801.357.801.8v7.195z" />
    </svg>
  );
}

/*
 * ZendeskMark is Zendesk's logo in the current text colour.
 */
export function ZendeskMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={className} fill="currentColor">
      <path d="M12.914 2.904V16.29L24 2.905H12.914zM0 2.906C0 5.966 2.483 8.45 5.543 8.45s5.542-2.484 5.543-5.544H0zm11.086 4.807L0 21.096h11.086V7.713zm7.37 7.84c-3.063 0-5.542 2.48-5.542 5.543H24c0-3.06-2.48-5.543-5.543-5.543z" />
    </svg>
  );
}

/*
 * NotionMark is Notion's logo in the current text colour.
 */
export function NotionMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={className} fill="currentColor">
      <path d="M4.459 4.208c.746.606 1.026.56 2.428.466l13.215-.793c.28 0 .047-.28-.046-.326L17.86 1.968c-.42-.326-.981-.7-2.055-.607L3.01 2.295c-.466.046-.56.28-.374.466zm.793 3.08v13.904c0 .747.373 1.027 1.214.98l14.523-.84c.841-.046.935-.56.935-1.167V6.354c0-.606-.233-.933-.748-.887l-15.177.887c-.56.047-.747.327-.747.933zm14.337.745c.093.42 0 .84-.42.888l-.7.14v10.264c-.608.327-1.168.514-1.635.514-.748 0-.935-.234-1.495-.933l-4.577-7.186v6.952L12.21 19s0 .84-1.168.84l-3.222.186c-.093-.186 0-.653.327-.746l.84-.233V9.854L7.822 9.76c-.094-.42.14-1.026.793-1.073l3.456-.233 4.764 7.279v-6.44l-1.215-.139c-.093-.514.28-.887.747-.933zM1.936 1.035l13.31-.98c1.634-.14 2.055-.047 3.082.7l4.249 2.986c.7.513.934.653.934 1.213v16.378c0 1.026-.373 1.634-1.68 1.726l-15.458.934c-.98.047-1.448-.093-1.962-.747l-3.129-4.06c-.56-.747-.793-1.306-.793-1.96V2.667c0-.839.374-1.54 1.447-1.632z" />
    </svg>
  );
}

/*
 * PagerDutyMark is PagerDuty's logo in the current text colour.
 */
export function PagerDutyMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={className} fill="currentColor">
      <path d="M16.965 1.18C15.085.164 13.769 0 10.683 0H3.73v14.55h6.926c2.743 0 4.8-.164 6.61-1.37 1.975-1.303 3.004-3.484 3.004-6.007 0-2.716-1.262-4.896-3.305-5.994zm-5.5 10.326h-4.21V3.113l3.977-.027c3.62-.028 5.43 1.234 5.43 4.128 0 3.113-2.248 4.292-5.197 4.292zM3.73 17.61h3.525V24H3.73Z" />
    </svg>
  );
}

/*
 * JiraMark is Jira's mark in the current text colour.
 */
export function JiraMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={className} fill="currentColor">
      <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.736a5.215 5.215 0 0 0 5.215 5.214h2.129v2.058a5.218 5.218 0 0 0 5.215 5.214V6.758a1.001 1.001 0 0 0-1.001-1.001zM23.013 0H11.455a5.215 5.215 0 0 0 5.215 5.215h2.129v2.057A5.215 5.215 0 0 0 24 12.483V1.005A1.001 1.001 0 0 0 23.013 0Z" />
    </svg>
  );
}

/*
 * IncidentIOMark is incident.io's flame in the current text colour.
 */
export function IncidentIOMark({ className }: { className?: string }) {
  return (
    <svg viewBox="-45 0 346 346" aria-hidden className={className} fill="currentColor">
      <path d="M96.73 344.737V298.16h60.969v46.99C214.062 331.68 256 280.686 256 219.843c0-49.516-35.14-119.19-75.886-140.748c5.48 19.333-2.749 44.528-13.71 42.32c-4.13-.826-4.56-7.355-5.226-17.903C159.78 81.43 157.27 41.716 117.015 0c-3.209 29.135-49.405 88.484-79.46 127.118a3238 3238 0 0 0-11.28 14.536A129.25 129.25 0 0 0 0 219.844c-.12 58.859 39.713 110.296 96.73 124.91zm65.147-98.952a34.09 34.09 0 0 1-33.996 34.202a34.09 34.09 0 0 1-33.996-34.202c0-7.8 2.685-15.14 6.99-20.763l2.987-3.86c7.99-10.262 20.254-26.021 21.112-33.757c25.306 17.093 36.903 45.226 36.903 58.38" />
    </svg>
  );
}

/*
 * RootlyMark is Rootly's sprout in the current text colour.
 */
export function RootlyMark({ className }: { className?: string }) {
  return (
    <svg viewBox="5.2 5.6 21.8 21.8" aria-hidden className={className} fill="currentColor">
      <path d="M6.19141 18.8463C6.21248 18.8424 13.1815 17.5605 16.0888 21.3252C18.9213 17.6089 25.9864 18.8463 25.9864 18.8463L25.6658 20.8958C25.6658 20.8958 17.89 19.1903 17.29 24.1252H14.89C14.29 19.2026 6.51191 20.8958 6.51191 20.8958L6.19141 18.8463ZM16.09 13.6002C16.09 13.6002 17.44 15.0252 17.44 16.5252C17.44 17.4252 16.8356 18.2502 16.09 18.2502C15.3445 18.2502 14.74 17.4252 14.74 16.5252C14.74 15.0252 16.09 13.6002 16.09 13.6002ZM7.33955 14.8147C7.33955 14.8147 9.1003 13.9469 10.5331 14.3908C11.3928 14.6572 12.0019 15.4786 11.7813 16.1908C11.5606 16.903 10.5938 17.2362 9.73408 16.9699C8.30242 16.5264 7.34109 14.8175 7.33955 14.8147ZM21.6378 14.3908C23.0698 13.9472 24.8293 14.8138 24.8313 14.8147C24.8271 14.8221 23.8665 16.527 22.4368 16.9699C21.5771 17.2363 20.6102 16.903 20.3896 16.1908C20.1689 15.4786 20.7781 14.6572 21.6378 14.3908ZM9.76397 10.6146C9.76877 10.6137 11.6922 10.2404 12.9567 11.0438C13.7164 11.5264 14.0887 12.4789 13.6889 13.1082C13.2891 13.7375 12.2685 13.8053 11.5089 13.3227C10.2428 12.5183 9.76397 10.6146 9.76397 10.6146ZM19.1688 11.0438C20.4334 10.2404 22.3568 10.6137 22.3616 10.6146C22.3616 10.6146 21.8828 12.5183 20.6167 13.3227C19.857 13.8053 18.8365 13.7375 18.4367 13.1082C18.0369 12.4789 18.4092 11.5264 19.1688 11.0438ZM16.09 7.0752C16.09 7.0752 17.44 8.5002 17.44 10.0002C17.44 10.9002 16.8356 11.7252 16.09 11.7252C15.3445 11.7252 14.74 10.9002 14.74 10.0002C14.74 8.5002 16.09 7.0752 16.09 7.0752Z" />
    </svg>
  );
}

const marks: Record<string, { mark: Mark; brand: string; own?: boolean }> = {
  linear: { mark: LinearMark, brand: "#5E6AD2", own: true },
  github: { mark: RiGithubFill, brand: "#181717" },
  jira: { mark: JiraMark, brand: "#0052CC", own: true },
  incident_io: { mark: IncidentIOMark, brand: "#F25533", own: true },
  rootly: { mark: RootlyMark, brand: "#8D6FDE", own: true },
  pagerduty: { mark: PagerDutyMark, brand: "#06AC38", own: true },
};

function markOf(provider: string) {
  return marks[provider] ?? { mark: RiPlugLine, brand: "#525252" };
}

/*
 * ToolMark is the tool's mark alone, in its colour, beside an issue.
 */
export function ToolMark({ provider, className }: { provider: string; className?: string }) {
  const { mark: Icon, brand, own } = markOf(provider);
  const branded = own === true;

  return (
    <span aria-hidden className="flex shrink-0" style={branded ? { color: brand } : undefined}>
      <Icon className={cx("size-3.5", branded ? null : "text-foreground-icon-secondary", className)} />
    </span>
  );
}

const tiles = {
  sm: { box: "size-6 rounded-md", mark: "size-3.5" },
  md: { box: "size-8 rounded-lg", mark: "size-[18px]" },
  lg: { box: "size-10 rounded-xl", mark: "size-[22px]" },
} as const;

/*
 * ToolTile is the tool's mark in white on its colour.
 */
export function ToolTile({ provider, size = "md" }: { provider: string; size?: keyof typeof tiles }) {
  const { mark: Icon, brand } = markOf(provider);

  return (
    <span
      aria-hidden
      className={cx(
        "flex shrink-0 items-center justify-center ring-1 ring-border-button-default ring-inset",
        tiles[size].box,
      )}
      style={{ backgroundColor: brand }}
    >
      <Icon className={cx(tiles[size].mark, "text-text-white")} />
    </span>
  );
}
