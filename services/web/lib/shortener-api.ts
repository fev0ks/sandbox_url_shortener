const defaultApiBaseUrl = "http://localhost:8080";
const defaultAnalyticsApiBaseUrl = "http://localhost:9090";

export type CreateLinkPayload = {
  original_url: string;
  title?: string;
  max_clicks?: number;
};

export type LinkResponse = {
  id: string;
  short_code: string;
  short_url: string;
  original_url: string;
  title?: string;
  created_at: string;
  updated_at: string;
  expires_at?: string;
  max_clicks: number;
  redirect_count: number;
  disabled_at?: string;
  last_accessed_at?: string;
};

export type ListLinksResponse = {
  items: LinkResponse[];
};

export type LinkStatsBucket = {
  bucket_start: string;
  clicks: number;
};

export type LinkReferrerSummary = {
  referer: string;
  clicks: number;
};

export type LinkStatsResponse = {
  total_clicks: number;
  last_visited_at?: string;
  hourly: LinkStatsBucket[];
  top_referrers: LinkReferrerSummary[];
};

type ApiErrorBody = {
  error?: {
    code?: string;
    message?: string;
    request_id?: string;
  };
};

export class ApiError extends Error {
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
    this.name = "ApiError";
    this.code = params.code;
    this.statusCode = params.statusCode;
    this.requestId = params.requestId;
  }
}

function apiBaseUrl(): string {
  return process.env.NEXT_PUBLIC_API_BASE_URL ?? defaultApiBaseUrl;
}

function analyticsApiBaseUrl(): string {
  return process.env.ANALYTICS_API_BASE_URL ?? defaultAnalyticsApiBaseUrl;
}

async function parseApiError(response: Response, fallbackMessage: string): Promise<never> {
  let body: ApiErrorBody | undefined;
  try {
    body = (await response.json()) as ApiErrorBody;
  } catch {
    body = undefined;
  }

  throw new ApiError({
    message: body?.error?.message ?? fallbackMessage,
    code: body?.error?.code ?? "unknown_error",
    statusCode: response.status,
    requestId: body?.error?.request_id,
  });
}

export async function createLink(payload: CreateLinkPayload): Promise<LinkResponse> {
  const response = await fetch(`${apiBaseUrl()}/api/v1/links`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
    cache: "no-store",
  });

  if (!response.ok) {
    await parseApiError(response, "Failed to create short link.");
  }

  return (await response.json()) as LinkResponse;
}

export async function listLinks(): Promise<ListLinksResponse> {
  const response = await fetch(`${apiBaseUrl()}/api/v1/links`, {
    method: "GET",
    cache: "no-store",
  });

  if (!response.ok) {
    await parseApiError(response, "Failed to load links.");
  }

  return (await response.json()) as ListLinksResponse;
}

export async function getLinkById(id: string): Promise<LinkResponse> {
  const response = await fetch(`${apiBaseUrl()}/api/v1/links/${id}`, {
    method: "GET",
    cache: "no-store",
  });

  if (!response.ok) {
    await parseApiError(response, "Failed to load link details.");
  }

  return (await response.json()) as LinkResponse;
}

export async function getLinkStats(id: string): Promise<LinkStatsResponse> {
  const response = await fetch(`${analyticsApiBaseUrl()}/api/v1/links/${id}/stats`, {
    method: "GET",
    cache: "no-store",
  });

  if (!response.ok) {
    await parseApiError(response, "Failed to load link stats.");
  }

  return (await response.json()) as LinkStatsResponse;
}

async function postLinkAction(id: string, action: "enable" | "disable"): Promise<LinkResponse> {
  const response = await fetch(`${apiBaseUrl()}/api/v1/links/${id}/${action}`, {
    method: "POST",
    cache: "no-store",
  });

  if (!response.ok) {
    await parseApiError(response, `Failed to ${action} link.`);
  }

  return (await response.json()) as LinkResponse;
}

export function enableLink(id: string): Promise<LinkResponse> {
  return postLinkAction(id, "enable");
}

export function disableLink(id: string): Promise<LinkResponse> {
  return postLinkAction(id, "disable");
}

export async function deleteLink(id: string): Promise<void> {
  const response = await fetch(`${apiBaseUrl()}/api/v1/links/${id}`, {
    method: "DELETE",
    cache: "no-store",
  });

  if (!response.ok) {
    await parseApiError(response, "Failed to delete link.");
  }
}
