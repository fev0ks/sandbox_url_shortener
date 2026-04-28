import Link from "next/link";

import { SiteNav } from "@/components/site-nav";
import { identityMessageForCode } from "@/lib/identity-api";

type AuthCallbackPageProps = {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

function readParam(
  params: Record<string, string | string[] | undefined> | undefined,
  key: string,
): string | undefined {
  const value = params?.[key];
  if (Array.isArray(value)) {
    return value[0];
  }
  return value;
}

export default async function AuthCallbackPage({ searchParams }: AuthCallbackPageProps) {
  const query = searchParams ? await searchParams : undefined;
  const status = readParam(query, "status");
  const error = readParam(query, "error");
  const requestID = readParam(query, "request_id");
  const success = status === "success";

  return (
    <main className="page-shell">
      <SiteNav active="auth" />
      <section className="create-shell">
        <section className="empty-state-card">
          <p className="eyebrow muted">Provider sign-in</p>
          <h2>{success ? "Signed in" : "Sign-in failed"}</h2>
          <p className="result-placeholder">
            {success
              ? "Your identity session is ready."
              : identityMessageForCode(
                  error ?? "oauth_provider_invalid",
                  "The provider did not return a usable sign-in response.",
                )}
          </p>
          {!success && error ? (
            <p className="auth-diagnostic">
              Error code: <code>{error}</code>
            </p>
          ) : null}
          {!success && requestID ? (
            <p className="auth-diagnostic">
              Request ID: <code>{requestID}</code>
            </p>
          ) : null}
          <Link className="secondary-link" href={success ? "/" : "/auth"}>
            {success ? "Go to create page" : "Return to account access"}
          </Link>
        </section>
      </section>
    </main>
  );
}
