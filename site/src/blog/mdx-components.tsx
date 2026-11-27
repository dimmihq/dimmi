/*
 * Styled MDX typography and embedded product screenshot components for blog posts.
 */
import type { ComponentPropsWithoutRef } from "react";

import { ProductShot } from "../components/product-shot";

export function MdxH2(props: ComponentPropsWithoutRef<"h2">) {
  return (
    <h2
      {...props}
      className="mt-10 mb-3 text-[22px] leading-[1.25] font-medium tracking-[-0.02em] text-text-primary sm:text-[26px]"
    />
  );
}

export function MdxH3(props: ComponentPropsWithoutRef<"h3">) {
  return <h3 {...props} className="mt-7 mb-2 text-[18px] leading-[1.35] font-medium text-text-primary" />;
}

export function MdxP(props: ComponentPropsWithoutRef<"p">) {
  return <p {...props} className="my-4 text-[16px] leading-[1.7] text-text-secondary" />;
}

export function MdxUl(props: ComponentPropsWithoutRef<"ul">) {
  return (
    <ul {...props} className="my-4 flex list-disc flex-col gap-2 pl-6 text-[16px] leading-[1.65] text-text-secondary" />
  );
}

export function MdxOl(props: ComponentPropsWithoutRef<"ol">) {
  return (
    <ol
      {...props}
      className="my-4 flex list-decimal flex-col gap-2 pl-6 text-[16px] leading-[1.65] text-text-secondary"
    />
  );
}

export function MdxLi(props: ComponentPropsWithoutRef<"li">) {
  return <li {...props} className="pl-1" />;
}

export function MdxA(props: ComponentPropsWithoutRef<"a">) {
  return (
    <a
      {...props}
      className="font-medium text-accent-700 underline decoration-accent-500/40 underline-offset-4 transition-colors hover:decoration-accent-600 dark:text-accent-400"
    />
  );
}

export function MdxStrong(props: ComponentPropsWithoutRef<"strong">) {
  return <strong {...props} className="font-medium text-text-primary" />;
}

export function MdxCode(props: ComponentPropsWithoutRef<"code">) {
  return (
    <code
      {...props}
      className="rounded bg-background-secondary-default px-1.5 py-0.5 font-mono text-[14px] text-text-primary"
    />
  );
}

export function MdxPre(props: ComponentPropsWithoutRef<"pre">) {
  return (
    <pre
      {...props}
      className="my-6 overflow-x-auto rounded-xl border border-border-button-default bg-background-secondary-default/60 p-4 font-mono text-[13px] leading-6 text-text-primary"
    />
  );
}

export function MdxBlockquote(props: ComponentPropsWithoutRef<"blockquote">) {
  return (
    <blockquote
      {...props}
      className="my-6 border-l-2 border-accent-500 pl-4 text-[15px] leading-[1.65] text-text-secondary italic"
    />
  );
}

export function MdxProductShot(props: Record<string, unknown>) {
  const name = typeof props.name === "string" ? props.name : "inbox";
  const alt = typeof props.alt === "string" ? props.alt : "dimmi product screenshot";
  return (
    <figure className="my-8 flex flex-col gap-2">
      <ProductShot name={name} alt={alt} />
      <figcaption className="text-[13px] leading-5 text-text-tertiary">{alt}</figcaption>
    </figure>
  );
}
