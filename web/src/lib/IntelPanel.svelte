<script lang="ts">
  import type { Engine, Lang } from './engine';
  import { langColor } from './palette';

  interface Intel { threshold: number; langs: Lang[]; m: number[][] }
  interface Props { engine: Engine; tick: number; onlang: (id: number) => void }
  let { engine, tick, onlang }: Props = $props();
  let data = $state<Intel | null>(null);
  let hover = $state<[number, number] | null>(null);

  $effect(() => {
    void tick;
    engine.query<Intel>('intel').then((d) => (data = d));
  });

  // Sequential blue ramp, light → dark (near zero recedes toward the surface).
  const lightRamp = ['#cde2fb', '#b7d3f6', '#9ec5f4', '#86b6ef', '#6da7ec', '#5598e7', '#3987e5', '#2a78d6', '#256abf', '#1c5cab', '#184f95', '#104281', '#0d366b'];
  const prefersDark = typeof matchMedia !== 'undefined' && matchMedia('(prefers-color-scheme: dark)').matches;
  const ramp = prefersDark ? [...lightRamp].reverse() : lightRamp;
  const step = (v: number) => Math.min(ramp.length - 1, Math.round((v / 1000) * (ramp.length - 1)));
  const cell = (v: number) => ramp[step(v)];
  // Dark text on the light half of the ramp, light text on the dark half.
  const ink = (v: number) => ((step(v) < 6) !== prefersDark ? 'var(--ink)' : 'var(--paper)');
</script>

{#if data && data.langs.length > 1}
  <p class="lede">
    How well speakers of each language understand each other, from shared core vocabulary and sound systems.
    Pairs at or above <strong>{(data.threshold / 10).toFixed(0)}%</strong> would count as dialects of one language; outlined cells cross that line.
  </p>
  <div class="wrap">
    <table aria-label="Mutual intelligibility between living languages, in percent">
      <thead>
        <tr>
          <td></td>
          {#each data.langs as l, j (l.id)}
            <th scope="col" class:hi={hover?.[1] === j}><span class="col"><span class="sw" style:background={langColor(l.id)}></span>{l.name}</span></th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each data.langs as a, i (a.id)}
          <tr>
            <th scope="row" class:hi={hover?.[0] === i}>
              <button class="link" onclick={() => onlang(a.id)}><span class="sw" style:background={langColor(a.id)}></span>{a.name}</button>
            </th>
            {#each data.m[i] as v, j (j)}
              <td
                style:background={i === j ? 'transparent' : cell(v)}
                style:color={i === j ? 'var(--ink-soft)' : ink(v)}
                class:over={i !== j && v >= data.threshold}
                onmouseenter={() => (hover = [i, j])}
                onmouseleave={() => (hover = null)}
                title={i === j ? a.name : `${a.name} ↔ ${data.langs[j].name}: ${(v / 10).toFixed(0)}%`}
              >{i === j ? '—' : (v / 10).toFixed(0)}</td>
            {/each}
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  <div class="legend" aria-hidden="true">
    <span>0%</span>
    <span class="bar" style:background={`linear-gradient(to right, ${ramp.join(',')})`}></span>
    <span>100%</span>
  </div>
  {#if hover && hover[0] !== hover[1]}
    <p class="readout">{data.langs[hover[0]].name} ↔ {data.langs[hover[1]].name}: <strong>{(data.m[hover[0]][hover[1]] / 10).toFixed(0)}%</strong> intelligible</p>
  {:else}
    <p class="readout hint">Hover a cell for the pair; click a language for its sketch.</p>
  {/if}
{:else if data}
  <p class="hint">Only one language is spoken so far. Once dialects drift far enough apart to split, this shows how well each pair understands the other.</p>
{/if}

<style>
  .lede { color: var(--ink-soft); max-width: 60ch; margin: 0 0 0.8rem; font-size: var(--t-sm); }
  .wrap { overflow: auto; max-height: calc(100vh - 16rem); }
  table { border-collapse: separate; border-spacing: 2px; font-size: var(--t-sm); }
  th { font-weight: 500; text-align: left; white-space: nowrap; padding: 0.1rem 0.4rem; color: var(--ink-soft); }
  th.hi { color: var(--ink); }
  thead th { vertical-align: bottom; height: 7rem; }
  .col { display: inline-flex; align-items: center; gap: 0.3rem; writing-mode: vertical-rl; transform: rotate(180deg); }
  td { width: 2.4rem; height: 2.1rem; text-align: center; font-variant-numeric: tabular-nums; border-radius: 3px; cursor: default; }
  td.over { box-shadow: inset 0 0 0 2px var(--ink); }
  td:hover { box-shadow: inset 0 0 0 2px var(--focus); }
  .sw { display: inline-block; width: 0.7rem; height: 0.7rem; border-radius: 2px; margin-right: 0.3rem; flex: none; }
  .col .sw { margin: 0 0 0.3rem; }
  .link { background: none; border: none; padding: 0; cursor: pointer; display: inline-flex; align-items: center; }
  .link:hover { text-decoration: underline; }
  .legend { display: flex; align-items: center; gap: 0.5rem; font-size: var(--t-xs); color: var(--ink-soft); margin-top: 0.6rem; }
  .bar { width: 10rem; height: 0.6rem; border-radius: 2px; }
  .readout { font-size: var(--t-sm); margin: 0.4rem 0 0; min-height: 1.4em; }
  .hint { color: var(--ink-soft); }
</style>
