// Thin fetch wrapper for the IssueTrax API. The session lives in an
// HttpOnly cookie, so requests only need to be same-origin.

/** An API error as returned by the backend ({"error": {code, message, fields}}). */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: Record<string, string>;

  constructor(status: number, code: string, message: string, fields: Record<string, string> = {}) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}

let redirecting = false;

/** Sends the browser to sign in, coming back to the current page afterwards. */
export function redirectToLogin(): void {
  if (redirecting) return;
  redirecting = true;

  const returnTo = window.location.pathname + window.location.search;
  window.location.assign(`/auth/login?returnTo=${encodeURIComponent(returnTo)}`);
}

interface ErrorBody {
  error?: { code?: string; message?: string; fields?: Record<string, string> };
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;

  try {
    res = await fetch(path, {
      method,
      headers: body === undefined ? { Accept: 'application/json' } : { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, 'network', 'network error');
  }

  if (res.status === 401) {
    redirectToLogin();
  }

  const data: unknown = await res.json().catch(() => null);

  if (!res.ok) {
    const err = (data as ErrorBody | null)?.error;
    throw new ApiError(res.status, err?.code ?? 'internal_error', err?.message ?? res.statusText, err?.fields);
  }

  return data as T;
}

/** Ends the session here and at the identity provider. */
export async function signOut(): Promise<void> {
  const { redirectUrl } = await request<{ redirectUrl: string }>('POST', '/auth/logout');
  window.location.assign(redirectUrl || '/');
}

/** Builds a query string from defined, non-empty values (arrays repeat the key). */
export function query(params: Record<string, string | string[] | undefined>): string {
  const q = new URLSearchParams();

  for (const [key, value] of Object.entries(params)) {
    for (const v of Array.isArray(value) ? value : [value]) {
      if (v) q.append(key, v);
    }
  }

  const s = q.toString();
  return s ? `?${s}` : '';
}
