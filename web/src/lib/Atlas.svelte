<script lang="ts">
  import type { MapLabel, World } from './engine';
  import { langColor, biomeColor } from './palette';

  interface Props {
    world: World;
    langOf: Int32Array; // per hex, -1 unsettled
    selected: number;
    isogloss: Set<number> | null; // hexes that adopted the chosen innovation
    level?: number[] | null; // live wave level per hex (per-mille), if still spreading
    substrates?: boolean; // outline the areas of the languages spoken before the family
    polityOf?: Int32Array | null; // per hex; shown instead of languages when set
    capitals?: number[];
    routes?: number[][];
    labels?: MapLabel[]; // place names to print
    colorOf?: ((hex: number) => string | null) | null; // overrides the fill of settled hexes (the word map)
    onselect: (hex: number) => void;
  }
  let { world, langOf, selected, isogloss, level = null, substrates = false, polityOf = null, capitals = [], routes = [], labels = [], colorOf = null, onselect }: Props = $props();

  const R = 14; // hex radius
  const W = Math.sqrt(3) * R;
  const cx = (col: number, row: number) => W * (col + 0.5 * (row & 1)) + W / 2 + 4;
  const cy = (row: number) => 1.5 * R * row + R + 4;

  // Pointy-top vertices, clockwise from the upper-right corner.
  const ang = [-30, 30, 90, 150, 210, 270].map((a) => (a * Math.PI) / 180);
  const vx = (x: number, i: number) => x + R * Math.cos(ang[i]);
  const vy = (y: number, i: number) => y + R * Math.sin(ang[i]);
  // Edge (pair of vertices) facing each neighbour direction, matching the
  // engine's odd-r order: E, NE, NW, W, SW, SE.
  const edgeVerts = [[0, 1], [5, 0], [4, 5], [3, 4], [2, 3], [1, 2]];
  const offs = [
    [[1, 0], [0, -1], [-1, -1], [-1, 0], [-1, 1], [0, 1]],
    [[1, 0], [1, -1], [0, -1], [-1, 0], [0, 1], [1, 1]],
  ];

  const width = $derived(W * world.w + W / 2 + 8);
  const height = $derived(1.5 * R * (world.h - 1) + 2 * R + 8);

  function poly(col: number, row: number) {
    const x = cx(col, row), y = cy(row);
    return ang.map((_, i) => `${vx(x, i).toFixed(1)},${vy(y, i).toFixed(1)}`).join(' ');
  }
  const polys = $derived(world.hexes.map((h) => poly(h.col, h.row)));

  function nbr(id: number, d: number) {
    const h = world.hexes[id];
    const [dc, dr] = offs[h.row & 1][d];
    const c = h.col + dc, r = h.row + dr;
    if (c < 0 || r < 0 || c >= world.w || r >= world.h) return -1;
    return r * world.w + c;
  }

  function edgePath(filter: (a: number, b: number) => boolean) {
    let d = '';
    for (const h of world.hexes) {
      for (let k = 0; k < 6; k++) {
        const n = nbr(h.id, k);
        if (n < 0 || n < h.id) continue;
        if (!filter(h.id, n)) continue;
        const [i, j] = edgeVerts[k];
        const x = cx(h.col, h.row), y = cy(h.row);
        d += `M${vx(x, i).toFixed(1)},${vy(y, i).toFixed(1)}L${vx(x, j).toFixed(1)},${vy(y, j).toFixed(1)}`;
      }
    }
    return d;
  }

  const borders = $derived(
    polityOf
      ? edgePath((a, b) => (polityOf![a] >= 0 || polityOf![b] >= 0) && polityOf![a] !== polityOf![b] && langOf[a] >= 0 && langOf[b] >= 0)
      : edgePath((a, b) => langOf[a] >= 0 && langOf[b] >= 0 && langOf[a] !== langOf[b]),
  );
  const routePath = $derived(
    routes
      .map((p) => p.map((h, i) => `${i ? 'L' : 'M'}${cx(world.hexes[h].col, world.hexes[h].row).toFixed(1)},${cy(world.hexes[h].row).toFixed(1)}`).join(''))
      .join(''),
  );
  const coast = $derived(edgePath((a, b) => (world.hexes[a].biome === 'sea') !== (world.hexes[b].biome === 'sea')));
  const isoPath = $derived(
    isogloss ? edgePath((a, b) => langOf[a] >= 0 && langOf[b] >= 0 && isogloss!.has(a) !== isogloss!.has(b)) : '',
  );
  const subPath = $derived(
    substrates ? edgePath((a, b) => world.hexes[a].sub !== world.hexes[b].sub && world.hexes[a].biome !== 'sea' && world.hexes[b].biome !== 'sea') : '',
  );
  const rivers = $derived(
    world.hexes
      .filter((h) => h.river && h.down >= 0)
      .map((h) => {
        const d = world.hexes[h.down];
        return `M${cx(h.col, h.row).toFixed(1)},${cy(h.row).toFixed(1)}L${cx(d.col, d.row).toFixed(1)},${cy(d.row).toFixed(1)}`;
      })
      .join(''),
  );

  function fill(id: number) {
    const h = world.hexes[id];
    if (h.biome === 'sea') return 'var(--sea)';
    if (colorOf && langOf[id] >= 0) return colorOf(id) ?? 'color-mix(in srgb, var(--ink) 10%, var(--paper))';
    if (polityOf && langOf[id] >= 0) return polityOf[id] >= 0 ? langColor(polityOf[id] + 3) : 'color-mix(in srgb, var(--ink) 14%, var(--paper))';
    if (langOf[id] >= 0) return langColor(langOf[id]);
    return biomeColor(h.biome);
  }

  function key(e: KeyboardEvent) {
    if (selected < 0) return;
    const dirs: Record<string, number> = { ArrowRight: 0, ArrowLeft: 3, ArrowUp: 1, ArrowDown: 5 };
    if (e.key in dirs) {
      const n = nbr(selected, dirs[e.key]);
      if (n >= 0) onselect(n);
      e.preventDefault();
    }
  }
</script>

<!-- A custom keyboard widget: arrow keys move the selected hex. -->
<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<svg
  viewBox="0 0 {width} {height}"
  role="application"
  aria-label="Map of the continent; each settled hex is coloured by its language. Arrow keys move the selection."
  tabindex="0"
  onkeydown={key}
>
  <defs>
    <pattern id="hatch" width="5" height="5" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
      <rect width="5" height="5" fill="var(--isogloss)" opacity="0.35" />
      <line x1="0" y1="0" x2="0" y2="5" stroke="var(--isogloss)" stroke-width="2" />
    </pattern>
  </defs>
  {#each world.hexes as h (h.id)}
    <polygon
      points={polys[h.id]}
      fill={fill(h.id)}
      class:land={h.biome !== 'sea'}
      class:mountain={h.biome === 'mountain'}
      onclick={() => h.biome !== 'sea' && onselect(h.id)}
      role="presentation"
    />
  {/each}
  {#if level}
    {#each world.hexes as h (h.id)}
      {#if level[h.id] > 0 && langOf[h.id] >= 0}
        <polygon points={polys[h.id]} class="wave" class:done={isogloss?.has(h.id)} style:opacity={isogloss?.has(h.id) ? 0.35 : 0.15 + 0.6 * (level[h.id] / 1000)} />
      {/if}
    {/each}
  {/if}
  <path d={rivers} class="river" />
  <path d={coast} class="coast" />
  <path d={borders} class="border" />
  {#if routePath}<path d={routePath} class="route" />{/if}
  {#each capitals as h (h)}
    {@const hx = world.hexes[h]}
    <circle cx={cx(hx.col, hx.row)} cy={cy(hx.row)} r="3.6" class="capital" />
  {/each}
  {#if isoPath}<path d={isoPath} class="iso" />{/if}
  {#if subPath}
    <path d={subPath} class="subarea" />
    {#each world.substrates as sb (sb.seed)}
      {@const h = world.hexes[sb.seed]}
      <text x={cx(h.col, h.row)} y={cy(h.row)} class="sublabel">{sb.name}</text>
    {/each}
  {/if}
  {#each labels as lb (lb.hex)}
    {@const hx = world.hexes[lb.hex]}
    <text x={cx(hx.col, hx.row)} y={cy(hx.row) - (lb.capital ? 7 : 0)} class="place" class:cap={lb.capital}>
      <title>{lb.name}</title>{lb.short}
    </text>
  {/each}
  {#if selected >= 0}
    <polygon points={polys[selected]} class="sel" />
  {/if}
</svg>

<style>
  .route { fill: none; stroke: var(--ink); stroke-width: 1.4; stroke-dasharray: 1 3; stroke-linecap: round; opacity: 0.7; pointer-events: none; }
  .capital { fill: var(--paper); stroke: var(--ink); stroke-width: 1.6; pointer-events: none; }
  .subarea { fill: none; stroke: var(--ink); stroke-width: 1.6; stroke-dasharray: 4 3; opacity: 0.75; pointer-events: none; }
  .sublabel { font-family: var(--serif); font-style: italic; font-size: 13px; text-anchor: middle; dominant-baseline: middle; fill: var(--ink);
    paint-order: stroke; stroke: var(--paper); stroke-width: 3px; pointer-events: none; }
  .place { font-family: var(--serif); font-size: 11px; text-anchor: middle; dominant-baseline: middle; fill: var(--ink);
    paint-order: stroke; stroke: var(--paper); stroke-width: 2.6px; stroke-linejoin: round; pointer-events: none; }
  .place.cap { font-weight: 700; font-size: 12px; dominant-baseline: auto; }
  svg { width: 100%; height: auto; display: block; max-height: calc(100vh - 9rem); }
  svg:focus:not(:focus-visible) { outline: none; }
  polygon { stroke: none; }
  polygon.land { cursor: pointer; stroke: var(--paper); stroke-width: 0.6; }
  polygon.mountain { stroke-dasharray: 1.5 1.5; stroke: var(--rule); }
  .river { stroke: var(--river); stroke-width: 1.8; fill: none; stroke-linecap: round; opacity: 0.85; pointer-events: none; }
  .coast { stroke: var(--rule); stroke-width: 1; fill: none; pointer-events: none; }
  .border { stroke: var(--ink); stroke-width: 1.6; fill: none; stroke-linecap: round; pointer-events: none; }
  .iso { stroke: var(--isogloss); stroke-width: 3; fill: none; stroke-linecap: round; pointer-events: none; }
  .wave { fill: var(--isogloss); pointer-events: none; }
  .wave.done { fill: url(#hatch); }
  .sel { fill: none; stroke: var(--ink); stroke-width: 2.5; pointer-events: none; }
</style>
