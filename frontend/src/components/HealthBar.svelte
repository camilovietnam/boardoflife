<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import { getHealthbar } from '../api.js';

  // Arcade-style vitality gauge for one section (books, exercise, ...). Display only.
  // 20 half-point segments; the fill is masked into segments by a gap overlay.
  let { section, label = 'Health' } = $props();

  const SEGMENTS = 20;

  let status = $state('loading'); // 'loading' | 'ready' | 'error'
  let health = $state(0); // value the meter reports (real value)
  let max = $state(10);
  let shown = $state(0); // value the fill is drawn at (0 first, so it animates in)
  let trail = $state(0); // lagging "damage" layer
  let controller;
  let seq = 0;

  const state = $derived(health <= 3 ? 'critical' : health <= 6 ? 'weakened' : 'healthy');
  const WORD = { healthy: 'Healthy', weakened: 'Weakened', critical: 'Critical' };
  const word = $derived(WORD[state]);
  const pct = (v) => (max > 0 ? Math.min(100, Math.max(0, (v / max) * 100)) : 0);
  const fmt = (v) => (Number.isInteger(v) ? String(v) : v.toFixed(1));
  const valuetext = $derived(`${label} health: ${fmt(health)} out of ${fmt(max)}, ${word.toLowerCase()}`);
  const gap = $derived(`${100 / SEGMENTS}%`);

  async function load() {
    const mine = ++seq;
    controller?.abort();
    controller = new AbortController();
    status = 'loading';
    try {
      const res = await getHealthbar(section, { signal: controller.signal });
      if (mine !== seq) return;
      const v = Number(res?.health);
      const m = Number(res?.max);
      if (!Number.isFinite(v) || !Number.isFinite(m) || m <= 0) throw new Error('bad payload');
      max = m;
      health = Math.min(m, Math.max(0, v));
      status = 'ready';
      await tick(); // render at 0 first so the fill sweeps in
      requestAnimationFrame(() => {
        if (mine !== seq) return;
        shown = health;
        trail = health;
      });
    } catch (err) {
      if (err?.name === 'AbortError' || mine !== seq) return;
      status = 'error';
    }
  }

  // If health later drops, the bright fill moves at once and the trail catches up slowly.
  $effect(() => {
    if (status !== 'ready') return;
    const h = health;
    if (h < shown) shown = h;
    else if (h > shown) {
      shown = h;
      trail = h;
    }
  });

  onMount(load);
  onDestroy(() => {
    seq += 1;
    controller?.abort();
  });
</script>

<div class="hb" data-state={status === 'ready' ? state : undefined} style="--seg: {gap}">
  {#if status === 'error'}
    <p class="err">
      <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M12 7v6" /><path d="M12 16.5v.01" /></svg>
      <span>Health unavailable</span>
      <button type="button" onclick={load} aria-label={`Retry loading ${label} health`}>Retry</button>
    </p>
  {:else}
    <div class="frame" class:skeleton={status === 'loading'} aria-busy={status === 'loading'}>
      <div class="head">
        <span class="icon" aria-hidden="true">
          {#if status !== 'ready' || state === 'healthy'}
            <svg viewBox="0 0 16 16" width="20" height="20" shape-rendering="crispEdges"><path fill="currentColor" d="M2 3h4v1h1v1h2V4h1V3h4v1h1v5h-1v1h-1v1h-1v1h-1v1H9v1H7v-1H6v-1H5v-1H4v-1H3V9H2z" /></svg>
          {:else if state === 'weakened'}
            <svg viewBox="0 0 16 16" width="20" height="20" shape-rendering="crispEdges"><path fill="currentColor" fill-rule="evenodd" d="M2 3h4v1h1v1h2V4h1V3h4v1h1v5h-1v1h-1v1h-1v1h-1v1H9v1H7v-1H6v-1H5v-1H4v-1H3V9H2zM8 5h1v2H8zm-1 2h1v2H7zm1 2h1v2H8z" /></svg>
          {:else}
            <svg viewBox="0 0 16 16" width="20" height="20" shape-rendering="crispEdges"><path fill="currentColor" d="M2 3h4v1h1v1h1v2H7v2h1v2H7v1H6v-1H5v-1H4v-1H3V9H2zM10 3h4v1h1v5h-1v1h-1v1h-1v1h-1v1H9v-1h1v-2H9V8h1V6H9V4h1z" /></svg>
          {/if}
        </span>
        <span class="name">{label} <abbr title="Vitality">VIT</abbr></span>
        <span class="state">{status === 'ready' ? word : 'Loading'}</span>
        <span class="num" aria-hidden="true">{status === 'ready' ? `${fmt(health)} / ${fmt(max)}` : '– / –'}</span>
      </div>

      {#if status === 'ready'}
        <div
          class="track"
          role="meter"
          aria-label={`${label} health`}
          aria-valuemin="0"
          aria-valuemax={max}
          aria-valuenow={health}
          aria-valuetext={valuetext}
        >
          <span class="trail" style="width: {pct(trail)}%"></span>
          <span class="fill" style="width: {pct(shown)}%"></span>
          <span class="cuts" aria-hidden="true"></span>
        </div>
      {:else}
        <div class="track" aria-hidden="true"><span class="cuts"></span></div>
        <p class="sr-only" role="status">Loading {label} health…</p>
      {/if}
    </div>
  {/if}
</div>

<style>
  .hb {
    --hue: #1b6b3a; /* text/badge color per state, >= 4.5:1 on white */
    --bar: #4cc27a; /* fill on the dark track, >= 3:1 */
    --well: #1d1d2e;
    --texture: none;
    margin: 0.2rem 0 0.1rem;
    max-width: 30rem;
    min-height: 4.6rem; /* reserve space: loading / ready / error never shift the list */
  }
  .hb[data-state='weakened'] { --hue: #7a4f00; --bar: #f0b429; --texture: repeating-linear-gradient(135deg, rgba(0, 0, 0, 0.32) 0 3px, transparent 3px 7px); }
  .hb[data-state='critical'] { --hue: #a4161a; --bar: #ff6b6b; --texture: radial-gradient(rgba(0, 0, 0, 0.45) 1.4px, transparent 1.8px) 0 0 / 5px 5px; }

  .frame {
    background: #fff;
    border: 2px solid var(--space-indigo);
    /* Stepped corner + hard shadow: the HUD plate. */
    box-shadow: 4px 4px 0 var(--space-indigo), inset 0 0 0 2px #fff, inset 0 0 0 3px var(--rosy-taupe);
    padding: 0.55rem 0.8rem 0.7rem;
    animation: plate 480ms cubic-bezier(0.2, 0.8, 0.2, 1) backwards;
  }
  @keyframes plate { from { opacity: 0; transform: translateY(-6px); } to { opacity: 1; transform: none; } }

  .head { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.4rem; font-size: 0.8rem; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; }
  .icon { display: inline-flex; color: var(--hue); }
  .name { color: var(--space-indigo); }
  .name abbr { text-decoration: none; color: var(--rosy-taupe); }
  .state { padding: 0.05rem 0.45rem; color: #fff; background: var(--hue); font-size: 0.7rem; }
  .num { margin-left: auto; font-variant-numeric: tabular-nums; font-size: 1rem; letter-spacing: 0.04em; color: var(--space-indigo); }

  .track {
    position: relative;
    height: 1.35rem;
    background: var(--well);
    border: 2px solid var(--space-indigo);
    overflow: hidden;
  }
  .fill, .trail { position: absolute; inset: 0 auto 0 0; }
  .fill {
    background: var(--texture), linear-gradient(180deg, rgba(255, 255, 255, 0.45) 0 18%, transparent 18% 100%), var(--bar);
    transition: width 900ms cubic-bezier(0.65, 0, 0.35, 1) 150ms;
    overflow: hidden;
  }
  /* Lagging damage trail: a paler sliver that drains after the fill. */
  .trail { background: #fff; opacity: 0.85; transition: width 1100ms cubic-bezier(0.65, 0, 0.35, 1) 700ms; }
  /* Segment gaps: 20 cuts, so every half point is its own cell. */
  .cuts {
    position: absolute;
    inset: 0;
    pointer-events: none;
    background: repeating-linear-gradient(90deg, transparent 0 calc(var(--seg) - 3px), var(--well) calc(var(--seg) - 3px) var(--seg));
  }
  /* Shine sweep across the fill. */
  .fill::after {
    content: '';
    position: absolute;
    inset: 0;
    width: 40%;
    background: linear-gradient(100deg, transparent, rgba(255, 255, 255, 0.55), transparent);
    transform: translateX(-120%);
    animation: shine 3.6s ease-in-out 1.4s infinite;
  }
  @keyframes shine { 0%, 55% { transform: translateX(-120%); } 100% { transform: translateX(320%); } }

  [data-state='critical'] .frame { animation: plate 480ms cubic-bezier(0.2, 0.8, 0.2, 1) backwards, alarm 1.2s ease-in-out 600ms infinite; }
  [data-state='critical'] .fill::after { display: none; }
  @keyframes alarm { 50% { box-shadow: 4px 4px 0 var(--space-indigo), inset 0 0 0 2px #fff, inset 0 0 0 3px #a4161a; } }

  .skeleton .track { background: #e6e2ec; }
  .skeleton .cuts { background: repeating-linear-gradient(90deg, transparent 0 calc(var(--seg) - 3px), #fff calc(var(--seg) - 3px) var(--seg)); }
  .skeleton { animation: plate 480ms ease backwards, breathe 1.4s ease-in-out infinite; }
  .skeleton .state, .skeleton .icon { color: #4a4a5c; }
  .skeleton .state { background: #e6e2ec; color: #3b3b58; }
  @keyframes breathe { 50% { opacity: 0.6; } }

  .err { display: flex; align-items: center; gap: 0.5rem; margin: 0; min-height: 2rem; font-size: 0.85rem; color: #4a4a5c; }
  .err svg { flex: none; color: var(--rosy-taupe); }
  .err button { font: inherit; font-weight: 600; font-size: 0.85rem; cursor: pointer; background: none; border: 0; padding: 0.25rem 0; color: var(--amethyst-smoke); text-decoration: underline; text-underline-offset: 3px; }
  .err button:hover { color: var(--space-indigo); }

  @media (max-width: 420px) {
    .head { flex-wrap: wrap; row-gap: 0.2rem; }
    .num { font-size: 0.9rem; }
  }
  @media (prefers-reduced-motion: reduce) {
    .frame, .skeleton, [data-state='critical'] .frame { animation: none; }
    .fill, .trail { transition: none; }
    .fill::after { display: none; }
  }
</style>
