"use server";

import {
  parseCreateLinkForm,
  readCreateLinkFormData,
} from "@/lib/create-link";
import {
  initialCreateLinkState,
  type CreateLinkActionState,
} from "@/lib/create-link-action-state";
import { ApiError, createLink } from "@/lib/shortener-api";

export async function createLinkAction(
  _prevState: CreateLinkActionState,
  formData: FormData,
): Promise<CreateLinkActionState> {
  const values = readCreateLinkFormData(formData);
  const parsed = parseCreateLinkForm(values);

  if (!parsed.ok) {
    return {
      status: "error",
      values: parsed.values,
      fieldErrors: parsed.fieldErrors,
      formError: "Fix the highlighted fields and submit again.",
    };
  }

  try {
    const createdLink = await createLink(parsed.payload);
    return {
      status: "success",
      values: initialCreateLinkState.values,
      fieldErrors: {},
      createdLink,
    };
  } catch (error) {
    if (error instanceof ApiError) {
      return {
        status: "error",
        values: parsed.values,
        fieldErrors: {},
        formError: error.message,
      };
    }

    return {
      status: "error",
      values: parsed.values,
      fieldErrors: {},
      formError: "The shortener API is unavailable right now. Try again shortly.",
    };
  }
}
