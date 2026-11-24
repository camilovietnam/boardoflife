<script>
  import { login, ApiError } from '../api.js';

  let { onsuccess } = $props();

  let username = $state('');
  let password = $state('');
  let pending = $state(false);
  let error = $state('');

  async function submit(event) {
    event.preventDefault();
    if (pending) return;
    error = '';
    pending = true;
    try {
      const user = await login(username.trim(), password);
      password = '';
      onsuccess(user);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        error = 'Invalid username or password';
      } else if (err instanceof ApiError && err.status === 400) {
        error = 'Please enter both a username and a password.';
      } else {
        error = 'Could not sign in right now. Please try again.';
      }
      pending = false;
    }
  }
</script>

<section class="panel">
    <p class="eyebrow">Welcome back</p>
    <h1>Board of Life</h1>
    <p class="lede">Books, exercise, learning, mindfulness: one board. Sign in to continue.</p>

    <form onsubmit={submit} novalidate>
      <div class="field">
        <label for="username">Username</label>
        <input
          id="username"
          name="username"
          type="text"
          autocomplete="username"
          autocapitalize="none"
          spellcheck="false"
          required
          bind:value={username}
          aria-invalid={error ? 'true' : undefined}
          aria-describedby={error ? 'login-error' : undefined}
        />
      </div>

      <div class="field">
        <label for="password">Password</label>
        <input
          id="password"
          name="password"
          type="password"
          autocomplete="current-password"
          required
          bind:value={password}
          aria-invalid={error ? 'true' : undefined}
          aria-describedby={error ? 'login-error' : undefined}
        />
      </div>

      {#if error}
        <p id="login-error" class="error" role="alert">
          <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10" />
            <path d="M12 7v6" />
            <path d="M12 16.5v.01" />
          </svg>
          <span>{error}</span>
        </p>
      {/if}

      <button type="submit" disabled={pending}>
        {pending ? 'Signing in…' : 'Sign in'}
      </button>
    </form>
  </section>

<style>
  /* Positioned by Board (right-hand stage); sits on the light surface #f6f5f8. */
  .panel {
    width: min(26rem, calc(100% - 3rem));
    padding: 2.5rem 0;
    margin: 0 auto;
  }

  .eyebrow {
    margin: 0 0 0.5rem;
    font-size: 0.8rem;
    font-weight: 600;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--old-rose);
  }

  h1 {
    margin: 0;
    font-family: var(--font-display);
    font-size: clamp(2.5rem, 6vw, 3.75rem);
    font-weight: 600;
    line-height: 1;
    letter-spacing: -0.02em;
    color: var(--space-indigo);
  }

  .lede {
    margin: 1rem 0 2rem;
    line-height: 1.5;
    color: #4a4a5c;
  }

  .field {
    margin-bottom: 1.25rem;
  }

  label {
    display: block;
    margin-bottom: 0.4rem;
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--space-indigo);
  }

  input {
    width: 100%;
    box-sizing: border-box;
    font: inherit;
    padding: 0.75rem 0.9rem;
    color: var(--space-indigo);
    background: #fff;
    border: 2px solid #8a8aa0;
    border-radius: 0;
    transition: border-color 150ms ease, box-shadow 150ms ease;
  }

  input:hover {
    border-color: var(--space-indigo);
  }

  input:focus-visible {
    outline: 3px solid var(--amethyst-smoke);
    outline-offset: 2px;
    border-color: var(--amethyst-smoke);
  }

  input[aria-invalid='true'] {
    border-color: #a4161a;
  }

  .error {
    display: flex;
    align-items: flex-start;
    gap: 0.5rem;
    margin: 0 0 1.25rem;
    padding: 0.7rem 0.9rem;
    font-size: 0.925rem;
    font-weight: 600;
    color: #8a1015;
    background: #fdecec;
    border-left: 4px solid #a4161a;
  }

  .error svg {
    flex: none;
    margin-top: 0.1rem;
  }

  button {
    width: 100%;
    font: inherit;
    font-weight: 600;
    letter-spacing: 0.02em;
    padding: 0.85rem 1rem;
    color: #fff;
    background: var(--dusty-lavender);
    border: 0;
    cursor: pointer;
    box-shadow: 6px 6px 0 var(--space-indigo);
    transition: transform 150ms ease, box-shadow 150ms ease, background-color 150ms ease;
  }

  button:hover:not(:disabled) {
    background: var(--amethyst-smoke);
    transform: translate(2px, 2px);
    box-shadow: 4px 4px 0 var(--space-indigo);
  }

  button:disabled {
    cursor: progress;
    opacity: 0.75;
  }

  @media (prefers-reduced-motion: reduce) {
    input,
    button {
      transition: none;
    }

    button:hover:not(:disabled) {
      transform: none;
    }
  }
</style>
