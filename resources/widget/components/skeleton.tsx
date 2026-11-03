/*
 * Loading placeholders in BoardUI's skeleton style: blocks shaped like the
 * content on its way, tinted from the text color so they show in either
 * theme, with a sheen sweeping across them. A loadable region holds both
 * the placeholder and the real content; the view it sits in says which
 * one shows, and the content sharpens in from a blur once it arrives.
 * Placeholders are hidden from assistive technology; the view announces
 * that it is busy instead.
 */
import type { Child } from "widget-jsx/jsx-runtime";

type Shape = "rect" | "line" | "circle";

function Bone({ shape = "rect", className = "" }: { shape?: Shape; className?: string }) {
  return <span className={`bone ${shape} ${className}`.trim()} />;
}

export function Loadable({ skeleton, children }: { skeleton: Child; children: Child }) {
  return (
    <>
      <div className="skeleton-set" aria-hidden="true">
        {skeleton}
      </div>
      <div className="loaded">{children}</div>
    </>
  );
}

function BoneRow({ index, round }: { index: number; round: boolean }) {
  return (
    <div className="row bone-row">
      <Bone shape={round ? "circle" : "rect"} className="size-36" />
      <span className="text">
        <Bone shape="line" className={index % 2 ? "w-50" : "w-70"} />
        <Bone shape="line" className={index % 2 ? "w-90" : "w-80"} />
      </span>
    </div>
  );
}

export function SkeletonRows({ count, round = false }: { count: number; round?: boolean }) {
  return (
    <div className="list">
      {Array.from({ length: count }, (_, index) => `row-${index}`).map((key, index) => (
        <BoneRow index={index} round={round} key={key} />
      ))}
    </div>
  );
}

export function SkeletonSection({ count, round = false }: { count: number; round?: boolean }) {
  return (
    <div className="section">
      <Bone shape="line" className="w-30 label-bone" />
      <SkeletonRows count={count} round={round} />
    </div>
  );
}

export function SkeletonCards({ count }: { count: number }) {
  return (
    <>
      {Array.from({ length: count }, (_, index) => `card-${index}`).map((key) => (
        <div className="card bone-card" key={key}>
          <Bone className="art-bone" />
          <span className="copy">
            <Bone shape="line" className="w-20" />
            <Bone shape="line" className="w-70 tall" />
            <Bone shape="line" className="w-100" />
            <Bone shape="line" className="w-60" />
          </span>
        </div>
      ))}
    </>
  );
}

export function SkeletonArticle() {
  return (
    <div className="bone-page">
      <Bone shape="line" className="w-80 title" />
      <span className="byline">
        <Bone shape="circle" className="size-24" />
        <Bone shape="line" className="w-40" />
      </span>
      <Bone shape="line" className="w-100" />
      <Bone shape="line" className="w-90" />
      <Bone shape="line" className="w-60" />
      <Bone className="figure-bone" />
      <Bone shape="line" className="w-100" />
      <Bone shape="line" className="w-70" />
    </div>
  );
}

export function SkeletonThread() {
  return (
    <div className="bone-thread">
      <Bone shape="line" className="w-20 day-bone" />
      <span className="bone-said">
        <Bone shape="circle" className="size-24" />
        <Bone className="bubble-bone w-60" />
      </span>
      <Bone className="bubble-bone w-70 mine" />
      <Bone className="bubble-bone w-50 mine" />
      <span className="bone-said">
        <Bone shape="circle" className="size-24" />
        <Bone className="bubble-bone w-80 tall" />
      </span>
      <Bone className="bubble-bone w-40 mine" />
    </div>
  );
}
