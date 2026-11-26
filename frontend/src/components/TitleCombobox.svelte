<script>
  import { onDestroy } from 'svelte';
  import { lookupBooks, ApiError } from '../api.js';
  import BookCover from './BookCover.svelte';

  // Title input with optional catalog suggestions (ARIA 1.2 editable combobox with list
  // autocomplete). Typing freely always works; a failed lookup never blocks anything.
  let {
    value = $bindable(''),
    id = 'bf-name',
    name = 'name',
    invalid = false,
    describedby,
    onpick,
  } = $props();

  const MIN_CHARS = 2;
  const DEBOUNCE_MS = 350;
  const LIMIT = 8;

  let results = $state([]);
  let open = $state(false);
  let active = $state(-1);
  let loading = $state(false);
  // { kind: 'info' | 'warn', text } — rendered visibly and as a polite live region.
  let note = $state({ kind: 'info', text: '' });
  let timer;
  let controller;
  let seq = 0;
  let listEl;

  const listId = $derived(`${id}-listbox`);
  const optId = (i) => `${id}-opt-${i}`;

  function cancel() {
    clearTimeout(timer);
    controller?.abort();
    controller = null;
    seq += 1; // any response still in flight is now stale
  }

  function reset() {
    cancel();
    results = [];
    open = false;
    active = -1;
    loading = false;
    note = { kind: 'info', text: '' };
  }

  onDestroy(cancel);

  function oninput(event) {
    const q = event.currentTarget.value.trim();
    cancel();
    if (q.length < MIN_CHARS) {
      reset();
      return;
    }
    loading = true;
    note = { kind: 'info', text: 'Searching…' };
    timer = setTimeout(() => search(q), DEBOUNCE_MS);
  }

  async function search(q) {
    const mine = ++seq;
    controller?.abort();
    controller = new AbortController();
    try {
      const res = await lookupBooks(q, { limit: LIMIT, signal: controller.signal });
      if (mine !== seq) return; // out-of-order response
      results = Array.isArray(res?.items) ? res.items : [];
      active = -1;
      open = results.length > 0;
      note = results.length
        ? { kind: 'info', text: `${results.length} ${results.length === 1 ? 'match' : 'matches'} found. Use the up and down arrow keys to review, Enter to pick.` }
        : { kind: 'info', text: 'No matches. You can still enter the details by hand.' };
    } catch (err) {
      if (err?.name === 'AbortError' || mine !== seq) return;
      results = [];
      open = false;
      active = -1;
      if (err instanceof ApiError && err.status === 401) {
        note = { kind: 'info', text: '' }; // the app is already returning to the sign-in page
      } else if (err instanceof ApiError && err.status === 429) {
        note = { kind: 'warn', text: 'Too many searches, try again in a moment.' };
      } else {
        note = { kind: 'warn', text: 'Book search is unavailable right now. You can still enter the details by hand.' };
      }
    } finally {
      if (mine === seq) loading = false;
    }
  }

  function pick(i) {
    const item = results[i];
    if (!item) return;
    cancel();
    loading = false;
    open = false;
    active = -1;
    results = [];
    value = item.name;
    note = { kind: 'info', text: `Picked “${item.name}” by ${item.author}. Cover and details attached.` };
    onpick?.(item);
  }

  function move(delta) {
    if (!results.length) return;
    if (!open) open = true;
    active = active < 0 ? (delta > 0 ? 0 : results.length - 1) : (active + delta + results.length) % results.length;
    listEl?.querySelector(`#${CSS.escape(optId(active))}`)?.scrollIntoView({ block: 'nearest' });
  }

  function onkeydown(event) {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      move(1);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      move(-1);
    } else if (event.key === 'Enter' && open && active >= 0) {
      event.preventDefault(); // pick, don't submit the form
      pick(active);
    } else if (event.key === 'Escape' && open) {
      event.preventDefault();
      event.stopPropagation(); // close the list only, not the dialog
      open = false;
      active = -1;
    } else if (event.key === 'Tab') {
      open = false;
    }
  }

  function onfocusout(event) {
    if (!event.currentTarget.contains(event.relatedTarget)) {
      open = false;
      active = -1;
    }
  }
</script>

<div class="combo" {onfocusout}>
  <input
    {id}
    {name}
    type="text"
    role="combobox"
    aria-autocomplete="list"
    aria-expanded={open}
    aria-controls={listId}
    aria-activedescendant={open && active >= 0 ? optId(active) : undefined}
    aria-invalid={invalid ? 'true' : undefined}
    aria-describedby={[describedby, `${id}-note`].filter(Boolean).join(' ')}
    aria-busy={loading}
    maxlength="255"
    autocomplete="off"
    spellcheck="false"
    bind:value
    {oninput}
    {onkeydown}
  />

  <ul id={listId} class="list" role="listbox" aria-label="Matching books" hidden={!open} bind:this={listEl}>
    {#each results as item, i (item.external_id ?? i)}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <li
        id={optId(i)}
        role="option"
        aria-selected={i === active}
        class:on={i === active}
        onmousedown={(e) => e.preventDefault()}
        onclick={() => pick(i)}
        onmousemove={() => (active = i)}
      >
        <BookCover name={item.name} author={item.author} src={item.cover_url} size="thumb" />
        <span class="txt">
          <span class="t">{item.name}</span>
          <span class="m">{item.author}{item.published_year ? ` · ${item.published_year}` : ''}</span>
        </span>
      </li>
    {/each}
  </ul>

  <p id="{id}-note" class="note" class:warn={note.kind === 'warn'} class:idle={!note.text} role="status" aria-live="polite">
    {#if note.text}
      {#if loading}
        <span class="dot" aria-hidden="true"></span>
      {:else if note.kind === 'warn'}
        <svg viewBox="0 0 24 24" width="15" height="15" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3l10 18H2z" /><path d="M12 10v5" /><path d="M12 18v.01" /></svg>
      {:else}
        <svg viewBox="0 0 24 24" width="15" height="15" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M12 11v6" /><path d="M12 7.5v.01" /></svg>
      {/if}
      <span>{note.text}</span>
    {/if}
  </p>
</div>

<style>
  .combo { position: relative; }
  input {
    width: 100%; box-sizing: border-box; font: inherit; padding: 0.65rem 0.8rem; color: var(--space-indigo);
    background: #fff; border: 2px solid #8a8aa0; border-radius: 0;
  }
  input:hover { border-color: var(--space-indigo); }
  input:focus-visible { outline: 3px solid var(--amethyst-smoke); outline-offset: 2px; }
  input[aria-invalid='true'] { border-color: #a4161a; }

  .list {
    position: absolute; z-index: 5; left: 0; right: 0; margin: 0; padding: 0; list-style: none;
    max-height: 18rem; overflow-y: auto; background: #fff; border: 2px solid var(--space-indigo);
    box-shadow: 6px 6px 0 rgba(59, 59, 88, 0.25);
  }
  .list[hidden] { display: none; }
  li { display: flex; align-items: center; gap: 0.8rem; padding: 0.55rem 0.8rem; cursor: pointer; border-bottom: 1px dashed #cfcbd9; border-left: 4px solid transparent; }
  li:last-child { border-bottom: 0; }
  /* The active option is marked by a bar and an underline as well as its tint. */
  li.on { background: #eeecf3; border-left-color: var(--amethyst-smoke); }
  li.on .t { text-decoration: underline; text-underline-offset: 3px; }
  .txt { min-width: 0; display: flex; flex-direction: column; gap: 0.1rem; }
  .t { font-family: var(--font-display); font-weight: 600; line-height: 1.2; overflow-wrap: anywhere; display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .m { font-size: 0.8rem; color: #4a4a5c; font-style: italic; }

  .note { display: flex; align-items: flex-start; gap: 0.45rem; margin: 0.4rem 0 0; min-height: 1.3rem; font-size: 0.8rem; line-height: 1.35; color: #4a4a5c; }
  .note.idle { min-height: 0; margin: 0; }
  .note svg { flex: none; margin-top: 0.1rem; }
  .note.warn { padding: 0.4rem 0.7rem; font-weight: 600; color: var(--space-indigo); background: #f4f0f5; border-left: 4px solid var(--rosy-taupe); }
  .dot { flex: none; width: 9px; height: 9px; margin: 0.28rem 0.2rem 0 0.15rem; background: var(--dusty-lavender); transform: rotate(45deg); animation: pulse 1.1s ease-in-out infinite; }
  @keyframes pulse { 0%, 100% { opacity: 0.25; transform: rotate(45deg) scale(0.8); } 50% { opacity: 1; transform: rotate(45deg) scale(1.1); } }

  @media (prefers-reduced-motion: reduce) { .dot { animation: none; opacity: 0.8; } }
</style>
