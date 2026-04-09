import type { CreateLinkPayload } from "./shortener-api";

export type CreateLinkFormValues = {
  originalUrl: string;
  title: string;
  maxClicks: string;
};

export type CreateLinkFieldErrors = Partial<Record<keyof CreateLinkFormValues, string>>;

export type CreateLinkParseResult =
  | {
      ok: true;
      values: CreateLinkFormValues;
      payload: CreateLinkPayload;
    }
  | {
      ok: false;
      values: CreateLinkFormValues;
      fieldErrors: CreateLinkFieldErrors;
    };

export function readCreateLinkFormData(formData: FormData): CreateLinkFormValues {
  return {
    originalUrl: String(formData.get("originalUrl") ?? "").trim(),
    title: String(formData.get("title") ?? "").trim(),
    maxClicks: String(formData.get("maxClicks") ?? "").trim(),
  };
}

export function parseCreateLinkForm(values: CreateLinkFormValues): CreateLinkParseResult {
  const normalizedValues: CreateLinkFormValues = {
    originalUrl: values.originalUrl.trim(),
    title: values.title.trim(),
    maxClicks: values.maxClicks.trim(),
  };
  const fieldErrors: CreateLinkFieldErrors = {};

  if (normalizedValues.originalUrl === "") {
    fieldErrors.originalUrl = "Original URL is required.";
  } else {
    try {
      const parsed = new URL(normalizedValues.originalUrl);
      if (!["http:", "https:"].includes(parsed.protocol)) {
        fieldErrors.originalUrl = "Use an http or https URL.";
      }
    } catch {
      fieldErrors.originalUrl = "Enter a valid URL.";
    }
  }

  if (normalizedValues.maxClicks !== "") {
    const parsed = Number(normalizedValues.maxClicks);
    if (!Number.isInteger(parsed) || parsed < 0) {
      fieldErrors.maxClicks = "Max clicks must be a whole number greater than or equal to zero.";
    }
  }

  if (Object.keys(fieldErrors).length > 0) {
    return {
      ok: false,
      values: normalizedValues,
      fieldErrors,
    };
  }

  const payload: CreateLinkPayload = {
    original_url: normalizedValues.originalUrl,
  };

  if (normalizedValues.title !== "") {
    payload.title = normalizedValues.title;
  }

  if (normalizedValues.maxClicks !== "") {
    payload.max_clicks = Number(normalizedValues.maxClicks);
  }

  return {
    ok: true,
    values: normalizedValues,
    payload,
  };
}
