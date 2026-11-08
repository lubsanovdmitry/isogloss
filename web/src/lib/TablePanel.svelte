<script lang="ts">
  import type { Engine, Table } from './engine';
  import { cogTint } from './palette';

  interface Props { engine: Engine; tick: number; onpick: (concept: string) => void }
  let { engine, tick, onpick }: Props = $props();
  let table = $state<Table | null>(null);
  let filter = $state('');

  $effect(() => {
    void tick;
    engine.query<Table>('compare', { core: true }).then((t) => (table = t));
  });
  const rows = $derived(table ? table.rows.filter((r) => r.concept.includes(filter.trim().toLowerCase())) : []);
</script>

<div class="bar">
  <label for="tfilter">Find concept</label>
  <input id="tfilter" bind:value={filter} placeholder="e.g. water" />
  <span class="note">Shaded cells use a new root; the same shade in a row marks cognates.</span>
</div>
{#if table}
  <div class="wrap">
    <table>
      <thead>
        <tr><th scope="col">Concept</th>{#each table.langs as l, i (i)}<th scope="col" class:proto={i === 0}>{l}</th>{/each}</tr>
      </thead>
      <tbody>
        {#each rows as r (r.id)}
          <tr>
            <th scope="row"><button class="link" onclick={() => onpick(r.concept)}>{r.concept}</button></th>
            {#each r.forms as f, i (i)}
              <td class="ipa" style:background={cogTint(r.cog[i])}>{i === 0 ? '*' : ''}{f}</td>
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
  .link { background: none; border: none; padding: 0; cursor: pointer; }
  .link:hover { text-decoration: underline; }
</style>
