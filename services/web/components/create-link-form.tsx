"use client";

import { useActionState, useState } from "react";

import { initialCreateLinkState } from "@/lib/create-link-action-state";
import { createLinkAction } from "@/app/actions";

export function CreateLinkForm() {
  const [state, formAction, isPending] = useActionState(createLinkAction, initialCreateLinkState);
  const currentState = state ?? initialCreateLinkState;
  const [copied, setCopied] = useState(false);

  async function handleCopy(shortUrl: string) {
    try {
      await navigator.clipboard.writeText(shortUrl);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  }

  return (
    <section className="create-shell">
      <div className="create-copy">
        <p className="eyebrow">E04-002</p>
        <h1>Create a short link</h1>
        <p className="lede">
          This page submits through a Next.js server action that calls the shortener API on the
          server side. That keeps the frontend API client typed and avoids introducing CORS work
          into this task.
        </p>
      </div>

      <div className="create-grid">
        <form action={formAction} className="form-card">
          <label className="field">
            <span>Original URL</span>
            <input
              name="originalUrl"
              type="url"
              placeholder="https://example.com/article"
              required
              defaultValue={currentState.values.originalUrl}
            />
            {currentState.fieldErrors.originalUrl ? (
              <span className="field-error">{currentState.fieldErrors.originalUrl}</span>
            ) : null}
          </label>

          <label className="field">
            <span>Title</span>
            <input
              name="title"
              type="text"
              placeholder="Optional internal label"
              defaultValue={currentState.values.title}
            />
          </label>

          <label className="field">
            <span>Max clicks</span>
            <input
              name="maxClicks"
              type="number"
              inputMode="numeric"
              min="0"
              step="1"
              placeholder="Unlimited by default"
              defaultValue={currentState.values.maxClicks}
            />
            {currentState.fieldErrors.maxClicks ? (
              <span className="field-error">{currentState.fieldErrors.maxClicks}</span>
            ) : null}
          </label>

          {currentState.formError ? <p className="form-error">{currentState.formError}</p> : null}

          <button className="primary-button" type="submit" disabled={isPending}>
            {isPending ? "Creating..." : "Create short link"}
          </button>
        </form>

        <aside className="result-card">
          {currentState.createdLink ? (
            <>
              <p className="eyebrow success">Created</p>
              <h2>{currentState.createdLink.title ?? "Untitled link"}</h2>
              <p className="result-original">{currentState.createdLink.original_url}</p>
              <div className="short-url-box">
                <code>{currentState.createdLink.short_url}</code>
                <button
                  className="secondary-button"
                  type="button"
                  onClick={() => handleCopy(currentState.createdLink?.short_url ?? "")}
                >
                  {copied ? "Copied" : "Copy URL"}
                </button>
              </div>
              <dl className="result-meta">
                <div>
                  <dt>Short code</dt>
                  <dd>{currentState.createdLink.short_code}</dd>
                </div>
                <div>
                  <dt>Max clicks</dt>
                  <dd>
                    {currentState.createdLink.max_clicks === 0
                      ? "Unlimited"
                      : currentState.createdLink.max_clicks}
                  </dd>
                </div>
                <div>
                  <dt>Redirect count</dt>
                  <dd>{currentState.createdLink.redirect_count}</dd>
                </div>
                <div>
                  <dt>Link id</dt>
                  <dd>{currentState.createdLink.id}</dd>
                </div>
              </dl>
            </>
          ) : (
            <>
              <p className="eyebrow muted">Result</p>
              <h2>Created link details appear here</h2>
              <p className="result-placeholder">
                Submit a valid URL and the page will show the created short URL, key metadata, and
                a copy action without exposing raw API responses to the user.
              </p>
            </>
          )}
        </aside>
      </div>
    </section>
  );
}
