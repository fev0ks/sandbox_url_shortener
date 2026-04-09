import { describe, expect, it } from "vitest";

import { getLinkStatus } from "./link-state";

describe("getLinkStatus", () => {
  it("marks active links as disable-able", () => {
    expect(getLinkStatus({ disabled_at: undefined })).toEqual({
      label: "Active",
      tone: "active",
      canEnable: false,
      canDisable: true,
    });
  });

  it("marks disabled links as enable-able", () => {
    expect(getLinkStatus({ disabled_at: "2026-04-09T00:00:00Z" })).toEqual({
      label: "Disabled",
      tone: "muted",
      canEnable: true,
      canDisable: false,
    });
  });
});
