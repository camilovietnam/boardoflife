<script>
  // A 2:3 book cover. Shows the remote image when there is one; otherwise (or when the
  // image fails to load) a generated cover: initials on a palette color chosen from the
  // title, with a small repeating pattern. The box size never depends on the image.
  let { name = '', author = '', src = null, alt = '', size = 'card' } = $props();

  const COLORS = ['--rosy-taupe', '--old-rose', '--amethyst-smoke', '--dusty-lavender', '--space-indigo'];

  let failed = $state(false);
  // Only absolute https URLs are ever handed to <img>.
  const usable = $derived(typeof src === 'string' && src.startsWith('https://') && !failed);

  // A new URL (e.g. picked from search) gets a fresh chance to load.
  $effect(() => {
    if (src !== undefined) failed = false;
  });

  const hash = $derived.by(() => {
    let h = 5381;
    for (const ch of String(name).toLowerCase()) h = ((h * 33) ^ ch.codePointAt(0)) >>> 0;
    return h;
  });
  const color = $derived(COLORS[hash % COLORS.length]);
  const pattern = $derived((hash >>> 3) % 3);
  const initials = $derived.by(() => {
    const words = String(name).split(/\s+/).filter((w) => /[\p{L}\p{N}]/u.test(w));
    const letters = words.slice(0, 2).map((w) => Array.from(w.replace(/^[^\p{L}\p{N}]+/u, ''))[0] ?? '');
    return (letters.join('') || '?').toUpperCase();
  });
</script>

<span class="cover {size}" style="--tone: var({color})">
  {#if usable}
    <img {src} {alt} loading="lazy" decoding="async" referrerpolicy="no-referrer" onerror={() => (failed = true)} />
  {:else}
    <span class="fallback p{pattern}" aria-hidden="true">
      <span class="mono">{initials}</span>
      <span class="rule"></span>
    </span>
    {#if alt}<span class="sr-only">{alt}</span>{/if}
  {/if}
  <!-- Spine crease and page-edge highlight, drawn over either variant. -->
  <span class="edge" aria-hidden="true"></span>
</span>

<style>
  .cover {
    --w: 5.25rem;
    position: relative;
    display: block;
    flex: none;
    width: var(--w);
    aspect-ratio: 2 / 3;
    background: #e6e2ec;
    box-shadow: 3px 3px 0 rgba(59, 59, 88, 0.22), 0 0 0 1px rgba(59, 59, 88, 0.35);
    overflow: hidden;
  }
  .cover.thumb { --w: 2.5rem; box-shadow: 0 0 0 1px rgba(59, 59, 88, 0.35); }
  .cover.preview { --w: 6rem; }

  img { display: block; width: 100%; height: 100%; object-fit: cover; }

  .fallback {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: calc(var(--w) * 0.08);
    background-color: var(--tone);
    color: #fff;
  }
  /* Overlays only ever darken the color, so white initials keep at least their base contrast. */
  .p0 { background-image: repeating-linear-gradient(135deg, rgba(0, 0, 0, 0.16) 0 2px, transparent 2px 9px); }
  .p1 { background-image: radial-gradient(rgba(0, 0, 0, 0.2) 1.5px, transparent 1.8px); background-size: 8px 8px; }
  .p2 { background-image: repeating-linear-gradient(0deg, rgba(0, 0, 0, 0.16) 0 1px, transparent 1px 8px), repeating-linear-gradient(90deg, rgba(0, 0, 0, 0.16) 0 1px, transparent 1px 8px); }

  .mono {
    font-family: var(--font-display);
    font-size: calc(var(--w) * 0.38);
    font-weight: 600;
    line-height: 1;
    letter-spacing: 0.02em;
    padding: 0.12em 0.2em;
    background: var(--tone);
  }
  .rule { width: 46%; height: 5px; box-sizing: border-box; border-top: 1px solid currentColor; border-bottom: 1px solid currentColor; }
  .thumb .rule { display: none; }

  .edge {
    position: absolute;
    inset: 0;
    pointer-events: none;
    background:
      linear-gradient(90deg, rgba(0, 0, 0, 0.32) 0, rgba(255, 255, 255, 0.28) 3%, rgba(0, 0, 0, 0.12) 6%, transparent 14%),
      linear-gradient(270deg, rgba(0, 0, 0, 0.12), transparent 6%);
  }
</style>
