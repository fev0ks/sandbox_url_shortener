"use server";

import { redirect } from "next/navigation";

import { ApiError, deleteLink, disableLink, enableLink } from "@/lib/shortener-api";

function detailRoute(id: string, params?: Record<string, string>): string {
  if (!params || Object.keys(params).length === 0) {
    return `/links/${id}`;
  }

  const search = new URLSearchParams(params);
  return `/links/${id}?${search.toString()}`;
}

function dashboardRoute(params?: Record<string, string>): string {
  if (!params || Object.keys(params).length === 0) {
    return "/links";
  }

  const search = new URLSearchParams(params);
  return `/links?${search.toString()}`;
}

export async function disableLinkAction(formData: FormData): Promise<void> {
  const id = String(formData.get("id") ?? "");
  if (!id) {
    redirect(dashboardRoute({ error: "missing-link-id" }));
  }

  let target: string;
  try {
    await disableLink(id);
    target = detailRoute(id, { notice: "disabled" });
  } catch (error) {
    const message = error instanceof ApiError ? error.message : "Disable failed.";
    target = detailRoute(id, { error: message });
  }

  redirect(target);
}

export async function enableLinkAction(formData: FormData): Promise<void> {
  const id = String(formData.get("id") ?? "");
  if (!id) {
    redirect(dashboardRoute({ error: "missing-link-id" }));
  }

  let target: string;
  try {
    await enableLink(id);
    target = detailRoute(id, { notice: "enabled" });
  } catch (error) {
    const message = error instanceof ApiError ? error.message : "Enable failed.";
    target = detailRoute(id, { error: message });
  }

  redirect(target);
}

export async function deleteLinkAction(formData: FormData): Promise<void> {
  const id = String(formData.get("id") ?? "");
  if (!id) {
    redirect(dashboardRoute({ error: "missing-link-id" }));
  }

  let target: string;
  try {
    await deleteLink(id);
    target = dashboardRoute({ notice: "deleted" });
  } catch (error) {
    const message = error instanceof ApiError ? error.message : "Delete failed.";
    target = detailRoute(id, { error: message });
  }

  redirect(target);
}
