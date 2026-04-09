import Link from "next/link";

import { CreateLinkForm } from "@/components/create-link-form";

export default function HomePage() {
  return (
    <main className="page-shell">
      <nav className="topbar">
        <div>
          <p className="brand-kicker">Sandbox URL Shortener</p>
          <strong className="brand-title">Web frontend</strong>
        </div>
        <div className="topbar-links">
          <span className="nav-pill is-active">Create link</span>
          <Link className="nav-pill" href="/links">
            Links dashboard
          </Link>
        </div>
      </nav>
      <CreateLinkForm />
    </main>
  );
}
