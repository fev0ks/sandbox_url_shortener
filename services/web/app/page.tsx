import { CreateLinkForm } from "@/components/create-link-form";
import { SiteNav } from "@/components/site-nav";

export default function HomePage() {
  return (
    <main className="page-shell">
      <SiteNav active="create" />
      <CreateLinkForm />
    </main>
  );
}
