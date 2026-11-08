<script lang="ts">
  import type { Concept, Engine, Etym } from './engine';

  interface Props { engine: Engine; hex: number; tick: number; concepts: Concept[]; concept: string; onconcept: (c: string) => void }
  let { engine, hex, tick, concepts, concept, onconcept }: Props = $props();
  let etym = $state<Etym | null>(null);
  let error = $state('');

  $effect(() => {
    const h = hex, c = concept;
    void tick;
    if (h < 0) { etym = null; return; }
    engine.query<Etym>('word', { hex: h, label: c })
      .then((e) => { etym = e; error = ''; })
      .catch((e) => { etym = null; error = String(e.message ?? e); });
  });
</script>

<div class="bar">
  <label for="concept">Concept</label>
  <input id="concept" list="concept-list" value={concept} onchange={(e) => onconcept(e.currentTarget.value)} />
  <datalist id="concept-list">
    {#each concepts as c (c.id)}<option value={c.label}>{c.gloss}</option>{/each}
  </datalist>
</div>

{#if hex < 0}
  <p class="hint">Select a settled hex on the map to see how its word for “{concept}” came to be.</p>
{:else if error}
  <p class="hint">{error}. Choose a hex where people live.</p>
{:else if etym}
  <article>
    <h2><span class="ipa">{etym.form}</span> <span class="gloss">‘{etym.concept}’</span></h2>
    <p class="lang">{etym.lang}, hex {etym.hex}</p>
    <ol>
      <li class="origin"><span class="t"></span><span class="what">{etym.protoLang}</span><span class="ipa">*{etym.proto}</span></li>
      {#each etym.steps as s, i (i)}
        <li class:repair={s.innov === 0}>
          <span class="t">{s.tick * 25} yrs</span>
          <span class="what">{s.label}</span>
          <span class="ipa">{i < etym.steps.length - 1 ? '*' : ''}{s.after}</span>
        </li>
      {/each}
    </ol>
    <p class="root">Root: {etym.root}</p>
    {#if etym.steps.length === 0}<p class="hint">No change yet: the word is still as it was in {etym.protoLang}.</p>{/if}
  </article>
{/if}

<style>
  .bar { display: flex; gap: 0.5rem; align-items: center; margin-bottom: 1rem; }
  label { font-size: var(--t-sm); color: var(--ink-soft); }
  h2 { font-family: var(--serif); font-weight: 400; font-size: var(--t-2xl); margin: 0; }
  .gloss { font-size: var(--t-lg); color: var(--ink-soft); }
  .lang { margin: 0 0 0.8rem; color: var(--ink-soft); }
  ol { list-style: none; padding: 0; margin: 0; }
  li { display: grid; grid-template-columns: 4.5rem 1fr auto; gap: 0.75rem; padding: 0.3rem 0; border-bottom: 1px solid var(--rule-faint); align-items: baseline; }
  li.repair .what { color: var(--ink-soft); font-style: italic; }
  .t { color: var(--ink-soft); font-size: var(--t-sm); font-variant-numeric: tabular-nums; }
  .origin .what { font-style: italic; }
  .root { color: var(--ink-soft); font-size: var(--t-sm); }
  .hint { color: var(--ink-soft); }
</style>
