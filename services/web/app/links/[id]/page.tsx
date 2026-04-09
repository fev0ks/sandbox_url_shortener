import Link from "next/link";

import { deleteLinkAction, disableLinkAction, enableLinkAction } from "./actions";
import { getLinkStatus } from "@/lib/link-state";
import { getLinkById, ShortenerApiError } from "@/lib/shortener-api";

type LinkDetailsPageProps = {
  params: Promise<{ id: string }>;
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

function formatDate(value?: string): string {
  if (!value) {
    return "Not set";
  }

  return new Intl.DateTimeFormat("en", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

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

export default async function LinkDetailsPage({ params, searchParams }: LinkDetailsPageProps) {
  const routeParams = await params;
  const query = searchParams ? await searchParams : undefined;
  const notice = readParam(query, "notice");
  const error = readParam(query, "error");

  try {
    const link = await getLinkById(routeParams.id);
    const status = getLinkStatus(link);

    return (
      <main className="page-shell">
        <nav className="topbar">
          <div>
            <p className="brand-kicker">Sandbox URL Shortener</p>
            <strong className="brand-title">Web frontend</strong>
          </div>
          <div className="topbar-links">
            <Link className="nav-pill" href="/">
              Create link
            </Link>
            <Link className="nav-pill" href="/links">
              Links dashboard
            </Link>
            <span className="nav-pill is-active">Link details</span>
          </div>
        </nav>

        <section className="create-shell">
          <div className="create-copy">
            <p className="eyebrow">E04-004</p>
            <h1>Link details</h1>
            <p className="lede">
              Browser-facing details view over the current backend state for a single link. Actions
              call the existing shortener API and then redirect back with a clear notice.
            </p>
          </div>

          {notice ? (
            <div className="notice-banner success-banner">
              {notice === "enabled" && "Link enabled successfully."}
              {notice === "disabled" && "Link disabled successfully."}
            </div>
          ) : null}
          {error ? <div className="notice-banner error-banner">{error}</div> : null}

          <div className="details-grid">
            <section className="details-card">
              <div className="dashboard-card-head">
                <div>
                  <p className="eyebrow muted">Short code</p>
                  <h2>{link.title ?? link.short_code}</h2>
                </div>
                <div className="status-row">
                  <span className={`status-chip ${status.tone === "muted" ? "muted" : ""}`}>
                    {status.label}
                  </span>
                  <span className="status-chip">
                    {link.max_clicks === 0 ? "Unlimited" : "Capped"}
                  </span>
                </div>
              </div>

              <p className="dashboard-original">{link.original_url}</p>

              <div className="short-url-box compact">
                <code>{link.short_url}</code>
              </div>

              <dl className="dashboard-meta">
                <div>
                  <dt>Created</dt>
                  <dd>{formatDate(link.created_at)}</dd>
                </div>
                <div>
                  <dt>Updated</dt>
                  <dd>{formatDate(link.updated_at)}</dd>
                </div>
                <div>
                  <dt>Redirects</dt>
                  <dd>{link.redirect_count}</dd>
                </div>
                <div>
                  <dt>Max clicks</dt>
                  <dd>{link.max_clicks === 0 ? "Unlimited" : link.max_clicks}</dd>
                </div>
                <div>
                  <dt>Expires at</dt>
                  <dd>{formatDate(link.expires_at)}</dd>
                </div>
                <div>
                  <dt>Last accessed</dt>
                  <dd>{formatDate(link.last_accessed_at)}</dd>
                </div>
                <div>
                  <dt>Disabled at</dt>
                  <dd>{formatDate(link.disabled_at)}</dd>
                </div>
                <div>
                  <dt>Link id</dt>
                  <dd>{link.id}</dd>
                </div>
              </dl>
            </section>

            <aside className="result-card">
              <p className="eyebrow muted">Actions</p>
              <h2>Manage link lifecycle</h2>
              <p className="result-placeholder">
                These actions call the existing backend endpoints and then reload the page so the
                browser view stays aligned with the latest backend state.
              </p>

              <div className="action-stack">
                {status.canDisable ? (
                  <form action={disableLinkAction}>
                    <input type="hidden" name="id" value={link.id} />
                    <button className="primary-button" type="submit">
                      Disable link
                    </button>
                  </form>
                ) : null}

                {status.canEnable ? (
                  <form action={enableLinkAction}>
                    <input type="hidden" name="id" value={link.id} />
                    <button className="secondary-button wide-button" type="submit">
                      Enable link
                    </button>
                  </form>
                ) : null}

                <form action={deleteLinkAction}>
                  <input type="hidden" name="id" value={link.id} />
                  <button className="danger-button" type="submit">
                    Delete link
                  </button>
                </form>
              </div>
            </aside>
          </div>
        </section>
      </main>
    );
  } catch (err) {
    const message =
      err instanceof ShortenerApiError ? err.message : "Failed to load link details.";

    return (
      <main className="page-shell">
        <nav className="topbar">
          <div>
            <p className="brand-kicker">Sandbox URL Shortener</p>
            <strong className="brand-title">Web frontend</strong>
          </div>
          <div className="topbar-links">
            <Link className="nav-pill" href="/">
              Create link
            </Link>
            <Link className="nav-pill" href="/links">
              Links dashboard
            </Link>
            <span className="nav-pill is-active">Link details</span>
          </div>
        </nav>

        <section className="create-shell">
          <section className="empty-state-card">
            <p className="eyebrow muted">API error</p>
            <h2>Link unavailable</h2>
            <p className="result-placeholder">{message}</p>
            <Link className="secondary-link" href="/links">
              Back to dashboard
            </Link>
          </section>
        </section>
      </main>
    );
  }
}
