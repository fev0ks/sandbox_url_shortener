import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  IdentityApiError,
  currentSession,
  identityApiBaseUrl,
  identityErrorCodes,
  identityHeaders,
  identityMessageForCode,
  identityProviders,
  login,
  logout,
  providerStartUrl,
} from "./identity-api";

describe("identity api client", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
    delete process.env.NEXT_PUBLIC_IDENTITY_API_BASE_URL;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("uses a single configurable identity base url", () => {
    process.env.NEXT_PUBLIC_IDENTITY_API_BASE_URL = "http://identity.test";

    expect(identityApiBaseUrl()).toBe("http://identity.test");
    expect(providerStartUrl(identityProviders.google)).toBe("http://identity.test/api/v1/auth/oauth/google/start");
  });

  it("uses one cached csrf token across state-changing requests", async () => {
    process.env.NEXT_PUBLIC_IDENTITY_API_BASE_URL = "http://identity.test";
    const fetchMock = vi.mocked(fetch);
    fetchMock
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ csrf_token: "csrf-1" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            account: {
              id: "account-1",
              email: "user@example.com",
              email_verified: false,
              providers: ["password"],
            },
            session: { expires_at: "2026-04-28T12:00:00Z" },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "ok" }), { status: 200 }));

    await login({ email: "user@example.com", password: "very-strong-password" });
    await logout();

    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "http://identity.test/api/v1/auth/login",
      expect.objectContaining({
        headers: expect.any(Headers),
      }),
    );
    expect((fetchMock.mock.calls[1]?.[1]?.headers as Headers).get(identityHeaders.csrf)).toBe("csrf-1");
    expect((fetchMock.mock.calls[2]?.[1]?.headers as Headers).get(identityHeaders.csrf)).toBe("csrf-1");
  });

  it("returns null for anonymous current session", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValue(new Response("{}", { status: 401 }));

    await expect(currentSession()).resolves.toBeNull();
  });

  it("parses identity errors", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          error: {
            code: identityErrorCodes.oauthLinkingBlocked,
            message: "provider account could not be linked safely",
            request_id: "req-1",
          },
        }),
        { status: 409, headers: { "Content-Type": "application/json" } },
      ),
    );

    await expect(currentSession()).rejects.toMatchObject({
      name: "IdentityApiError",
      code: identityErrorCodes.oauthLinkingBlocked,
      statusCode: 409,
      requestId: "req-1",
    } satisfies Partial<IdentityApiError>);
  });

  it("explains blocked OAuth linking", () => {
    expect(identityMessageForCode(identityErrorCodes.oauthLinkingBlocked, "fallback")).toContain(
      "cannot be linked safely",
    );
  });
});
