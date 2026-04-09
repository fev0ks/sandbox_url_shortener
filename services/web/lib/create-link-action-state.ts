import type { CreateLinkFieldErrors, CreateLinkFormValues } from "./create-link";
import type { LinkResponse } from "./shortener-api";

export type CreateLinkActionState = {
  status: "idle" | "success" | "error";
  values: CreateLinkFormValues;
  fieldErrors: CreateLinkFieldErrors;
  formError?: string;
  createdLink?: LinkResponse;
};

export const initialCreateLinkState: CreateLinkActionState = {
  status: "idle",
  values: {
    originalUrl: "",
    title: "",
    maxClicks: "",
  },
  fieldErrors: {},
};
