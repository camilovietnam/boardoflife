<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import { listBooks, updateBook, deleteBook } from '../api.js';
  import StarRating from './StarRating.svelte';
  import ProgressBar from './ProgressBar.svelte';
  import Pagination from './Pagination.svelte';
  import BookForm from './BookForm.svelte';
  import BookCover from './BookCover.svelte';
  import HealthBar from './HealthBar.svelte';

  const PAGE_SIZE = 12;
  const TABS = [
    { id: 'to_read', label: 'To read', icon: 'bookmark' },
    { id: 'reading', label: 'Reading', icon: 'book' },
    { id: 'finished', label: 'Finished', icon: 'check' },
  ];
  const EMPTY = {
    to_read: { title: 'The stack is empty', text: 'Nothing waiting on the nightstand. Add the books you mean to get to someday.' },
    reading: { title: 'No book in hand', text: 'Pick something from your list and start a chapter tonight.' },
    finished: { title: 'No finished books yet', text: 'Your first closed cover will be shelved here.' },
  };

  let tab = $state('reading');
  let query = $state('');
  let page = $state(1);
  let items = $state([]);
  let meta = $state({ total: 0, total_pages: 1, page: 1 });
  let counts = $state({ to_read: 0, reading: 0, finished: 0 });
  let loaded = $state(false); // first response received
  let loading = $state(true);
  let failed = $state(false);
  let batch = $state(0); // bump to replay the staggered reveal
  let announcement = $state('');
  let formOpen = $state(false);
  let editing = $state(null);
  let confirmId = $state(null);
  let busyId = $state(null);
  let aboutOpen = $state({}); // book id -> description disclosure expanded
  let addBtn;
  let trigger = null;
  let seq = 0;
  let debounce;

  const searching = $derived(query.trim() !== '');

  async function load({ reveal = true } = {}) {
    const mine = ++seq;
    loading = true;
    failed = false;
    try {
      const res = await listBooks({ status: tab, q: query.trim(), page, pageSize: PAGE_SIZE });
      if (mine !== seq) return;
      // Deleting the last item of a later page: step back one page.
      if (res.items.length === 0 && res.total > 0 && page > 1) {
        page = Math.max(1, res.total_pages);
        return load({ reveal });
      }
      items = res.items;
      meta = { total: res.total, total_pages: Math.max(1, res.total_pages), page: res.page };
      counts = res.counts;
      loaded = true;
      if (reveal) batch += 1;
    } catch {
      if (mine !== seq) return;
      failed = true;
    } finally {
      if (mine === seq) loading = false;
    }
  }

  onMount(() => load());
  onDestroy(() => clearTimeout(debounce));

  function selectTab(id) {
    if (id === tab) return;
    tab = id;
    page = 1;
    confirmId = null;
    load();
  }

  function onSearch() {
    clearTimeout(debounce);
    debounce = setTimeout(() => {
      page = 1;
      load();
    }, 300);
  }

  function clearSearch() {
    query = '';
    page = 1;
    load();
  }

  function goPage(p) {
    page = p;
    confirmId = null;
    load();
  }

  function openForm(book, event) {
    trigger = event?.currentTarget ?? null;
    editing = book;
    formOpen = true;
  }

  async function closeForm() {
    formOpen = false;
    editing = null;
    await tick();
    (trigger?.isConnected ? trigger : addBtn)?.focus();
  }

  async function saved(book) {
    announcement = `Saved “${book.name}”.`;
    if (book.status !== tab) {
      tab = book.status;
      page = 1;
    }
    // The trigger card may move/disappear; fall back to the Add button.
    trigger = null;
    await load();
  }

  async function quick(book, patch) {
    busyId = book.id;
    // PUT is a full replace: carry the catalog fields through or they would be wiped.
    const next = {
      name: book.name, author: book.author, status: book.status, progress: book.progress, rating: book.rating,
      cover_url: book.cover_url ?? null, description: book.description ?? null, external_id: book.external_id ?? null,
      ...patch,
    };
    if (next.status === 'to_read') next.progress = 0;
    if (next.status === 'finished') next.progress = 100;
    try {
      await updateBook(book.id, next);
      announcement = `Updated “${book.name}”.`;
      await load({ reveal: false });
    } catch {
      announcement = `Could not update “${book.name}”.`;
    } finally {
      busyId = null;
    }
  }

  async function askDelete(book) {
    confirmId = book.id;
    await tick();
    document.getElementById(`cancel-${book.id}`)?.focus();
  }

  async function doDelete(book) {
    busyId = book.id;
    try {
      await deleteBook(book.id);
      announcement = `Deleted “${book.name}”.`;
    } catch {
      announcement = `Could not delete “${book.name}”.`;
    }
    confirmId = null;
    busyId = null;
    await load({ reveal: false });
    addBtn?.focus();
  }

  const statusLabel = (s) => TABS.find((t) => t.id === s)?.label ?? s;
</script>

{#snippet icon(name)}
  <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    {#if name === 'bookmark'}<path d="M6 3h12v18l-6-4-6 4z" />
    {:else if name === 'book'}<path d="M2 5c3-1.5 7-1.5 10 1v14c-3-2.5-7-2.5-10-1z" /><path d="M22 5c-3-1.5-7-1.5-10 1v14c3-2.5 7-2.5 10-1z" />
    {:else if name === 'check'}<circle cx="12" cy="12" r="9" /><path d="M8 12.5l3 3 5-6" />
    {:else if name === 'search'}<circle cx="11" cy="11" r="7" /><path d="M20 20l-4-4" />
    {:else if name === 'plus'}<path d="M12 5v14M5 12h14" />{/if}
  </svg>
{/snippet}

<section class="books" aria-labelledby="books-title">
  <header class="head">
    <div>
      <p class="eyebrow">Reading journal</p>
      <h1 id="books-title">Books</h1>
      <HealthBar section="books" label="Books" />
    </div>
    <button type="button" class="add" bind:this={addBtn} onclick={(e) => openForm(null, e)}>
      {@render icon('plus')} Add book
    </button>
  </header>

  <div class="toolbar">
    <div class="tabs" role="group" aria-label="Filter by status">
      {#each TABS as t (t.id)}
        <button type="button" class="tab" class:on={tab === t.id} aria-pressed={tab === t.id} onclick={() => selectTab(t.id)}>
          {@render icon(t.icon)}
          <span>{t.label}</span>
          <span class="count" aria-label={`${counts[t.id]} books`}>{counts[t.id]}</span>
        </button>
      {/each}
    </div>

    <div class="search">
      {@render icon('search')}
      <label class="sr-only" for="book-search">Search books by title or author</label>
      <input id="book-search" type="search" placeholder="Search title or author" autocomplete="off" bind:value={query} oninput={onSearch} />
    </div>
  </div>

  <div class="results" aria-busy={loading}>
    {#if failed}
      <div class="notice err" role="alert">
        <h2>Couldn't load your books</h2>
        <p>The shelf didn't answer. Your data is safe.</p>
        <button type="button" class="ghost" onclick={() => load()}>Try again</button>
      </div>
    {:else if !loaded}
      <ul class="list" aria-hidden="true">
        {#each Array(6) as _, i}
          <li class="skel" style="--d: {i * 70}ms"><span></span><span></span><span></span></li>
        {/each}
      </ul>
      <p class="sr-only" role="status">Loading books…</p>
    {:else if items.length === 0}
      <div class="notice">
        <svg class="art" viewBox="0 0 120 80" width="120" height="80" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linejoin="round">
          <rect x="14" y="14" width="14" height="56" /><rect x="32" y="24" width="12" height="46" /><rect x="48" y="10" width="16" height="60" />
          <path d="M74 70l16-48 14 5-14 43z" /><path d="M6 70h108" />
        </svg>
        {#if searching}
          <h2>No matches in “{statusLabel(tab)}”</h2>
          <p>Nothing here matches “{query.trim()}”. Try another title or author.</p>
          <button type="button" class="ghost" onclick={clearSearch}>Clear search</button>
        {:else}
          <h2>{EMPTY[tab].title}</h2>
          <p>{EMPTY[tab].text}</p>
          <button type="button" class="ghost" onclick={(e) => openForm(null, e)}>Add a book</button>
        {/if}
      </div>
    {:else}
      {#key batch}
        <ul class="list" class:dim={loading}>
          {#each items as book, i (book.id)}
            <li class="book" class:pending={busyId === book.id} data-status={book.status} style="--d: {i * 55}ms">
              <span class="spine" aria-hidden="true"></span>
              <div class="cover-col"><BookCover name={book.name} author={book.author} src={book.cover_url} /></div>
              <div class="body">
                <div class="top">
                  <div class="titles">
                    <h2>{book.name}</h2>
                    <p class="author">by {book.author}</p>
                  </div>
                  <span class="badge">{@render icon(TABS.find((t) => t.id === book.status).icon)} {statusLabel(book.status)}</span>
                </div>

                <ProgressBar value={book.progress} label={`Progress for ${book.name}`} />
                <StarRating value={book.rating} label={`Rating for ${book.name}`} />

                {#if book.description}
                  <div class="about">
                    <button type="button" class="about-btn" aria-expanded={aboutOpen[book.id] === true} aria-controls={`about-${book.id}`}
                      onclick={() => (aboutOpen[book.id] = !aboutOpen[book.id])}>
                      <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M9 6l6 6-6 6" /></svg>
                      About this book<span class="sr-only">: {book.name}</span>
                    </button>
                    <!-- Plain text only: never {@html}. Focusable so keyboard users can scroll long text. -->
                    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
                    <div id={`about-${book.id}`} class="about-text" role="region" aria-label={`About ${book.name}`} tabindex="0" hidden={aboutOpen[book.id] !== true}>{book.description}</div>
                  </div>
                {/if}

                {#if confirmId === book.id}
                  <div class="confirm" role="group" aria-label={`Delete ${book.name}?`}>
                    <span>Delete “{book.name}” for good?</span>
                    <button type="button" class="danger" onclick={() => doDelete(book)} disabled={busyId === book.id}>Yes, delete</button>
                    <button type="button" class="link" id={`cancel-${book.id}`} onclick={() => (confirmId = null)}>Cancel</button>
                  </div>
                {:else}
                  <div class="actions">
                    {#if book.status === 'to_read'}
                      <button type="button" class="link" onclick={() => quick(book, { status: 'reading' })}>Start reading</button>
                    {:else if book.status === 'reading'}
                      <button type="button" class="link" disabled={book.progress >= 100} onclick={() => quick(book, { progress: Math.min(100, book.progress + 10) })} aria-label={`Add 10 percent progress to ${book.name}`}>+10%</button>
                      <button type="button" class="link" onclick={() => quick(book, { status: 'finished' })}>Mark finished</button>
                    {:else}
                      <button type="button" class="link" onclick={() => quick(book, { status: 'reading', progress: 0 })}>Read again</button>
                    {/if}
                    <span class="grow"></span>
                    <button type="button" class="link" onclick={(e) => openForm(book, e)} aria-label={`Edit ${book.name}`}>Edit</button>
                    <button type="button" class="link warn" onclick={() => askDelete(book)} aria-label={`Delete ${book.name}`}>Delete</button>
                  </div>
                {/if}
              </div>
            </li>
          {/each}
        </ul>
      {/key}
    {/if}
  </div>

  {#if loaded && !failed && meta.total > 0}
    <Pagination page={meta.page} totalPages={meta.total_pages} total={meta.total} pageSize={PAGE_SIZE} onpage={goPage} />
  {/if}

  <p class="sr-only" role="status" aria-live="polite">{announcement}</p>
</section>

{#if formOpen}
  <BookForm book={editing} onsaved={saved} onclose={closeForm} />
{/if}

<style>
  .books { display: flex; flex-direction: column; gap: 1.25rem; min-height: 100%; }
  .head { display: flex; align-items: flex-end; justify-content: space-between; gap: 1rem; flex-wrap: wrap; }
  .eyebrow { margin: 0 0 0.2rem; font-size: 0.75rem; font-weight: 600; letter-spacing: 0.16em; text-transform: uppercase; color: var(--rosy-taupe); }
  h1 { margin: 0; font-family: var(--font-display); font-size: clamp(2.2rem, 4vw, 3.2rem); line-height: 1; font-weight: 600; letter-spacing: -0.02em; color: var(--space-indigo); font-variation-settings: 'opsz' 144; }

  button { font: inherit; cursor: pointer; }
  .add { display: inline-flex; align-items: center; gap: 0.45rem; font-weight: 600; padding: 0.7rem 1.2rem; color: #fff; background: var(--dusty-lavender); border: 0; box-shadow: 5px 5px 0 var(--space-indigo); transition: transform 150ms ease, box-shadow 150ms ease, background-color 150ms ease; }
  .add:hover { background: var(--amethyst-smoke); transform: translate(2px, 2px); box-shadow: 3px 3px 0 var(--space-indigo); }

  .toolbar { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0.9rem 1.5rem; border-bottom: 2px solid var(--space-indigo); padding-bottom: 0.9rem; }
  .tabs { display: flex; flex-wrap: wrap; gap: 0.4rem; }
  .tab { display: inline-flex; align-items: center; gap: 0.45rem; font-weight: 600; padding: 0.5rem 0.85rem; color: var(--space-indigo); background: #fff; border: 2px solid #8a8aa0; transition: background-color 150ms ease, color 150ms ease, border-color 150ms ease; }
  .tab:hover:not(.on) { border-color: var(--space-indigo); background: #eeecf3; }
  .tab.on { color: #fff; background: var(--space-indigo); border-color: var(--space-indigo); }
  .count { min-width: 1.4rem; padding: 0 0.35rem; text-align: center; font-size: 0.78rem; font-variant-numeric: tabular-nums; border: 1.5px solid currentColor; border-radius: 999px; }

  .search { position: relative; display: flex; align-items: center; color: var(--space-indigo); }
  .search svg { position: absolute; left: 0.7rem; pointer-events: none; }
  .search input { font: inherit; width: min(18rem, 100%); box-sizing: border-box; padding: 0.55rem 0.8rem 0.55rem 2.2rem; color: var(--space-indigo); background: #fff; border: 2px solid #8a8aa0; border-radius: 0; }
  .search input:hover { border-color: var(--space-indigo); }

  /* Reserve room for a full page of rows so paging doesn't make the layout jump. */
  .results { min-height: 28rem; flex: 1; }
  .list { list-style: none; margin: 0; padding: 0; display: grid; grid-template-columns: repeat(auto-fill, minmax(21rem, 1fr)); gap: 1rem 1.25rem; transition: opacity 200ms ease; }
  .list.dim { opacity: 0.55; }

  .book { display: flex; background: #fff; border: 1px solid #cfcbd9; box-shadow: 4px 4px 0 rgba(59, 59, 88, 0.1); animation: shelve 520ms cubic-bezier(0.2, 0.8, 0.2, 1) backwards; animation-delay: var(--d); transition: transform 180ms ease, box-shadow 180ms ease; }
  .book:hover { transform: translate(-2px, -2px); box-shadow: 6px 6px 0 rgba(59, 59, 88, 0.16); }
  .book.pending { opacity: 0.6; }
  @keyframes shelve { from { opacity: 0; transform: translateY(18px) rotate(0.6deg); } to { opacity: 1; transform: none; } }

  /* The "spine": its pattern (not color alone) encodes status. */
  .spine { flex: 0 0 0.9rem; background-color: var(--dusty-lavender); }
  [data-status='to_read'] .spine { background: repeating-linear-gradient(0deg, var(--rosy-taupe) 0 6px, #fff 6px 12px); border-right: 2px solid var(--rosy-taupe); }
  [data-status='reading'] .spine { background: repeating-linear-gradient(135deg, var(--amethyst-smoke) 0 5px, #e9dbe9 5px 10px); border-right: 2px solid var(--amethyst-smoke); }
  [data-status='finished'] .spine { background: var(--space-indigo); }

  .body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 0.7rem; padding: 0.95rem 1.1rem 0.7rem; }
  .top { display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 0.5rem 0.75rem; }

  /* Cover sits beside the text; its box is fixed (2:3) so lazy images never shift the card. */
  .cover-col { flex: none; padding: 0.95rem 0 0.7rem 1rem; }
  .book:has(.cover-col) .body { padding-left: 1rem; }
  .book { container-type: inline-size; }
  @container (max-width: 19rem) { .cover-col { padding-left: 0.7rem; } .cover-col :global(.cover) { --w: 3.9rem; } .book:has(.cover-col) .body { padding-left: 0.7rem; padding-right: 0.8rem; } }

  .about { display: flex; flex-direction: column; gap: 0.35rem; }
  .about-btn { align-self: flex-start; display: inline-flex; align-items: center; gap: 0.35rem; background: none; border: 0; padding: 0.2rem 0; font-size: 0.85rem; font-weight: 600; color: var(--amethyst-smoke); }
  .about-btn:hover { color: var(--space-indigo); }
  .about-btn svg { transition: transform 150ms ease; }
  .about-btn[aria-expanded='true'] svg { transform: rotate(90deg); }
  .about-text { max-height: 9rem; overflow-y: auto; padding: 0.55rem 0.75rem; font-size: 0.85rem; line-height: 1.5; color: #4a4a5c; background: #f6f5f8; border-left: 3px solid var(--amethyst-smoke); white-space: pre-line; overflow-wrap: anywhere; animation: unfold 200ms ease; }
  .about-text[hidden] { display: none; }
  @keyframes unfold { from { opacity: 0; transform: translateY(-4px); } to { opacity: 1; transform: none; } }
  .titles { min-width: 0; }
  h2 { margin: 0; font-family: var(--font-display); font-size: 1.25rem; font-weight: 600; line-height: 1.2; overflow-wrap: anywhere; }
  .author { margin: 0.2rem 0 0; font-size: 0.875rem; font-style: italic; color: #4a4a5c; }
  .badge { flex: none; display: inline-flex; align-items: center; gap: 0.3rem; font-size: 0.72rem; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; padding: 0.2rem 0.5rem; border: 1.5px solid currentColor; color: var(--old-rose); }
  [data-status='finished'] .badge { color: #fff; background: var(--space-indigo); border-color: var(--space-indigo); }

  .actions, .confirm { display: flex; align-items: center; gap: 0.9rem; flex-wrap: wrap; margin-top: auto; padding-top: 0.5rem; border-top: 1px dashed #cfcbd9; min-height: 2rem; }
  .grow { flex: 1; }
  .link { background: none; border: 0; padding: 0.25rem 0; font-size: 0.85rem; font-weight: 600; color: var(--amethyst-smoke); text-decoration: underline; text-underline-offset: 3px; }
  .link:hover:not(:disabled) { color: var(--space-indigo); }
  .link:disabled { color: #6b6b80; cursor: default; text-decoration: none; }
  .link.warn { color: #8a1015; }
  .confirm { font-size: 0.85rem; font-weight: 600; background: #fdecec; border-top: 0; border-left: 4px solid #a4161a; padding: 0.5rem 0.7rem; color: #8a1015; }
  .danger { font-weight: 600; font-size: 0.85rem; padding: 0.35rem 0.8rem; color: #fff; background: #a4161a; border: 0; }
  .danger:hover:not(:disabled) { background: #8a1015; }

  .ghost { font-weight: 600; padding: 0.55rem 1.1rem; color: var(--space-indigo); background: #fff; border: 2px solid var(--space-indigo); }
  .ghost:hover { background: #eeecf3; }

  .notice { display: flex; flex-direction: column; align-items: center; text-align: center; gap: 0.5rem; padding: 3rem 1rem; border: 2px dashed #8a8aa0; background: rgba(255, 255, 255, 0.6); }
  .notice h2 { font-size: 1.6rem; color: var(--space-indigo); }
  .notice p { margin: 0 0 0.75rem; max-width: 30ch; line-height: 1.5; color: #4a4a5c; }
  .notice .art { color: var(--old-rose); margin-bottom: 0.5rem; }
  .notice.err { border: 1px solid #cfcbd9; border-left: 6px solid #a4161a; align-items: flex-start; text-align: left; border-style: solid; border-left-width: 6px; }

  .skel { height: 9.5rem; display: flex; flex-direction: column; gap: 0.8rem; padding: 1rem 1.2rem; background: #fff; border: 1px solid #e0dde8; animation: breathe 1.4s ease-in-out infinite; animation-delay: var(--d); }
  .skel span { display: block; height: 0.9rem; background: #e6e2ec; }
  .skel span:nth-child(1) { width: 65%; height: 1.2rem; }
  .skel span:nth-child(2) { width: 40%; }
  .skel span:nth-child(3) { width: 85%; }
  @keyframes breathe { 50% { opacity: 0.55; } }

  .books > :global(.pager) { padding-top: 0.5rem; border-top: 2px solid var(--space-indigo); }

  @media (max-width: 720px) { .list { grid-template-columns: 1fr; } }
  @media (prefers-reduced-motion: reduce) {
    .book, .skel { animation: none; }
    .book, .add, .tab, .list { transition: none; }
    .about-text { animation: none; }
    .about-btn svg { transition: none; }
    .book:hover, .add:hover { transform: none; }
  }
</style>
