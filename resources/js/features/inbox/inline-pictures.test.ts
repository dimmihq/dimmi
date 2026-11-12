import { describe, expect, it } from "vitest";

import type { Attachment } from "@/lib/api/inbox";

import { inlinePictures, pictureFor } from "./inline-pictures";

function file(change: Partial<Attachment>): Attachment {
  return {
    id: "att_1",
    filename: "shot.png",
    content_type: "image/png",
    size_bytes: 10,
    width: null,
    height: null,
    url: "/files/shot.png",
    blocked: false,
    content_id: null,
    ...change,
  };
}

describe("inline pictures", () => {
  it("draws a picture the text names in place and lists the rest", () => {
    const shot = file({ id: "att_1", content_id: "shot@mail" });
    const receipt = file({ id: "att_2", filename: "receipt.pdf", content_type: "application/pdf" });

    const { pictures, listed } = inlinePictures("See ![shot](cid:shot@mail)", [shot, receipt]);

    expect(pictures).toEqual({ "shot@mail": "/files/shot.png" });
    expect(listed).toEqual([receipt]);
  });

  it("keeps listing a picture the text never names, or one not readable yet", () => {
    const unnamed = file({ id: "att_1", content_id: "other@mail" });
    const waiting = file({ id: "att_2", content_id: "shot@mail", url: null });
    const blocked = file({ id: "att_3", content_id: "bad@mail", blocked: true });

    const { pictures, listed } = inlinePictures("![a](cid:shot@mail) ![b](cid:bad@mail)", [unnamed, waiting, blocked]);

    expect(pictures).toEqual({});
    expect(listed).toEqual([unnamed, waiting, blocked]);
  });

  it("resolves only cid links, and drops one naming no picture", () => {
    const pictures = { "shot@mail": "/files/shot.png" };

    expect(pictureFor("cid:shot@mail", pictures)).toBe("/files/shot.png");
    expect(pictureFor("cid:gone@mail", pictures)).toBe("");
    expect(pictureFor("https://example.com/a.png", pictures)).toBeNull();
  });
});
