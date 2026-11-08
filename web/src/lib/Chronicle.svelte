<script lang="ts">
  import type { Event } from './engine';

  interface Props { events: Event[]; oninnov: (id: number) => void; onhex: (hex: number) => void }
  let { events, oninnov, onhex }: Props = $props();
  let show = $state<'all' | 'splits'>('splits');
  const list = $derived(
    [...events].reverse().filter((e) => show === 'all' || e.kind !== 'innovation'),
  );
</script>

<div class="bar" role="radiogroup" aria-label="Which events">
  <label><input type="radio" bind:group={show} value="splits" /> Splits and interventions</label>
  <label><input type="radio" bind:group={show} value="all" /> Everything</label>
</div>
{#if list.length === 0}
  <p class="hint">Nothing has happened yet. Step the simulation forward to write the chronicle.</p>
{/if}
<ol>
  {#each list as e, i (i)}
    <li class={e.kind}>
      <span class="t">year {e.tick * 25}</span>
      {#if e.kind === 'innovation'}
        <button class="link" onclick={() => oninnov(e.innov)}>{e.text}</button> begins near
        <button class="link" onclick={() => onhex(e.hex)}>hex {e.hex}</button>
      {:else}
        <span>{e.text}</span>
      {/if}
    </li>
  {/each}
</ol>

<style>
  .bar { display: flex; gap: 1rem; font-size: var(--t-sm); margin-bottom: 0.6rem; }
  ol { list-style: none; padding: 0; margin: 0; }
  li { padding: 0.25rem 0; border-bottom: 1px solid var(--rule-faint); }
  li.split, li.extinct { font-family: var(--serif); font-size: var(--t-lg); }
  li.intervention { color: var(--isogloss); }
  .t { display: inline-block; width: 6.5rem; color: var(--ink-soft); font-size: var(--t-sm); font-family: var(--sans); font-variant-numeric: tabular-nums; }
  .hint { color: var(--ink-soft); }
  .link { background: none; border: none; padding: 0; cursor: pointer; }
  .link:hover { text-decoration: underline; }
</style>
