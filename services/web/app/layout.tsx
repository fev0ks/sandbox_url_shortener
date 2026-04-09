import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Shortener Web",
  description: "Browser frontend for the sandbox URL shortener.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
