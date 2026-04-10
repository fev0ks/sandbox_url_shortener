import Link from "next/link";

import { ApiError, listLinks } from "@/lib/shortener-api";

type LinksPageProps = {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

function formatDate(value: string): string {
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

export default async function LinksPage({ searchParams }: LinksPageProps) {
  const query = searchParams ? await searchParams : undefined;
  const notice = readParam(query, "notice");
  const error = readParam(query, "error");

  try {
    const response = await listLinks();

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
            <span className="nav-pill is-active">Links dashboard</span>
          </div>
        </nav>

        <section className="create-shell">
          <div className="create-copy">
            <p className="eyebrow">E04-003</p>
            <h1>Recent links</h1>
            <p className="lede">
              Dashboard view over the latest non-deleted links returned by the shortener API. This
              stays intentionally narrow: recent-first ordering, no filters, no pagination yet.
            </p>
          </div>

          {notice === "deleted" ? (
            <div className="notice-banner success-banner">Link deleted successfully.</div>
          ) : null}
          {error === "missing-link-id" ? (
            <div className="notice-banner error-banner">Missing link id.</div>
          ) : null}

          {response.items.length == 0 ? (
            <section className="empty-state-card">
              <p className="eyebrow muted">Empty state</p>
              <h2>No links yet</h2>
              <p className="result-placeholder">
                Create your first short link from the main page. Once links exist, this dashboard
                will show them newest first.
              </p>
              <Link className="secondary-link" href="/">
                Go to create page
              </Link>
            </section>
          ) : (
            <section className="dashboard-list">
              {response.items.map((item) => (
                <article className="dashboard-card" key={item.id}>
                  <div className="dashboard-card-head">
                    <div>
                      <p className="eyebrow muted">Short code</p>
                      <h2>{item.title ?? item.short_code}</h2>
                    </div>
                    <div className="status-row">
                      <span className="status-chip">{item.max_clicks === 0 ? "Unlimited" : "Capped"}</span>
                      {item.disabled_at ? <span className="status-chip muted">Disabled</span> : null}
                    </div>
                  </div>

                  <p className="dashboard-original">{item.original_url}</p>

                  <div className="short-url-box compact">
                    <code>{item.short_url}</code>
                  </div>
                  <div className="link-action-row">
                    <a
                      className="secondary-link"
                      href={item.short_url}
                      target="_blank"
                      rel="noreferrer"
                    >
                      Open short link
                    </a>
                    <a
                      className="secondary-link"
                      href={item.original_url}
                      target="_blank"
                      rel="noreferrer"
                    >
                      Open original URL
                    </a>
                  </div>

                  <dl className="dashboard-meta">
                    <div>
                      <dt>Created</dt>
                      <dd>{formatDate(item.created_at)}</dd>
                    </div>
                    <div>
                      <dt>Redirects</dt>
                      <dd>{item.redirect_count}</dd>
                    </div>
                    <div>
                      <dt>Max clicks</dt>
                      <dd>{item.max_clicks === 0 ? "Unlimited" : item.max_clicks}</dd>
                    </div>
                    <div>
                      <dt>Link id</dt>
                      <dd>{item.id}</dd>
                    </div>
                  </dl>

                  <div className="dashboard-actions">
                    <Link className="secondary-link" href={`/links/${item.id}`}>
                      Open details
                    </Link>
                  </div>
                </article>
              ))}
            </section>
          )}
        </section>
      </main>
    );
  } catch (error) {
    const message =
      error instanceof ApiError
        ? error.message
        : "The links dashboard is temporarily unavailable. Try again shortly.";

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
            <span className="nav-pill is-active">Links dashboard</span>
          </div>
        </nav>

        <section className="create-shell">
          {notice === "deleted" ? (
            <div className="notice-banner success-banner">Link deleted successfully.</div>
          ) : null}
          {error === "missing-link-id" ? (
            <div className="notice-banner error-banner">Missing link id.</div>
          ) : null}
          <section className="empty-state-card">
            <p className="eyebrow muted">API error</p>
            <h2>Dashboard unavailable</h2>
            <p className="result-placeholder">{message}</p>
            <Link className="secondary-link" href="/">
              Go to create page
            </Link>
          </section>
        </section>
      </main>
    );
  }
}
