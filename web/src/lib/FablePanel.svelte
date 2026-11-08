<script lang="ts">
  import type { Engine, Fable } from './engine';
  import { langColor } from './palette';
  import Gloss from './Gloss.svelte';

  interface Props { engine: Engine; tick: number; onlang: (id: number) => void }
  let { engine, tick, onlang }: Props = $props();
  let fable = $state<Fable | null>(null);
  let line = $state(0);

  $effect(() => {
    void tick;
    engine.query<Fable>('fable').then((f) => (fable = f));
  });
</script>

{#if fable}
  <div class="bar" role="radiogroup" aria-label="Sentence">
    {#each fable.english as e, i (i)}
      <button role="radio" aria-checked={line === i} class:on={line === i} onclick={() => (line = i)}>{i + 1}</button>
    {/each}
    <span class="en">‘{fable.english[line]}’</span>
  </div>
  <p class="hint">The same sentence, built from each language’s own words, endings and word order. The second line glosses every piece.</p>
  {#each fable.langs as l (l.id)}
    <article>
      <h3>
        {#if l.id >= 0}
          <span class="sw" style:background={langColor(l.id)}></span>
          <button class="link" onclick={() => onlang(l.id)}>{l.name}</button>
        {:else}<span class="proto">{l.name}</span>{/if}
        <span class="ord">{l.order}</span>
      </h3>
      <div class="ilg">
        {#each l.lines[line] as w, i (i)}
          <span class="w"><span class="f ipa">{w.f}</span><span class="g"><Gloss text={w.g} /></span></span>
        {/each}
      </div>
    </article>
  {/each}
{/if}

<style>
  .bar { display: flex; flex-wrap: wrap; gap: 0.3rem; align-items: center; margin-bottom: 0.3rem; }
  .bar button { min-width: 2rem; }
  .bar button.on { background: var(--ink); color: var(--paper); border-color: var(--ink); }
  .en { font-family: var(--serif); font-style: italic; font-size: var(--t-lg); margin-left: 0.4rem; }
  .hint { color: var(--ink-soft); font-size: var(--t-sm); margin: 0.2rem 0 0.8rem; }
  article { padding: 0.5rem 0; border-top: 1px solid var(--rule-faint); }
  h3 { margin: 0 0 0.3rem; font-family: var(--serif); font-weight: 400; font-size: var(--t-lg); display: flex; align-items: center; gap: 0.45rem; }
  .proto { font-style: italic; }
  .ord { font-family: var(--sans); font-size: var(--t-xs); color: var(--ink-soft); letter-spacing: 0.05em; }
  .sw { width: 0.8rem; height: 0.8rem; border-radius: 2px; }
  .ilg { display: flex; flex-wrap: wrap; gap: 0.2rem 1.1rem; }
  .w { display: inline-flex; flex-direction: column; }
  .f { font-size: var(--t-lg); }
  .g { font-size: var(--t-xs); color: var(--ink-soft); letter-spacing: 0.02em; }
  .link { background: none; border: none; padding: 0; cursor: pointer; font: inherit; }
  .link:hover { text-decoration: underline; }
</style>
