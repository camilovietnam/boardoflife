<script>
  import { onMount } from 'svelte';
  import { createBook, updateBook, ApiError } from '../api.js';
  import StarRating from './StarRating.svelte';
  import TitleCombobox from './TitleCombobox.svelte';
  import BookCover from './BookCover.svelte';

  // Native <dialog>: showModal() traps focus, makes the page inert and handles Escape.
  let { book = null, onsaved, onclose } = $props();

  const STATUSES = [
    { id: 'to_read', label: 'To read' },
    { id: 'reading', label: 'Reading' },
    { id: 'finished', label: 'Finished' },
  ];

  let dialogEl;
  // Snapshot once: the form edits a copy, never the live prop.
  const initial = (() => book)();
  let name = $state(initial?.name ?? '');
  let author = $state(initial?.author ?? '');
  let status = $state(initial?.status ?? 'to_read');
  let progress = $state(initial?.progress ?? 0);
  let rating = $state(initial?.rating ?? null);
  // Catalog details. PUT is a full replace, so these are always sent back (null = none).
  let coverUrl = $state(initial?.cover_url ?? null);
  let description = $state(initial?.description ?? null);
  let externalId = $state(initial?.external_id ?? null);
  let detailsNote = $state('');
  const hasDetails = $derived(Boolean(coverUrl || description || externalId));
  let pending = $state(false);
  let errors = $state({});
  let formError = $state('');

  const locked = $derived(status !== 'reading');

  // Mirror the server: to_read forces 0, finished forces 100.
  $effect(() => {
    if (status === 'to_read') progress = 0;
    else if (status === 'finished') progress = 100;
  });

  onMount(() => {
    dialogEl.showModal();
  });

  function setProgress(v) {
    const n = Math.round(Number(v));
    progress = Number.isFinite(n) ? Math.min(100, Math.max(0, n)) : 0;
  }

  function pickResult(item) {
    author = item.author ?? author;
    coverUrl = item.cover_url ?? null;
    description = item.description ?? null;
    externalId = item.external_id ?? null;
    detailsNote = '';
  }

  function detach() {
    coverUrl = null;
    description = null;
    externalId = null;
    detailsNote = 'Cover and details removed.';
    dialogEl.querySelector('#bf-name')?.focus();
  }

  async function submit(event) {
    event.preventDefault();
    if (pending) return;
    formError = '';
    errors = {};
    const payload = {
      name: name.trim(),
      author: author.trim(),
      status,
      progress,
      rating,
      cover_url: coverUrl,
      description,
      external_id: externalId,
    };
    const local = {};
    if (!payload.name) local.name = 'Enter the book title.';
    if (!payload.author) local.author = 'Enter the author.';
    if (Object.keys(local).length) {
      errors = local;
      dialogEl.querySelector(`[name="${Object.keys(local)[0]}"]`)?.focus();
      return;
    }
    pending = true;
    try {
      const saved = book ? await updateBook(book.id, payload) : await createBook(payload);
      onsaved(saved);
      dialogEl.close();
    } catch (err) {
      pending = false;
      if (err instanceof ApiError && err.status === 400) {
        errors = err.fields ?? {};
        if (!Object.keys(errors).length) formError = err.message || 'Please check the form.';
        const first = Object.keys(errors)[0];
        if (first) dialogEl.querySelector(`[name="${first}"]`)?.focus();
      } else if (err instanceof ApiError && err.status === 404) {
        formError = 'This book no longer exists. Close this dialog and refresh the list.';
      } else if (!(err instanceof ApiError && err.status === 401)) {
        formError = 'Could not save right now. Please try again.';
      }
    }
  }
</script>

{#snippet fieldError(id, msg)}
  {#if msg}
    <p id={id} class="error" role="alert">
      <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M12 7v6" /><path d="M12 16.5v.01" /></svg>
      <span>{msg}</span>
    </p>
  {/if}
{/snippet}

<dialog bind:this={dialogEl} aria-labelledby="book-form-title" {onclose}>
  <form onsubmit={submit} novalidate>
    <header>
      <p class="eyebrow">{book ? 'Edit entry' : 'New entry'}</p>
      <h2 id="book-form-title">{book ? 'Edit book' : 'Add a book'}</h2>
    </header>

    <div class="field">
      <label for="bf-name">Title</label>
      <TitleCombobox bind:value={name} invalid={Boolean(errors.name)} describedby={errors.name ? 'bf-name-err' : undefined} onpick={pickResult} />
      {@render fieldError('bf-name-err', errors.name)}
    </div>

    <div class="field">
      <label for="bf-author">Author</label>
      <input id="bf-author" name="author" type="text" bind:value={author} maxlength="255" autocomplete="off"
        aria-invalid={errors.author ? 'true' : undefined} aria-describedby={errors.author ? 'bf-author-err' : undefined} />
      {@render fieldError('bf-author-err', errors.author)}
    </div>

    {#if hasDetails}
      <section class="details field" aria-labelledby="bf-details-title">
        <BookCover {name} {author} src={coverUrl} size="preview" alt={`Cover of ${name || 'this book'}`} />
        <div class="dbody">
          <h3 id="bf-details-title">From book search</h3>
          {#if description}
            <p class="snippet">{description}</p>
          {:else}
            <p class="snippet none">No description came with this match.</p>
          {/if}
          <button type="button" class="link" onclick={detach}>Remove cover and details</button>
        </div>
      </section>
      {@render fieldError('bf-details-err', errors.cover_url || errors.description || errors.external_id)}
    {/if}
    <p class="sr-only" role="status" aria-live="polite">{detailsNote}</p>

    <fieldset class="field" aria-describedby={errors.status ? 'bf-status-err' : undefined}>
      <legend>Status</legend>
      <div class="seg">
        {#each STATUSES as s (s.id)}
          <label class:on={status === s.id}>
            <input type="radio" name="status" value={s.id} bind:group={status} />
            <span aria-hidden="true">{status === s.id ? '●' : '○'}</span> {s.label}
          </label>
        {/each}
      </div>
      {@render fieldError('bf-status-err', errors.status)}
    </fieldset>

    <div class="field">
      <label for="bf-progress">Progress</label>
      <div class="prog">
        <input type="range" id="bf-progress" min="0" max="100" step="1" value={progress} disabled={locked}
          oninput={(e) => setProgress(e.currentTarget.value)} aria-describedby="bf-progress-help" />
        <span class="num">
          <input type="number" name="progress" min="0" max="100" step="1" value={progress} disabled={locked}
            aria-label="Progress percent" oninput={(e) => setProgress(e.currentTarget.value)}
            aria-describedby={errors.progress ? 'bf-progress-err' : 'bf-progress-help'} /> %
        </span>
      </div>
      <p id="bf-progress-help" class="help">
        {#if status === 'to_read'}Locked at 0% while the book is in “To read”.
        {:else if status === 'finished'}Locked at 100% once the book is “Finished”.
        {:else}Drag the slider or type a number from 0 to 100.{/if}
      </p>
      {@render fieldError('bf-progress-err', errors.progress)}
    </div>

    <div class="field">
      <span class="lbl" id="bf-rating-lbl">Rating</span>
      <StarRating bind:value={rating} editable size={30} label="Rating" />
      <p class="help">Click the left or right half of a star, or focus the stars and use ← → in half steps.</p>
      {@render fieldError('bf-rating-err', errors.rating)}
    </div>

    {#if formError}
      <p class="error banner" role="alert"><span>{formError}</span></p>
    {/if}

    <footer>
      <button type="button" class="ghost" onclick={() => dialogEl.close()}>Cancel</button>
      <button type="submit" class="primary" disabled={pending}>{pending ? 'Saving…' : book ? 'Save changes' : 'Add book'}</button>
    </footer>
  </form>
</dialog>

<style>
  dialog {
    width: min(34rem, calc(100vw - 2rem)); max-height: calc(100vh - 2rem); padding: 0; overflow: auto;
    border: 2px solid var(--space-indigo); background: #fff; color: var(--space-indigo);
    box-shadow: 10px 10px 0 var(--space-indigo);
    font-family: var(--font-body);
  }
  dialog[open] { animation: pop 260ms cubic-bezier(0.2, 0.8, 0.2, 1); }
  dialog::backdrop { background: rgba(59, 59, 88, 0.55); backdrop-filter: blur(2px); }
  @keyframes pop { from { opacity: 0; transform: translateY(14px); } to { opacity: 1; transform: none; } }

  form { padding: 1.75rem 2rem 1.5rem; }
  header { margin-bottom: 1.25rem; }
  .eyebrow { margin: 0 0 0.25rem; font-size: 0.75rem; font-weight: 600; letter-spacing: 0.14em; text-transform: uppercase; color: var(--old-rose); }
  h2 { margin: 0; font-family: var(--font-display); font-size: 2rem; font-weight: 600; letter-spacing: -0.01em; }

  .field { margin: 0 0 1.1rem; padding: 0; border: 0; min-width: 0; }
  label, legend, .lbl { display: block; padding: 0; margin-bottom: 0.35rem; font-size: 0.875rem; font-weight: 600; }
  input[type='text'], input[type='number'] {
    width: 100%; box-sizing: border-box; font: inherit; padding: 0.65rem 0.8rem; color: var(--space-indigo);
    background: #fff; border: 2px solid #8a8aa0; border-radius: 0;
  }
  input[type='text']:hover { border-color: var(--space-indigo); }
  input:focus-visible { outline: 3px solid var(--amethyst-smoke); outline-offset: 2px; }
  input[aria-invalid='true'] { border-color: #a4161a; }
  input:disabled { background: #eeecf3; color: #4a4a5c; border-color: #c9c6d3; }

  .details { display: flex; gap: 1rem; align-items: flex-start; padding: 0.9rem 1rem; background: #f6f5f8; border: 1px dashed #8a8aa0; border-left: 4px solid var(--amethyst-smoke); }
  .dbody { min-width: 0; flex: 1; display: flex; flex-direction: column; align-items: flex-start; gap: 0.4rem; }
  h3 { margin: 0; font-size: 0.75rem; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--old-rose); }
  .snippet { margin: 0; font-size: 0.85rem; line-height: 1.45; color: #4a4a5c; overflow-wrap: anywhere; display: -webkit-box; -webkit-line-clamp: 5; line-clamp: 5; -webkit-box-orient: vertical; overflow: hidden; }
  .snippet.none { font-style: italic; }
  .link { font: inherit; cursor: pointer; background: none; border: 0; padding: 0.25rem 0; font-size: 0.85rem; font-weight: 600; color: var(--amethyst-smoke); text-decoration: underline; text-underline-offset: 3px; }
  .link:hover { color: var(--space-indigo); }
  @media (max-width: 420px) { .details { flex-direction: column; } }

  .seg { display: grid; grid-template-columns: repeat(3, 1fr); }
  .seg label { position: relative; margin: 0; padding: 0.6rem 0.4rem; text-align: center; cursor: pointer; border: 2px solid #8a8aa0; margin-left: -2px; font-weight: 600; }
  .seg label:first-child { margin-left: 0; }
  .seg label.on { background: var(--space-indigo); color: #fff; border-color: var(--space-indigo); z-index: 1; }
  .seg input { position: absolute; opacity: 0; inset: 0; margin: 0; cursor: pointer; }
  .seg label:has(input:focus-visible) { outline: 3px solid var(--amethyst-smoke); outline-offset: 2px; z-index: 2; }

  .prog { display: flex; align-items: center; gap: 1rem; }
  input[type='range'] { flex: 1; accent-color: var(--dusty-lavender); height: 1.5rem; }
  .num { display: flex; align-items: center; gap: 0.3rem; font-weight: 600; }
  .num input { width: 4.5rem; text-align: right; }
  .help { margin: 0.35rem 0 0; font-size: 0.8rem; color: #4a4a5c; }

  .error { display: flex; align-items: flex-start; gap: 0.45rem; margin: 0.4rem 0 0; padding: 0.45rem 0.7rem; font-size: 0.875rem; font-weight: 600; color: #8a1015; background: #fdecec; border-left: 4px solid #a4161a; }
  .error svg { flex: none; margin-top: 0.1rem; }
  .banner { margin: 0 0 1rem; }

  footer { display: flex; justify-content: flex-end; gap: 0.9rem; margin-top: 1.5rem; }
  footer button { font: inherit; font-weight: 600; padding: 0.7rem 1.3rem; cursor: pointer; }
  .primary { color: #fff; background: var(--dusty-lavender); border: 2px solid var(--dusty-lavender); box-shadow: 4px 4px 0 var(--space-indigo); transition: transform 150ms ease, box-shadow 150ms ease, background-color 150ms ease; }
  .primary:hover:not(:disabled) { background: var(--amethyst-smoke); border-color: var(--amethyst-smoke); transform: translate(2px, 2px); box-shadow: 2px 2px 0 var(--space-indigo); }
  .primary:disabled { opacity: 0.75; cursor: progress; }
  .ghost { color: var(--space-indigo); background: #fff; border: 2px solid var(--space-indigo); }
  .ghost:hover { background: #eeecf3; }

  @media (prefers-reduced-motion: reduce) {
    dialog[open] { animation: none; }
    .primary { transition: none; }
    .primary:hover:not(:disabled) { transform: none; }
  }
</style>
