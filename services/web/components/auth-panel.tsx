"use client";

import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import {
  IdentityApiError,
  currentSession,
  identityMessageForCode,
  identityProviders,
  login,
  providerStartUrl,
  signup,
  type IdentitySession,
} from "@/lib/identity-api";

type Mode = "login" | "signup";

export function AuthPanel() {
  const router = useRouter();
  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [session, setSession] = useState<IdentitySession | null>(null);

  useEffect(() => {
    let active = true;
    const syncSession = () => {
      void currentSession()
        .then((nextSession) => {
          if (active) {
            setSession(nextSession);
          }
        })
        .catch(() => {
          if (active) {
            setSession(null);
          }
        });
    };
    syncSession();
    window.addEventListener("identity-session-changed", syncSession);
    return () => {
      active = false;
      window.removeEventListener("identity-session-changed", syncSession);
    };
  }, []);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const nextSession =
        mode === "signup"
          ? await signup({ email, display_name: displayName || undefined, password })
          : await login({ email, password });
      setSession(nextSession);
      router.refresh();
    } catch (err) {
      if (err instanceof IdentityApiError) {
        setError(identityMessageForCode(err.code, err.message));
      } else {
        setError("Identity service is unavailable.");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="auth-grid">
      <form className="form-card" onSubmit={onSubmit}>
        <div className="segmented-control" aria-label="Authentication mode">
          <button
            className={mode === "login" ? "is-selected" : ""}
            type="button"
            onClick={() => setMode("login")}
          >
            Log in
          </button>
          <button
            className={mode === "signup" ? "is-selected" : ""}
            type="button"
            onClick={() => setMode("signup")}
          >
            Sign up
          </button>
        </div>

        <label className="field">
          <span>Email</span>
          <input
            type="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            autoComplete="email"
            required
          />
        </label>

        {mode === "signup" ? (
          <label className="field">
            <span>Display name</span>
            <input
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              autoComplete="name"
            />
          </label>
        ) : null}

        <label className="field">
          <span>Password</span>
          <input
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete={mode === "signup" ? "new-password" : "current-password"}
            minLength={12}
            maxLength={72}
            required
          />
        </label>

        {error ? <p className="form-error">{error}</p> : null}

        <button className="primary-button" type="submit" disabled={busy}>
          {busy ? "Working..." : mode === "signup" ? "Create account" : "Log in"}
        </button>

        <div className="provider-row">
          <a className="secondary-link provider-link" href={providerStartUrl(identityProviders.google)}>
            Continue with Google
          </a>
          <a className="secondary-link provider-link" href={providerStartUrl(identityProviders.apple)}>
            Continue with Apple
          </a>
        </div>
      </form>

      <aside className="result-card">
        <p className="eyebrow muted">Session</p>
        <h2>{session ? "Signed in" : "No active account"}</h2>
        <p className="result-placeholder">
          {session
            ? `${session.account.email} is authenticated until ${new Intl.DateTimeFormat("en", {
                dateStyle: "medium",
                timeStyle: "short",
              }).format(new Date(session.session.expires_at))}.`
            : "Use password login or a configured provider to create an identity session."}
        </p>
      </aside>
    </section>
  );
}
