// Small API client. All paths are relative so the browser sends same-origin cookies.

let unauthorizedHandler = null;

/** Register a callback fired when a call (other than login/me) returns 401. */
export function onUnauthorized(fn) {
  unauthorizedHandler = fn;
}

export class ApiError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

async function errorMessage(res) {
  try {
    const body = await res.json();
    if (body && typeof body.error === 'string') return body.error;
  } catch {
    // body was empty or not JSON
  }
  return '';
}

async function request(path, { method = 'GET', body, notifyUnauthorized = true } = {}) {
  let res;
  try {
    res = await fetch(path, {
      method,
      credentials: 'same-origin',
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, 'Network error');
  }

  if (res.status === 401 && notifyUnauthorized && unauthorizedHandler) {
    unauthorizedHandler();
  }
  if (!res.ok) throw new ApiError(res.status, await errorMessage(res));
  if (res.status === 204) return null;
  return res.json();
}

/** Resolves to the user, or null when not signed in (401). Throws ApiError otherwise. */
export async function getMe() {
  try {
    return await request('/api/me', { notifyUnauthorized: false });
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) return null;
    throw err;
  }
}

export function login(username, password) {
  return request('/api/auth/login', {
    method: 'POST',
    body: { username, password },
    notifyUnauthorized: false,
  });
}

async function requestWithFields(path, opts) {
  let res;
  try {
    res = await fetch(path, {
      method: opts.method,
      credentials: 'same-origin',
      headers: opts.body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
      signal: opts.signal,
    });
  } catch (err) {
    // A cancelled request is not an error the UI should report: let callers ignore it.
    if (err?.name === 'AbortError') throw err;
    throw new ApiError(0, 'Network error');
  }
  if (res.status === 401 && unauthorizedHandler) unauthorizedHandler();
  if (res.ok) return res.status === 204 ? null : res.json();
  let message = '';
  let fields = {};
  try {
    const body = await res.json();
    if (typeof body?.error === 'string') message = body.error;
    if (body?.fields && typeof body.fields === 'object') fields = body.fields;
  } catch {
    // empty or non-JSON body
  }
  const err = new ApiError(res.status, message);
  err.fields = fields;
  err.retryAfter = res.headers.get('Retry-After');
  throw err;
}

/**
 * Book metadata search. q is 2..200 chars, limit defaults to 8 (max 20).
 * Pass an AbortSignal to cancel a stale request; an aborted call rejects with
 * an AbortError (check `err.name === 'AbortError'`), not an ApiError.
 * -> { items: [{ external_id, name, author, description, cover_url, published_year }] }
 */
export function lookupBooks(q, { limit = 8, signal } = {}) {
  const params = new URLSearchParams({ q, limit: String(limit) });
  return requestWithFields(`/api/books/lookup?${params}`, { method: 'GET', signal });
}

/** { status, q, page, pageSize } -> { items, page, page_size, total, total_pages, counts } */
export function listBooks({ status, q, page = 1, pageSize = 12 } = {}) {
  const params = new URLSearchParams();
  if (status) params.set('status', status);
  if (q) params.set('q', q);
  params.set('page', String(page));
  params.set('page_size', String(pageSize));
  return requestWithFields(`/api/books?${params}`, { method: 'GET' });
}

export function createBook(book) {
  return requestWithFields('/api/books', { method: 'POST', body: book });
}

export function updateBook(id, book) {
  return requestWithFields(`/api/books/${encodeURIComponent(id)}`, { method: 'PUT', body: book });
}

export function deleteBook(id) {
  return requestWithFields(`/api/books/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export function logout() {
  return request('/api/auth/logout', { method: 'POST', notifyUnauthorized: false });
}
