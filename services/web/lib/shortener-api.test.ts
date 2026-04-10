import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError, getLinkById, getLinkStats } from "./shortener-api";

describe("shortener api client", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
    delete process.env.NEXT_PUBLIC_API_BASE_URL;
    delete process.env.ANALYTICS_API_BASE_URL;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("uses the analytics base url for link stats", async () => {
    process.env.ANALYTICS_API_BASE_URL = "http://analytics.test";
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          total_clicks: 2,
          hourly: [],
          top_referrers: [],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await getLinkStats("019d7439-9656-762e-bc96-10b93d214352");

    expect(fetchMock).toHaveBeenCalledWith(
      "http://analytics.test/api/v1/links/019d7439-9656-762e-bc96-10b93d214352/stats",
      {
        method: "GET",
        cache: "no-store",
      },
    );
  });

  it("uses the main api base url for link details", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://shortener.test";
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          id: "019d7439-9656-762e-bc96-10b93d214352",
          short_code: "abc1234",
          short_url: "http://localhost:8080/abc1234",
          original_url: "https://example.com",
          created_at: "2026-04-10T08:00:00Z",
          updated_at: "2026-04-10T08:00:00Z",
          max_clicks: 0,
          redirect_count: 0,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await getLinkById("019d7439-9656-762e-bc96-10b93d214352");

    expect(fetchMock).toHaveBeenCalledWith(
      "http://shortener.test/api/v1/links/019d7439-9656-762e-bc96-10b93d214352",
      {
        method: "GET",
        cache: "no-store",
      },
    );
  });

  it("parses analytics api errors into ApiError", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          error: {
            code: "analytics_unavailable",
            message: "Stats backend unavailable.",
            request_id: "req-123",
          },
        }),
        { status: 503, headers: { "Content-Type": "application/json" } },
      ),
    );

    await expect(getLinkStats("019d7439-9656-762e-bc96-10b93d214352")).rejects.toMatchObject<ApiError>({
      name: "ApiError",
      code: "analytics_unavailable",
      statusCode: 503,
      requestId: "req-123",
      message: "Stats backend unavailable.",
    });
  });
});
