"use client";

import { useEffect, useState } from "react";

import { IdentityApiError, currentSession, logout, type IdentityAccount } from "@/lib/identity-api";

export function AuthNav() {
  const [account, setAccount] = useState<IdentityAccount | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    const load = () => currentSession()
      .then((session) => {
        if (active) {
          setAccount(session?.account ?? null);
        }
      })
      .catch(() => {
        if (active) {
          setAccount(null);
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false);
        }
      });
    void load();
    window.addEventListener("identity-session-changed", load);
    return () => {
      active = false;
      window.removeEventListener("identity-session-changed", load);
    };
  }, []);

  async function onLogout() {
    try {
      await logout();
      setAccount(null);
    } catch (error) {
      if (error instanceof IdentityApiError) {
        setAccount(null);
      }
    }
  }

  if (loading) {
    return <span className="nav-pill is-muted">Checking session</span>;
  }
  if (!account) {
    return <span className="nav-pill is-muted">Anonymous</span>;
  }
  return (
    <button className="nav-pill nav-button" type="button" onClick={onLogout}>
      {account.email} · Log out
    </button>
  );
}
