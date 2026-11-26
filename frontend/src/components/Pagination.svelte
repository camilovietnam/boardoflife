<script>
  let { page, totalPages, total, pageSize, onpage } = $props();

  const from = $derived(total === 0 ? 0 : (page - 1) * pageSize + 1);
  const to = $derived(Math.min(total, page * pageSize));

  // 1 … 4 5 [6] 7 8 … 20
  const pages = $derived.by(() => {
    const set = new Set([1, totalPages, page - 1, page, page + 1]);
    const list = [...set].filter((p) => p >= 1 && p <= totalPages).sort((a, b) => a - b);
    const out = [];
    for (const p of list) {
      if (out.length && p - out[out.length - 1] === 2) out.push(p - 1);
      else if (out.length && p - out[out.length - 1] > 2) out.push('…');
      out.push(p);
    }
    return out;
  });
</script>

<nav class="pager" aria-label="Pagination">
  <p class="range" aria-live="polite">Showing <strong>{from}–{to}</strong> of <strong>{total}</strong></p>
  {#if totalPages > 1}
    <ul>
      <li>
        <button type="button" class="step" disabled={page <= 1} onclick={() => onpage(page - 1)} aria-label="Previous page">
          <span aria-hidden="true">←</span> Prev
        </button>
      </li>
      {#each pages as p, i (typeof p === 'number' ? p : `gap${i}`)}
        <li>
          {#if p === '…'}
            <span class="gap" aria-hidden="true">…</span>
          {:else}
            <button
              type="button"
              class="num"
              class:current={p === page}
              aria-current={p === page ? 'page' : undefined}
              aria-label={`Page ${p}`}
              onclick={() => p !== page && onpage(p)}
            >{p}</button>
          {/if}
        </li>
      {/each}
      <li>
        <button type="button" class="step" disabled={page >= totalPages} onclick={() => onpage(page + 1)} aria-label="Next page">
          Next <span aria-hidden="true">→</span>
        </button>
      </li>
    </ul>
  {/if}
</nav>

<style>
  .pager { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0.75rem 1.5rem; }
  .range { margin: 0; font-size: 0.875rem; color: #4a4a5c; }
  .range strong { color: var(--space-indigo); font-variant-numeric: tabular-nums; }
  ul { display: flex; flex-wrap: wrap; gap: 0.3rem; list-style: none; margin: 0; padding: 0; align-items: center; }
  button {
    font: inherit; font-size: 0.875rem; font-weight: 600; min-width: 2.4rem; height: 2.4rem; padding: 0 0.7rem;
    color: var(--space-indigo); background: #fff; border: 2px solid #8a8aa0; cursor: pointer;
    transition: background-color 150ms ease, color 150ms ease, border-color 150ms ease;
  }
  button:hover:not(:disabled):not(.current) { border-color: var(--space-indigo); background: #eeecf3; }
  button:disabled { color: #6b6b80; border-color: #c9c6d3; cursor: default; }
  .current { background: var(--space-indigo); color: #fff; border-color: var(--space-indigo); text-decoration: underline; text-underline-offset: 4px; cursor: default; }
  .gap { display: inline-block; width: 1.4rem; text-align: center; color: #4a4a5c; }
  @media (prefers-reduced-motion: reduce) { button { transition: none; } }
</style>
