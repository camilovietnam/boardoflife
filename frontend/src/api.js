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

export function logout() {
  return request('/api/auth/logout', { method: 'POST', notifyUnauthorized: false });
}
