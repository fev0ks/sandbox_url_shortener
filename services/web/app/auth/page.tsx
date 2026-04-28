import { AuthPanel } from "@/components/auth-panel";
import { SiteNav } from "@/components/site-nav";

export default function AuthPage() {
  return (
    <main className="page-shell">
      <SiteNav active="auth" />
      <section className="create-shell">
        <div className="create-copy">
          <p className="eyebrow">E14</p>
          <h1>Account access</h1>
          <p className="lede">
            Password and provider sign-in for the identity service. Sessions are issued by the API
            and loaded through the current-session contract.
          </p>
        </div>
        <AuthPanel />
      </section>
    </main>
  );
}
