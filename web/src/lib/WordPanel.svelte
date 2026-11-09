<script lang="ts">
  import type { CellEtym, Concept, Engine, Etym } from './engine';
  import Gloss from './Gloss.svelte';

  interface Props {
    engine: Engine; hex: number; tick: number; concepts: Concept[]; concept: string;
    cell: number; cells: Record<string, string[]>;
    onconcept: (c: string) => void; oncell: (k: number) => void;
  }
  let { engine, hex, tick, concepts, concept, cell, cells, onconcept, oncell }: Props = $props();
  let etym = $state<Etym | null>(null);
  let cet = $state<CellEtym | null>(null);
  let error = $state('');

  const pos = $derived(concepts.find((c) => c.label === concept)?.pos);
  const forms = $derived(pos ? cells[pos] ?? [] : []);
  const useCell = $derived(pos !== undefined && cell >= 0 && cell < forms.length);

  $effect(() => {
    const h = hex, c = concept, k = cell, inflected = useCell;
    void tick;
    if (h < 0) { etym = null; cet = null; return; }
    const q = inflected ? engine.query<CellEtym>('cell', { hex: h, label: c, cell: k }) : engine.query<Etym>('word', { hex: h, label: c });
    q.then((e: any) => {
      if (inflected) { cet = e; etym = null; } else { etym = e; cet = null; }
      error = '';
    }).catch((e) => { etym = null; cet = null; error = String(e.message ?? e); });
  });
</script>

<div class="bar">
  <label for="concept">Concept</label>
  <input id="concept" list="concept-list" value={concept} onchange={(e) => onconcept(e.currentTarget.value)} />
  <datalist id="concept-list">
    {#each concepts as c (c.id)}<option value={c.label}>{c.gloss}</option>{/each}
  </datalist>
  {#if forms.length}
    <label for="form">Form</label>
    <select id="form" value={useCell ? cell : -1} onchange={(e) => oncell(+e.currentTarget.value)}>
      <option value={-1}>dictionary form</option>
      {#each forms as f, k (k)}<option value={k}>{f}</option>{/each}
    </select>
  {/if}
</div>

{#if hex < 0}
  <p class="hint">Select a settled hex on the map to see how its word for “{concept}” came to be.</p>
{:else if error}
  <p class="hint">{error}. Choose a hex where people live.</p>
{:else if etym}
  <article>
    <h2><span class="ipa">{etym.form || '—'}</span> <span class="gloss">‘{etym.concept}’</span></h2>
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
    {#if etym.spelling}<p class="root">Written <span class="spell">⟨{etym.spelling}⟩</span> in the {etym.script}{etym.gap > 0 ? `: spelling and sound have drifted ${Math.round(etym.gap / 10)}% apart` : ', as it sounds'}.</p>{/if}
    {#if etym.loan}<p class="loan">{etym.loan[0].toUpperCase() + etym.loan.slice(1)}: it has gone through only the changes since.</p>{/if}
    <p class="root">Root: {etym.root}</p>
    {#if etym.senses?.length}
      <p class="root">The same word also means
        {#each etym.senses as x, i (x)}{i ? ', ' : ''}<button class="link" onclick={() => onconcept(x)}>‘{x}’</button>{/each}.</p>
    {/if}
    {#if etym.also}<p class="root">It is starting to be used for <button class="link" onclick={() => onconcept(etym!.also)}>‘{etym.also}’</button> as well; the word for that may give way to it.</p>{/if}
    {#if etym.doublets?.length}
      <p class="root">Doublet{etym.doublets.length > 1 ? 's' : ''}: the same root came in twice, also giving
        {#each etym.doublets as d, i (d.concept)}{i ? ', ' : ''}<span class="ipa">{d.form}</span> <button class="link" onclick={() => onconcept(d.concept)}>‘{d.concept}’</button>{d.loan ? ' (a loan)' : ''}{/each}.</p>
    {/if}
    {#if etym.steps.length === 0 && !etym.loan && etym.form}<p class="hint">No change yet: the word is still as it was in {etym.protoLang}.</p>{/if}
  </article>
{:else if cet}
  <article>
    <h2><span class="ipa">{cet.form}</span> <span class="gloss">‘{cet.concept}’, {cet.cell}</span></h2>
    <p class="lang">{cet.lang ?? ''}, hex {hex} · <Gloss text={cet.gloss} /></p>
    <ol>
      <li class="origin"><span class="t"></span><span class="what">{cet.protoLang} <Gloss text={cet.protoGloss} /></span><span class="ipa">*{cet.proto}</span></li>
      {#each cet.steps as s, i (i)}
        <li class:gram={s.label.includes('clitic') || s.label.includes('suffix') || s.label.includes('prefix')} class:analogy={s.label.startsWith('analogy')}>
          <span class="t">{s.tick * 25} yrs</span>
          <span class="what">{s.label}</span>
          <span class="ipa">{i < cet.steps.length - 1 ? '*' : ''}{s.after}</span>
        </li>
      {/each}
    </ol>
    {#if cet.base}<p class="root">cf. {cet.baseLabel} <span class="ipa">{cet.base}</span>{cet.irregular ? ': this form is irregular' : ''}.</p>{/if}
    {#if cet.irregular}
      <p class="root">Analogy {cet.freq <= 2 ? 'has so far spared it: frequent words resist levelling' : 'may yet level it: rare words are regularized first'} (frequency class {cet.freq}).</p>
    {/if}
    {#if cet.steps.length === 0}<p class="hint">Unchanged since {cet.protoLang}.</p>{/if}
  </article>
{/if}

<style>
  .bar { display: flex; gap: 0.5rem; align-items: center; margin-bottom: 1rem; flex-wrap: wrap; }
  label { font-size: var(--t-sm); color: var(--ink-soft); }
  h2 { font-family: var(--serif); font-weight: 400; font-size: var(--t-2xl); margin: 0; }
  .gloss { font-size: var(--t-lg); color: var(--ink-soft); }
  .lang { margin: 0 0 0.8rem; color: var(--ink-soft); }
  ol { list-style: none; padding: 0; margin: 0; }
  li { display: grid; grid-template-columns: 4.5rem 1fr auto; gap: 0.75rem; padding: 0.3rem 0; border-bottom: 1px solid var(--rule-faint); align-items: baseline; }
  li.repair .what { color: var(--ink-soft); font-style: italic; }
  li.gram .what { font-weight: 600; }
  li.analogy .what { font-style: italic; }
  .t { color: var(--ink-soft); font-size: var(--t-sm); font-variant-numeric: tabular-nums; }
  .origin .what { font-style: italic; }
  .root { color: var(--ink-soft); font-size: var(--t-sm); }
  .hint { color: var(--ink-soft); }
  .spell { font-family: var(--serif); font-style: italic; }
  .loan { color: var(--isogloss); font-size: var(--t-sm); margin-bottom: 0; }
  .link { background: none; border: none; padding: 0; cursor: pointer; font: inherit; color: inherit; text-decoration: underline dotted; }
</style>
