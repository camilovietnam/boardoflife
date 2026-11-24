<script>
  import { onMount, tick } from 'svelte';
  import { getMe, logout, onUnauthorized } from './api.js';
  import Board from './components/Board.svelte';

  // Must match the strip <-> full-width transition handled by Board via --enter.
  const TRANSITION_MS = 1000;

  // 'loading' | 'error' | 'shell'. The shell (columns + login form + board) stays mounted
  // across sign-in and sign-out so the same elements animate between layouts.
  let view = $state('loading');
  // Within the shell: 'login' | 'entering' | 'ready' | 'leaving'
  let phase = $state('login');
  let intro = $state(false); // rise animation only when the login page first paints
  let formKey = $state(0);
  let user = $state(null);
  let signingOut = $state(false);
  let announcement = $state('');
  let board = $state();
  let timer;

  const reducedMotion = () =>
    window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const duration = () => (reducedMotion() ? 0 : TRANSITION_MS);
  const wait = (ms) => new Promise((resolve) => (timer = setTimeout(resolve, ms)));

  async function check() {
    view = 'loading';
    clearTimeout(timer);
    try {
      const me = await getMe();
      user = me;
      // Already signed in: show the board straight away, no intro.
      intro = !me;
      phase = me ? 'ready' : 'login';
      view = 'shell';
    } catch {
      view = 'error';
    }
  }

  async function handleLogin(u) {
    // Start immediately with the user returned by login; no /api/me round trip.
    user = u;
    phase = 'entering';
    announcement = '';
    await wait(duration());
    if (phase !== 'entering') return;
    phase = 'ready';
    announcement = 'Signed in. Board ready.';
    await tick(); // wait for `inert` to be lifted before moving focus
    board?.focusFirstColumn();
  }

  async function backToLogin() {
    if (phase === 'login' || phase === 'leaving') return;
    user = null;
    formKey += 1; // fresh, empty form
    phase = 'leaving';
    announcement = '';
    await wait(duration());
    if (phase !== 'leaving') return;
    phase = 'login';
    announcement = 'Signed out.';
    await tick();
    board?.focusLogin();
  }

  async function handleLogout() {
    signingOut = true;
    try {
      await logout();
    } catch {
      // Network drop or 401: confirm the real state below.
      try {
        if (await getMe()) {
          signingOut = false;
          return;
        }
      } catch {
        signingOut = false;
        view = 'error';
        return;
      }
    }
    signingOut = false;
    await backToLogin();
  }

  onMount(() => {
    // A 401 from any later call sends the user back to the login page.
    onUnauthorized(() => backToLogin());
    check();
    return () => clearTimeout(timer);
  });
</script>

<svelte:head>
  <title>Board of Life</title>
</svelte:head>

{#if view === 'loading'}
  <main class="state" aria-busy="true">
    <p class="sr-only" role="status">Loading…</p>
    <span class="pulse" aria-hidden="true"></span>
  </main>
{:else if view === 'error'}
  <main class="state">
    <div class="card" role="alert">
      <h1>Can't reach Board of Life</h1>
      <p>Something went wrong on our side or your connection dropped. Your data is safe.</p>
      <button type="button" class="retry" onclick={check}>Try again</button>
    </div>
  </main>
{:else}
  <Board bind:this={board} {phase} {intro} {formKey} duration={duration()} onlogin={handleLogin} />
  {#if phase === 'ready'}
    <button type="button" class="logout" onclick={handleLogout} disabled={signingOut}>
      <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
        <path d="M16 17l5-5-5-5" />
        <path d="M21 12H9" />
      </svg>
      <span>{signingOut ? 'Signing out…' : 'Sign out'}</span>
      <span class="sr-only">{user ? ` ${user.username}` : ''}</span>
    </button>
  {/if}
  <p class="sr-only" role="status" aria-live="polite">{announcement}</p>
{/if}

<style>
  .state {
    min-height: 100%;
    display: grid;
    place-items: center;
    background: #f6f5f8;
    padding: 1.5rem;
    box-sizing: border-box;
  }

  .pulse {
    width: 14px;
    height: 14px;
    background: var(--dusty-lavender);
    transform: rotate(45deg);
    animation: pulse 1.1s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 0.25; transform: rotate(45deg) scale(0.8); }
    50% { opacity: 1; transform: rotate(45deg) scale(1.1); }
  }

  .card {
    max-width: 26rem;
    background: #fff;
    border: 1px solid #dcd8e2;
    border-left: 6px solid #a4161a;
    padding: 1.75rem 2rem;
    box-shadow: 8px 8px 0 rgba(59, 59, 88, 0.08);
  }

  .card h1 {
    margin: 0 0 0.5rem;
    font-family: var(--font-display);
    font-size: 1.5rem;
    color: var(--space-indigo);
  }

  .card p {
    margin: 0 0 1.25rem;
    line-height: 1.5;
    color: #4a4a5c;
  }

  .retry {
    font: inherit;
    font-weight: 600;
    color: #fff;
    background: var(--dusty-lavender);
    border: 0;
    padding: 0.65rem 1.25rem;
    cursor: pointer;
  }

  .retry:hover {
    background: var(--space-indigo);
  }

  /* Floats above the board so the column layout is untouched. */
  .logout {
    position: fixed;
    right: 1rem;
    bottom: 1rem;
    z-index: 10;
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font: inherit;
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--space-indigo);
    background: #fff;
    border: 2px solid var(--space-indigo);
    border-radius: 999px;
    padding: 0.4rem 0.9rem;
    cursor: pointer;
    transition: background-color 150ms ease, color 150ms ease;
  }

  .logout:hover:not(:disabled) {
    background: var(--space-indigo);
    color: #fff;
  }

  .logout:disabled {
    cursor: progress;
    opacity: 0.8;
  }

  @media (prefers-reduced-motion: reduce) {
    .pulse { animation: none; opacity: 0.7; }
    .logout { transition: none; }
  }
</style>
