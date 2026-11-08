<script lang="ts">
  import type { Corresp, Engine, Table, TableRow } from './engine';
  import { cogTint } from './palette';

  interface Props { engine: Engine; tick: number; onpick: (concept: string) => void }
  let { engine, tick, onpick }: Props = $props();
  let table = $state<Table | null>(null);
  let corr = $state<Corresp | null>(null);
  let filter = $state('');
  let mode = $state<'words' | 'corr'>('words');
  let all = $state(false);
  let onlyLoans = $state(false);

  $effect(() => {
    void tick;
    const core = !all;
    if (mode === 'words') engine.query<Table>('compare', { core }).then((t) => (table = t));
    else engine.query<Corresp>('corresp', { min: 2 }).then((c) => (corr = c));
  });
  // Correspondence sets grouped by proto-segment (the engine sends them grouped).
  const groups = $derived.by(() => {
    const out: { proto: string; sets: Corresp['sets'] }[] = [];
    for (const s of corr?.sets ?? []) {
      if (!out.length || out[out.length - 1].proto !== s.proto) out.push({ proto: s.proto, sets: [] });
      out[out.length - 1].sets.push(s);
    }
    return out;
  });
  const rows = $derived(
    table
      ? table.rows.filter((r) => r.concept.includes(filter.trim().toLowerCase()) && (!onlyLoans || r.loans.length > 0 || r.dbl.length > 0))
      : [],
  );
  const loanOf = (r: TableRow, i: number) => r.loans.find((x) => x[0] === i);
  const dblOf = (r: TableRow, i: number) => r.dbl.find((x) => x[0] === i)?.[1];
  const year = (t: number) => t * 25;
  function cellTitle(r: TableRow, i: number): string {
    const parts: string[] = [];
    const l = loanOf(r, i);
    if (l) parts.push(l[1] === 'loan' ? `borrowed from ${l[2]} in year ${year(l[3])}` : l[1] === 'learned' ? `learned loan from ${l[2]} in year ${year(l[3])}` : `from the ${l[2]} substrate, year ${year(l[3])}`);
    const d = dblOf(r, i);
    if (d) parts.push(`doublet: the same root also gives the word for ‘${d}’`);
    return parts.join('; ');
  }
</script>

<div class="bar">
  <div class="seg" role="group" aria-label="Table">
    <button aria-pressed={mode === 'words'} onclick={() => (mode = 'words')}>Cognates</button>
    <button aria-pressed={mode === 'corr'} onclick={() => (mode = 'corr')}>Sound correspondences</button>
  </div>
  {#if mode === 'words'}
    <label for="tfilter">Find concept</label>
    <input id="tfilter" bind:value={filter} placeholder="e.g. water" />
    <label class="chk"><input type="checkbox" bind:checked={all} /> all concepts</label>
    <label class="chk"><input type="checkbox" bind:checked={onlyLoans} /> only loans and doublets</label>
    <span class="note">
      Shaded cells use a new root; the same shade in a row marks cognates.
      <span class="mk loan">L</span> loan from another language, <span class="mk sub">S</span> substrate word, <span class="mk loan">B</span> bookish loan from a classical language,
      <span class="mk dbl">D</span> doublet (the root came in twice). Hover a mark for its source.
    </span>
  {:else}
    <span class="note">Each row is a proto-sound and what it became in every language, counted over core words all of them inherited. Recurring sets are regular sound laws; a sound with several rows split by context.</span>
  {/if}
</div>
{#if mode === 'corr'}
  {#if corr && groups.length}
    <div class="wrap">
      <table>
        <thead>
          <tr><th scope="col" class="proto">{corr.proto}</th>{#each corr.langs as l, i (i)}<th scope="col">{l}</th>{/each}<th scope="col">Sets</th><th scope="col">e.g.</th></tr>
        </thead>
        <tbody>
          {#each groups as g (g.proto)}
            {#each g.sets as c, k (k)}
              <tr class:first={k === 0}>
                <th scope="row" class="ipa">{k === 0 ? '*' + c.proto : ''}</th>
                {#each c.reflexes as r, i (i)}<td class="ipa" class:same={r === c.proto} class:lost={r === '∅'}>{r}</td>{/each}
                <td class="num">{c.count}</td>
                <td class="ex">{c.examples.join(', ')}</td>
              </tr>
            {/each}
          {/each}
        </tbody>
      </table>
    </div>
  {:else if corr}
    <p class="note">Not enough words are still shared by every language to find regular correspondences.</p>
  {/if}
{:else if table}
  <div class="wrap">
    <table>
      <thead>
        <tr><th scope="col">Concept</th>{#each table.langs as l, i (i)}<th scope="col" class:proto={i < table.protos}>{l}</th>{/each}</tr>
      </thead>
      <tbody>
        {#each rows as r (r.id)}
          <tr>
            <th scope="row"><button class="link" onclick={() => onpick(r.concept)}>{r.concept}</button></th>
            {#each r.forms as f, i (i)}
              {@const l = loanOf(r, i)}
              {@const d = dblOf(r, i)}
              {#if r.cog[i] < 0}
                <td class="none" title="no word: the thing is not known here">—</td>
              {:else}
                <td class="ipa" style:background={cogTint(r.cog[i])} title={cellTitle(r, i) || undefined}>
                  {i < table.protos ? '*' : ''}{f}{#if l}<sup class="mk {l[1] === 'substrate' ? 'sub' : 'loan'}">{l[1] === 'loan' ? 'L' : l[1] === 'learned' ? 'B' : 'S'}</sup>{/if}{#if d}<sup class="mk dbl">D</sup>{/if}
                </td>
              {/if}
            {/each}
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<style>
  .bar { display: flex; gap: 0.6rem; align-items: center; flex-wrap: wrap; margin-bottom: 0.6rem; }
  label { font-size: var(--t-sm); color: var(--ink-soft); }
  .note { font-size: var(--t-sm); color: var(--ink-soft); }
  .wrap { overflow: auto; max-height: calc(100vh - 14rem); border-top: 1px solid var(--rule); }
  table { border-collapse: collapse; font-size: var(--t-sm); }
  th, td { padding: 0.2rem 0.6rem; text-align: left; border-bottom: 1px solid var(--rule-faint); white-space: nowrap; }
  thead th { position: sticky; top: 0; background: var(--paper); font-family: var(--serif); font-weight: 700; font-size: var(--t-md); z-index: 1; }
  thead th.proto { font-style: italic; font-weight: 400; }
  tbody th { position: sticky; left: 0; background: var(--paper); font-weight: 500; }
  td.ipa { font-size: var(--t-md); }
  .seg { display: inline-flex; }
  .seg button { border-radius: 0; }
  .seg button:first-child { border-radius: 3px 0 0 3px; }
  .seg button:last-child { border-radius: 0 3px 3px 0; margin-left: -1px; }
  .seg button[aria-pressed='true'] { background: var(--ink); color: var(--paper); border-color: var(--ink); }
  td.same { color: var(--ink-soft); }
  td.lost { color: var(--isogloss); }
  td.num { font-variant-numeric: tabular-nums; text-align: right; color: var(--ink-soft); }
  td.ex { color: var(--ink-soft); font-size: var(--t-xs); }
  tr.first > * { border-top: 1px solid var(--rule); }
  td.none { color: var(--ink-faint, var(--ink-soft)); text-align: center; }
  .chk { display: inline-flex; gap: 0.3rem; align-items: center; }
  .mk { font-family: var(--sans); font-size: 0.62rem; font-weight: 700; letter-spacing: 0.02em; margin-left: 0.15em; padding: 0 0.2em; border-radius: 2px; border: 1px solid currentColor; }
  sup.mk { vertical-align: super; line-height: 1; }
  .mk.loan { color: var(--isogloss); }
  .mk.sub { color: var(--ink-soft); }
  .mk.dbl { color: var(--ink); }
  .link { background: none; border: none; padding: 0; cursor: pointer; }
  .link:hover { text-decoration: underline; }
</style>
