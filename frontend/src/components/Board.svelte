<script>
  import Login from './Login.svelte';
  import Books from './Books.svelte';

  // The four columns are the same DOM elements on the login page and on the board.
  // `phase` drives their geometry:
  //   'login'    columns are a strip on the left, form on the right
  //   'entering' columns expand to full width, form fades out, labels fade in
  //   'ready'    the interactive board
  //   'leaving'  reverse of 'entering'
  let {
    phase = 'ready',
    duration = 1000, // ms for the strip <-> full-width transition
    intro = false, // play the one-time rise animation (first paint of the login page)
    formKey = 0, // change to remount (reset) the login form
    onlogin,
  } = $props();

  // Edit this list to rename or change the areas. Each color comes from the brand palette.
  const areas = [
    { id: 'books', label: 'Books', color: 'var(--rosy-taupe)' },
    { id: 'exercise', label: 'Exercise', color: 'var(--old-rose)' },
    { id: 'learning', label: 'Learning', color: 'var(--amethyst-smoke)' },
    { id: 'mindfulness', label: 'Mindfulness', color: 'var(--dusty-lavender)' },
  ];

  // Must match --duration in the CSS below.
  const DURATION_MS = 700;

  let panelAt = $state(null); // index of the column the panel sits after
  let open = $state(false); // whether the panel is expanded
  let busy = false;
  let columnEls = [];
  let stageEl;

  const collapsed = $derived(phase === 'login' || phase === 'leaving');

  // Leaving the board closes any open panel so it folds away with the columns.
  $effect(() => {
    if (phase !== 'ready') {
      open = false;
      busy = false;
    }
  });

  export function focusFirstColumn() {
    columnEls[0]?.focus();
  }

  export function focusLogin() {
    stageEl?.querySelector('input')?.focus();
  }

  const reducedMotion = () =>
    window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
  const nextFrame = () =>
    new Promise((resolve) =>
      requestAnimationFrame(() => requestAnimationFrame(resolve)),
    );

  async function toggle(i) {
    if (busy || phase !== 'ready') return;
    busy = true;
    const delay = reducedMotion() ? 0 : DURATION_MS;

    if (open && panelAt === i) {
      // Clicked the open column: revert to the original state.
      open = false;
      await wait(delay);
    } else {
      if (open) {
        // Another column is open: close it first, then open the new one.
        open = false;
        await wait(delay);
      }
      panelAt = i;
      await nextFrame(); // let the closed panel render so the transition runs
      open = true;
      await wait(delay);
    }
    busy = false;
  }
</script>

<main
  class="shell"
  class:collapsed
  class:leaving={phase === 'leaving'}
  class:intro
  style="--duration: {DURATION_MS}ms; --enter: {duration}ms"
>
  <!-- Login form: sits on the light surface to the right of the strip. -->
  <div
    class="stage"
    bind:this={stageEl}
    inert={phase !== 'login'}
    aria-hidden={phase !== 'login' ? 'true' : undefined}
  >
    {#key formKey}
      <Login onsuccess={onlogin} />
    {/key}
  </div>

  <div
    class="columns"
    class:open
    inert={phase !== 'ready'}
  >
    {#each areas as area, i (area.id)}
      <button
        type="button"
        class="column"
        class:active={open && panelAt === i}
        style="background-color: {area.color}; --i: {i}"
        aria-label={area.label}
        aria-expanded={open && panelAt === i}
        aria-controls="panel"
        bind:this={columnEls[i]}
        onclick={() => toggle(i)}
      >
        <span class="label label-wide" aria-hidden="true">{area.label}</span>
        <span class="label label-tall" aria-hidden="true">{area.label}</span>
      </button>

      {#if panelAt === i}
        <section id="panel" class="panel" aria-label={areas[i].label}>
          {#if areas[i].id === 'books'}
            <div class="panel-content scroll">
              <Books />
            </div>
          {:else}
            <div class="panel-content">
              <h1>{areas[i].label}</h1>
              <p>Nothing here yet.</p>
            </div>
          {/if}
        </section>
      {/if}
    {/each}
  </div>
</main>

<style>
  .shell {
    --collapsed: 4.5rem;
    --duration: 700ms;
    --enter: 1000ms;
    --ease: cubic-bezier(0.65, 0, 0.35, 1);
    /* Geometry of the column group: full page by default (the board). */
    --cols-w: 100%;
    --cols-h: 100%;
    position: relative;
    width: 100%;
    height: 100%;
    overflow: hidden;
    background:
      radial-gradient(circle at 85% 10%, rgba(120, 61, 120, 0.08), transparent 45%),
      repeating-linear-gradient(135deg, rgba(59, 59, 88, 0.035) 0 1px, transparent 1px 14px),
      #f6f5f8;
  }

  /* Login layout: the strip is 1/(1+1.1) of the page, like the old two-column grid. */
  .shell.collapsed {
    --cols-w: 47.619%;
  }

  .columns {
    position: absolute;
    top: 0;
    left: 0;
    z-index: 1;
    display: flex;
    width: var(--cols-w);
    height: var(--cols-h);
    overflow: hidden;
    transition:
      width var(--enter) var(--ease),
      height var(--enter) var(--ease);
  }

  .column {
    flex: 0 0 25%;
    min-width: 0;
    height: 100%;
    border: 0;
    padding: 0;
    position: relative;
    color: #ffffff;
    font: inherit;
    cursor: pointer;
    overflow: hidden;
    transition:
      flex-basis var(--duration) var(--ease),
      filter 200ms ease;
  }

  .column:hover {
    filter: brightness(0.88);
  }

  .column:focus-visible {
    /* Same cue as hover, but darker, so keyboard focus is distinguishable. */
    outline: none;
    filter: brightness(0.68);
  }

  /* One-time rise when the login page first paints. */
  .intro .column {
    transform-origin: bottom;
    animation: rise 800ms var(--ease) backwards;
    animation-delay: calc(var(--i) * 90ms);
  }

  @keyframes rise {
    from { transform: scaleY(0); }
    to { transform: scaleY(1); }
  }

  @keyframes grow {
    from { transform: scaleX(0); }
    to { transform: scaleX(1); }
  }

  /* Marks the open column with a shape, not just color. */
  .column.active::after {
    content: '';
    position: absolute;
    top: 0;
    right: 0;
    bottom: 0;
    width: 6px;
    background: #ffffff;
  }

  .open .column {
    flex-basis: var(--collapsed);
  }

  .label {
    position: absolute;
    top: 50%;
    left: 50%;
    white-space: nowrap;
    font-weight: 600;
    opacity: 0;
    transition: opacity var(--duration) var(--ease);
  }

  .label-wide {
    font-size: 1.75rem;
    transform: translate(-50%, -50%);
  }

  .label-tall {
    font-size: 1.1rem;
    writing-mode: vertical-rl;
    transform: translate(-50%, -50%) rotate(180deg);
  }

  /* Labels fade in once the columns are mostly expanded, and out right away when collapsing. */
  .shell:not(.collapsed) .label-wide {
    opacity: 1;
    transition-delay: calc(var(--enter) * 0.55);
  }

  .shell:not(.collapsed) .open .label-wide {
    opacity: 0;
    transition-delay: 0s;
  }

  .shell:not(.collapsed) .open .label-tall {
    opacity: 1;
    transition-delay: 0s;
  }

  .stage {
    position: absolute;
    top: 0;
    bottom: 0;
    left: 47.619%;
    right: 0;
    overflow-y: auto;
    display: grid;
    align-items: center;
    opacity: 1;
    visibility: visible;
    transition:
      opacity 350ms ease var(--stage-delay, 0s),
      visibility 0s linear var(--stage-delay, 0s);
  }

  /* On sign-out the form returns as the columns shrink back. */
  .leaving .stage {
    --stage-delay: calc(var(--enter) * 0.6);
  }

  /* On sign-in the form fades out immediately while the columns expand over it. */
  .shell:not(.collapsed) .stage {
    opacity: 0;
    visibility: hidden;
    transition:
      opacity 350ms ease,
      visibility 0s linear 350ms;
  }

  .intro .stage {
    animation: fade 700ms 350ms ease backwards;
  }

  @keyframes fade {
    from { opacity: 0; transform: translateY(14px); }
    to { opacity: 1; transform: none; }
  }

  .panel {
    flex: 0 0 0;
    min-width: 0;
    overflow: hidden;
    background: #f6f5f8;
    visibility: hidden;
    transition:
      flex-basis var(--duration) var(--ease),
      visibility 0s linear var(--duration);
  }

  .open .panel {
    flex-basis: calc(100% - 4 * var(--collapsed));
    visibility: visible;
    transition:
      flex-basis var(--duration) var(--ease),
      visibility 0s;
  }

  /* Fixed width so content doesn't reflow while the panel grows. */
  .panel-content {
    width: calc(100vw - 4 * var(--collapsed));
    padding: 3rem;
    box-sizing: border-box;
  }

  /* Books: the content scrolls inside the fixed-width panel. */
  .panel-content.scroll {
    height: 100%;
    overflow-y: auto;
    padding: 2rem 2.5rem 4.5rem;
  }

  .panel-content h1 {
    margin: 0 0 0.5rem;
    color: var(--space-indigo);
  }

  .panel-content p {
    margin: 0;
    color: var(--dusty-lavender);
  }

  @media (max-width: 720px) {
    /* Strip becomes a thin bar across the top of the page. */
    .shell.collapsed {
      --cols-w: 100%;
      --cols-h: 0.75rem;
    }

    .stage {
      top: 0.75rem;
      left: 0;
    }

    .intro .column {
      transform-origin: left;
      animation-name: grow;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .columns,
    .column,
    .label,
    .panel,
    .open .panel,
    .stage,
    .shell:not(.collapsed) .stage {
      transition-duration: 0s, 0s;
      transition-delay: 0s;
    }

    .intro .column,
    .intro .stage {
      animation: none;
    }
  }
</style>
