<script>
  let { value = 0, label = 'Progress' } = $props();
  const pct = $derived(Math.min(100, Math.max(0, Math.round(value))));
</script>

<div class="progress">
  <div class="track" role="progressbar" aria-label={label} aria-valuemin="0" aria-valuemax="100" aria-valuenow={pct}>
    <span class="bar" style="width: {pct}%"></span>
    <!-- Tick marks every 25% so the bar reads like a ruler, not just a color fill. -->
    <span class="ticks" aria-hidden="true"></span>
  </div>
  <span class="pct">{pct}%</span>
</div>

<style>
  .progress { display: flex; align-items: center; gap: 0.6rem; }
  .track { position: relative; flex: 1; height: 0.6rem; background: #e6e2ec; border: 1px solid #8a8aa0; }
  .bar { position: absolute; inset: 0 auto 0 0; background: var(--dusty-lavender); transition: width 500ms cubic-bezier(0.65, 0, 0.35, 1); }
  .ticks {
    position: absolute; inset: 0;
    background: repeating-linear-gradient(90deg, transparent 0 calc(25% - 1px), rgba(255, 255, 255, 0.85) calc(25% - 1px) 25%);
  }
  .pct { min-width: 2.6rem; text-align: right; font-size: 0.8rem; font-weight: 600; font-variant-numeric: tabular-nums; }
  @media (prefers-reduced-motion: reduce) { .bar { transition: none; } }
</style>
