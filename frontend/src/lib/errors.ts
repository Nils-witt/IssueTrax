import type { TFunction } from 'i18next';
import i18n from '../i18n';
import { ApiError } from '../api/client';

/** Translates an error by its API code, falling back to its message. */
export function errorText(t: TFunction, err: unknown): string {
  if (err instanceof ApiError) {
    const key = `errors.${err.code}`;

    // Codes are dynamic, so the typed key check is bypassed on purpose.
    return i18n.exists(key) ? t(key as 'errors.internal_error') : err.message;
  }

  return t('app.unexpected');
}

/** Translates the validation code for one input field, if any. */
export function fieldErrorText(t: TFunction, err: unknown, field: string): string | undefined {
  if (!(err instanceof ApiError)) return undefined;

  const code = err.fields[field];
  if (!code) return undefined;

  const key = `fieldErrors.${code}`;

  return i18n.exists(key) ? t(key as 'fieldErrors.invalid') : code;
}
