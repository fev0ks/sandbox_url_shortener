const defaultIdentityApiBaseUrl = "http://localhost:8081";
const localHostnames = new Set(["localhost", "127.0.0.1", "::1"]);

export const identityErrorCodes = {
  invalidCSRF: "invalid_csrf",
  invalidOAuthState: "invalid_oauth_state",
  oauthLinkingBlocked: "oauth_linking_blocked",
  oauthStateExpired: "oauth_state_expired",
  oauthUnconfigured: "oauth_unconfigured",
} as const;

export const identityProviders = {
  apple: "apple",
  google: "google",
} as const;

export const identityHeaders = {
  csrf: "X-CSRF-Token",
} as const;

let csrfTokenPromise: Promise<string> | null = null;

export type IdentityProvider = (typeof identityProviders)[keyof typeof identityProviders];

export type IdentityAccount = {
  id: string;
  email: string;
  display_name?: string;
  email_verified: boolean;
  providers: string[];
};

export type IdentitySession = {
  account: IdentityAccount;
  session: {
    expires_at: string;
  };
};

type ApiErrorBody = {
  error?: {
    code?: string;
    message?: string;
    request_id?: string;
  };
};

export class IdentityApiError extends Error {
  readonly code: string;
  readonly statusCode: number;
  readonly requestId?: string;

  constructor(params: {
    message: string;
    code: string;
    statusCode: number;
    requestId?: string;
  }) {
    super(params.message);
    this.name = "IdentityApiError";
    this.code = params.code;
    this.statusCode = params.statusCode;
    this.requestId = params.requestId;
  }
}

export function identityApiBaseUrl(): string {
  const configured = process.env.NEXT_PUBLIC_IDENTITY_API_BASE_URL;
  if (configured) {
    return configured;
  }
  if (typeof window !== "undefined" && !localHostnames.has(window.location.hostname)) {
    throw new Error("NEXT_PUBLIC_IDENTITY_API_BASE_URL is required outside local development.");
  }
  return defaultIdentityApiBaseUrl;
}

export function providerStartUrl(provider: IdentityProvider): string {
  return `${identityApiBaseUrl()}/api/v1/auth/oauth/${provider}/start`;
}

export async function bootstrapCSRF(): Promise<string> {
  csrfTokenPromise ??= fetchCSRF();
  return csrfTokenPromise;
}

export function identityMessageForCode(code: string, fallback: string): string {
  if (code === identityErrorCodes.oauthLinkingBlocked) {
    return "That provider account cannot be linked safely. Use password login or another verified provider email.";
  }
  if (code === identityErrorCodes.invalidCSRF) {
    return "The session token expired. Submit the form again.";
  }
  if (code === identityErrorCodes.oauthUnconfigured) {
    return "That provider is not configured in this environment.";
  }
  if (code === identityErrorCodes.invalidOAuthState || code === identityErrorCodes.oauthStateExpired) {
    return "The provider sign-in state expired. Start again.";
  }
  return fallback;
}

export function notifyIdentitySessionChanged(): void {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event("identity-session-changed"));
  }
}

async function fetchCSRF(): Promise<string> {
  const response = await fetch(`${identityApiBaseUrl()}/api/v1/auth/csrf`, {
    method: "GET",
    credentials: "include",
    cache: "no-store",
  });

  if (!response.ok) {
    await parseIdentityError(response, "Failed to initialize auth.");
  }

  const body = (await response.json()) as { csrf_token?: string };
  return body.csrf_token ?? "";
}

export async function currentSession(): Promise<IdentitySession | null> {
  const response = await fetch(`${identityApiBaseUrl()}/api/v1/auth/session`, {
    method: "GET",
    credentials: "include",
    cache: "no-store",
  });

  if (response.status === 401) {
    return null;
  }
  if (!response.ok) {
    await parseIdentityError(response, "Failed to load current session.");
  }
  return (await response.json()) as IdentitySession;
}

export async function signup(payload: {
  email: string;
  display_name?: string;
  password: string;
}): Promise<IdentitySession> {
  return passwordRequest("/api/v1/auth/signup", payload);
}

export async function login(payload: { email: string; password: string }): Promise<IdentitySession> {
  return passwordRequest("/api/v1/auth/login", payload);
}

export async function logout(): Promise<void> {
  const response = await fetchWithCSRF(`${identityApiBaseUrl()}/api/v1/auth/logout`, {
    method: "POST",
    credentials: "include",
    cache: "no-store",
  }, "Failed to log out.");
  if (!response.ok) {
    await parseIdentityError(response, "Failed to log out.");
  }
  notifyIdentitySessionChanged();
}

async function passwordRequest(path: string, payload: unknown): Promise<IdentitySession> {
  const response = await fetchWithCSRF(`${identityApiBaseUrl()}${path}`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
    cache: "no-store",
  }, "Authentication failed.");

  if (!response.ok) {
    await parseIdentityError(response, "Authentication failed.");
  }

  const session = (await response.json()) as IdentitySession;
  notifyIdentitySessionChanged();
  return session;
}

async function fetchWithCSRF(url: string, init: RequestInit, fallbackMessage: string): Promise<Response> {
  let response = await fetchWithCurrentCSRF(url, init);
  if (response.ok) {
    return response;
  }
  const cloned = response.clone();
  const apiError = await parseIdentityErrorValue(cloned, fallbackMessage);
  if (apiError.code !== identityErrorCodes.invalidCSRF) {
    return response;
  }
  csrfTokenPromise = null;
  response = await fetchWithCurrentCSRF(url, init);
  return response;
}

async function fetchWithCurrentCSRF(url: string, init: RequestInit): Promise<Response> {
  const csrf = await bootstrapCSRF();
  const headers = new Headers(init.headers);
  headers.set(identityHeaders.csrf, csrf);
  return fetch(url, { ...init, headers });
}

async function parseIdentityError(response: Response, fallbackMessage: string): Promise<never> {
  throw await parseIdentityErrorValue(response, fallbackMessage);
}

async function parseIdentityErrorValue(response: Response, fallbackMessage: string): Promise<IdentityApiError> {
  let body: ApiErrorBody | undefined;
  try {
    body = (await response.json()) as ApiErrorBody;
  } catch {
    body = undefined;
  }

  return new IdentityApiError({
    message: body?.error?.message ?? fallbackMessage,
    code: body?.error?.code ?? "unknown_error",
    statusCode: response.status,
    requestId: body?.error?.request_id,
  });
}
