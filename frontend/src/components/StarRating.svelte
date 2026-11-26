<script>
  // Half-star rating. Read-only by default (an image with a text label);
  // with `editable` it is a slider: click a star's left/right half, or use
  // arrow keys (0.5 steps), Home/End, Delete/Backspace to clear.
  let { value = $bindable(null), editable = false, size = 18, label = 'Rating' } = $props();

  const STAR = 'M12 2.5l2.9 6.1 6.6.8-4.9 4.6 1.3 6.6L12 17.3 6.1 20.6l1.3-6.6L2.5 9.4l6.6-.8z';

  let hover = $state(null);
  const shown = $derived(hover ?? value ?? 0);
  const text = $derived(value ? `${value} out of 5 stars` : 'Not rated');

  const clamp = (n) => Math.min(5, Math.max(0, n));

  function fromPointer(event, i) {
    const rect = event.currentTarget.getBoundingClientRect();
    return i + (event.clientX - rect.left < rect.width / 2 ? 0.5 : 1);
  }

  function onkeydown(event) {
    let next = null;
    const cur = value ?? 0;
    if (event.key === 'ArrowRight' || event.key === 'ArrowUp') next = clamp(cur + 0.5);
    else if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') next = clamp(cur - 0.5);
    else if (event.key === 'Home') next = 0.5;
    else if (event.key === 'End') next = 5;
    else if (event.key === 'Delete' || event.key === 'Backspace') next = 0;
    else return;
    event.preventDefault();
    value = next === 0 ? null : next;
  }
</script>

{#snippet stars()}
  {#each [0, 1, 2, 3, 4] as i}
    {@const fill = Math.min(1, Math.max(0, shown - i))}
    <span class="star" style="width: {size}px; height: {size}px">
      <svg viewBox="0 0 24 24" width={size} height={size} aria-hidden="true">
        <path d={STAR} class="outline" />
      </svg>
      <span class="fill" style="width: {fill * 100}%">
        <svg viewBox="0 0 24 24" width={size} height={size} aria-hidden="true">
          <path d={STAR} class="solid" />
        </svg>
      </span>
      {#if editable}
        <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
        <span
          class="hit"
          onclick={(e) => {
            const v = fromPointer(e, i);
            value = v === value ? null : v;
          }}
          onmousemove={(e) => (hover = fromPointer(e, i))}
        ></span>
      {/if}
    </span>
  {/each}
{/snippet}

{#if editable}
  <div class="wrap">
    <div
      class="stars editable"
      role="slider"
      tabindex="0"
      aria-label={label}
      aria-valuemin="0"
      aria-valuemax="5"
      aria-valuenow={value ?? 0}
      aria-valuetext={text}
      {onkeydown}
      onmouseleave={() => (hover = null)}
    >
      {@render stars()}
    </div>
    <span class="text" aria-hidden="true">{hover ? `${hover}` : value ? `${value}` : '–'}<small> / 5</small></span>
    <button type="button" class="clear" onclick={() => (value = null)} disabled={!value}>Clear rating</button>
  </div>
{:else}
  <span class="wrap">
    <span class="stars" role="img" aria-label={text}>{@render stars()}</span>
    {#if !value}<span class="text muted">Not rated</span>{/if}
  </span>
{/if}

<style>
  .wrap { display: inline-flex; align-items: center; gap: 0.6rem; flex-wrap: wrap; }
  .stars { display: inline-flex; gap: 2px; }
  .editable { cursor: pointer; padding: 2px; border-radius: 2px; }
  .star { position: relative; display: inline-block; }
  .star svg { display: block; }
  .fill { position: absolute; inset: 0 auto 0 0; overflow: hidden; }
  .outline { fill: none; stroke: #7d7d94; stroke-width: 1.6; stroke-linejoin: round; }
  .solid { fill: var(--old-rose); stroke: var(--old-rose); stroke-width: 1.6; stroke-linejoin: round; }
  .hit { position: absolute; inset: 0; }
  .text { font-size: 0.8rem; font-weight: 600; font-variant-numeric: tabular-nums; min-width: 2.6rem; }
  .text small { font-weight: 400; color: #4a4a5c; }
  .muted { font-weight: 500; color: #4a4a5c; font-style: italic; }
  .clear {
    font: inherit; font-size: 0.8rem; font-weight: 600; background: none; border: 0;
    color: var(--amethyst-smoke); text-decoration: underline; cursor: pointer; padding: 0.2rem;
  }
  .clear:disabled { color: #6b6b80; cursor: default; text-decoration: none; }
</style>
