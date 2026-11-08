<script lang="ts">
  // Adoption of one innovation over time: communities that applied it, and
  // communities it reached at any level.
  interface Props { curve: [number, number][]; born: number }
  let { curve, born }: Props = $props();
  const YEARS = 25;
  const W = 320, H = 96, L = 30, R = 64, T = 8, B = 20;
  let hover = $state(-1);

  const n = $derived(curve.length);
  const ymax = $derived(Math.max(1, ...curve.map((c) => c[1])));
  const x = (i: number) => L + (n <= 1 ? 0 : (i / (n - 1)) * (W - L - R));
  const y = (v: number) => T + (1 - v / ymax) * (H - T - B);
  const line = (k: 0 | 1) => curve.map((c, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(c[k]).toFixed(1)}`).join('');

  function move(e: PointerEvent) {
    const r = (e.currentTarget as SVGElement).getBoundingClientRect();
    const px = ((e.clientX - r.left) / r.width) * W;
    hover = Math.max(0, Math.min(n - 1, Math.round(((px - L) / (W - L - R)) * (n - 1))));
  }
</script>

{#if n > 1}
  <figure>
    <figcaption>Spread over time</figcaption>
    <svg viewBox="0 0 {W} {H}" role="img" aria-label="Communities reached and adopting, by year"
      onpointermove={move} onpointerleave={() => (hover = -1)}>
      <line class="axis" x1={L} x2={W - R} y1={y(0)} y2={y(0)} />
      <text class="tick" x={L - 4} y={y(ymax) + 3} text-anchor="end">{ymax}</text>
      <text class="tick" x={L - 4} y={y(0) + 3} text-anchor="end">0</text>
      <text class="tick" x={x(0)} y={H - 6} text-anchor="start">{born * YEARS}</text>
      <text class="tick" x={x(n - 1)} y={H - 6} text-anchor="end">{(born + n - 1) * YEARS}</text>
      <path d={line(1)} class="reached" />
      <path d={line(0)} class="applied" />
      <text class="lab" x={x(n - 1) + 5} y={y(curve[n - 1][1]) + 3}>reached</text>
      <text class="lab" x={x(n - 1) + 5} y={Math.max(y(curve[n - 1][0]) + 3, y(curve[n - 1][1]) + 14)}>adopted</text>
      {#if hover >= 0}
        <line class="cross" x1={x(hover)} x2={x(hover)} y1={T} y2={y(0)} />
        <circle cx={x(hover)} cy={y(curve[hover][1])} r="3.5" class="dot-r" />
        <circle cx={x(hover)} cy={y(curve[hover][0])} r="4" class="dot-a" />
      {/if}
      <rect x={L} y={0} width={W - L - R} height={H} fill="transparent" />
    </svg>
    <p class="read">
      {#if hover >= 0}
        Year {(born + hover) * YEARS}: adopted in <strong>{curve[hover][0]}</strong>, reached <strong>{curve[hover][1]}</strong> communities
      {:else}
        <span class="key a"></span>adopted <span class="key r"></span>reached (any level)
      {/if}
    </p>
  </figure>
{/if}

<style>
  figure { margin: 0; width: 100%; max-width: 26rem; }
  figcaption { font-size: var(--t-xs); color: var(--ink-soft); text-transform: uppercase; letter-spacing: 0.06em; }
  svg { width: 100%; height: auto; display: block; touch-action: none; }
  .axis { stroke: var(--rule); stroke-width: 1; }
  .tick, .lab { font-size: 9px; fill: var(--ink-soft); font-family: var(--sans); }
  .applied { fill: none; stroke: var(--isogloss); stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
  .reached { fill: none; stroke: var(--isogloss); stroke-width: 2; stroke-dasharray: 3 3; opacity: 0.55; }
  .cross { stroke: var(--rule); stroke-width: 1; }
  .dot-a { fill: var(--isogloss); stroke: var(--paper); stroke-width: 2; }
  .dot-r { fill: var(--paper); stroke: var(--isogloss); stroke-width: 2; }
  .read { margin: 0; font-size: var(--t-xs); color: var(--ink-soft); min-height: 1.3em; }
  .key { display: inline-block; width: 1rem; height: 0; border-top: 2px solid var(--isogloss); vertical-align: middle; margin: 0 0.25rem 0 0.4rem; }
  .key.r { border-top-style: dashed; opacity: 0.6; }
</style>
