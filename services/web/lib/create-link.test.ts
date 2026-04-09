import { describe, expect, it } from "vitest";

import { parseCreateLinkForm } from "./create-link";

describe("parseCreateLinkForm", () => {
  it("accepts a valid payload and trims optional fields", () => {
    const result = parseCreateLinkForm({
      originalUrl: "https://example.com/article",
      title: "  Example  ",
      maxClicks: "12",
    });

    expect(result.ok).toBe(true);
    if (!result.ok) {
      return;
    }

    expect(result.payload).toEqual({
      original_url: "https://example.com/article",
      title: "Example",
      max_clicks: 12,
    });
  });

  it("rejects missing or malformed urls", () => {
    expect(
      parseCreateLinkForm({
        originalUrl: "",
        title: "",
        maxClicks: "",
      }),
    ).toMatchObject({
      ok: false,
      fieldErrors: {
        originalUrl: "Original URL is required.",
      },
    });

    expect(
      parseCreateLinkForm({
        originalUrl: "not-a-url",
        title: "",
        maxClicks: "",
      }),
    ).toMatchObject({
      ok: false,
      fieldErrors: {
        originalUrl: "Enter a valid URL.",
      },
    });
  });

  it("rejects negative or non-integer max clicks", () => {
    expect(
      parseCreateLinkForm({
        originalUrl: "https://example.com",
        title: "",
        maxClicks: "-1",
      }),
    ).toMatchObject({
      ok: false,
      fieldErrors: {
        maxClicks: "Max clicks must be a whole number greater than or equal to zero.",
      },
    });

    expect(
      parseCreateLinkForm({
        originalUrl: "https://example.com",
        title: "",
        maxClicks: "1.5",
      }),
    ).toMatchObject({
      ok: false,
      fieldErrors: {
        maxClicks: "Max clicks must be a whole number greater than or equal to zero.",
      },
    });
  });
});
