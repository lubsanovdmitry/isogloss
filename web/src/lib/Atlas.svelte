<script lang="ts">
  import type { World } from './engine';
  import { langColor, biomeColor } from './palette';

  interface Props {
    world: World;
    langOf: Int32Array; // per hex, -1 unsettled
    selected: number;
    isogloss: Set<number> | null; // hexes that adopted the chosen innovation
    onselect: (hex: number) => void;
  }
  let { world, langOf, selected, isogloss, onselect }: Props = $props();

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

  const borders = $derived(edgePath((a, b) => langOf[a] >= 0 && langOf[b] >= 0 && langOf[a] !== langOf[b]));
  const coast = $derived(edgePath((a, b) => (world.hexes[a].biome === 'sea') !== (world.hexes[b].biome === 'sea')));
  const isoPath = $derived(
    isogloss ? edgePath((a, b) => langOf[a] >= 0 && langOf[b] >= 0 && isogloss!.has(a) !== isogloss!.has(b)) : '',
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
  <path d={rivers} class="river" />
  <path d={coast} class="coast" />
  <path d={borders} class="border" />
  {#if isoPath}<path d={isoPath} class="iso" />{/if}
  {#if selected >= 0}
    <polygon points={polys[selected]} class="sel" />
  {/if}
</svg>

<style>
  svg { width: 100%; height: auto; display: block; max-height: calc(100vh - 9rem); }
  svg:focus:not(:focus-visible) { outline: none; }
  polygon { stroke: none; }
  polygon.land { cursor: pointer; stroke: var(--paper); stroke-width: 0.6; }
  polygon.mountain { stroke-dasharray: 1.5 1.5; stroke: var(--rule); }
  .river { stroke: var(--river); stroke-width: 1.8; fill: none; stroke-linecap: round; opacity: 0.85; }
  .coast { stroke: var(--rule); stroke-width: 1; fill: none; }
  .border { stroke: var(--ink); stroke-width: 1.6; fill: none; stroke-linecap: round; }
  .iso { stroke: var(--isogloss); stroke-width: 3; fill: none; stroke-linecap: round; }
  .sel { fill: none; stroke: var(--ink); stroke-width: 2.5; pointer-events: none; }
</style>
