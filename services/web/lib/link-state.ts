import type { LinkResponse } from "./shortener-api";

export type LinkStatus = {
  label: string;
  tone: "active" | "muted";
  canEnable: boolean;
  canDisable: boolean;
};

export function getLinkStatus(link: Pick<LinkResponse, "disabled_at">): LinkStatus {
  if (link.disabled_at) {
    return {
      label: "Disabled",
      tone: "muted",
      canEnable: true,
      canDisable: false,
    };
  }

  return {
    label: "Active",
    tone: "active",
    canEnable: false,
    canDisable: true,
  };
}
