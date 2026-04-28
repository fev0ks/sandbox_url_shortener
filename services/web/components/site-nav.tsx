import Link from "next/link";

import { AuthNav } from "./auth-nav";

type SiteNavProps = {
  active: "create" | "links" | "details" | "auth";
};

export function SiteNav({ active }: SiteNavProps) {
  return (
    <nav className="topbar">
      <div>
        <p className="brand-kicker">Sandbox URL Shortener</p>
        <strong className="brand-title">Web frontend</strong>
      </div>
      <div className="topbar-links">
        {active === "create" ? (
          <span className="nav-pill is-active">Create link</span>
        ) : (
          <Link className="nav-pill" href="/">
            Create link
          </Link>
        )}
        {active === "links" ? (
          <span className="nav-pill is-active">Links dashboard</span>
        ) : (
          <Link className="nav-pill" href="/links">
            Links dashboard
          </Link>
        )}
        {active === "details" ? <span className="nav-pill is-active">Link details</span> : null}
        {active === "auth" ? (
          <span className="nav-pill is-active">Account</span>
        ) : (
          <Link className="nav-pill" href="/auth">
            Account
          </Link>
        )}
        <AuthNav />
      </div>
    </nav>
  );
}
