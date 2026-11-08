<script lang="ts">
  import type { Engine, Paradigm, Sketch } from './engine';
  import { langColor } from './palette';

  interface Props {
    engine: Engine;
    lang: number;
    tick: number;
    oninnov: (id: number) => void;
    oncell: (concept: string, cell: number) => void;
  }
  let { engine, lang, tick, oninnov, oncell }: Props = $props();
  let sk = $state<Sketch | null>(null);
  let para = $state<Paradigm | null>(null);
  let lexeme = $state('');
  let showAll = $state(false);

  $effect(() => {
    void tick;
    const id = lang;
    if (id < 0) { sk = null; return; }
    engine.query<Sketch>('sketch', { id }).then((s) => {
      sk = s;
      if (!lexeme && s.examples?.length) lexeme = s.examples[0];
    });
  });
  $effect(() => {
    void tick;
    const id = lang, l = lexeme;
    if (id < 0 || !l) { para = null; return; }
    engine.query<Paradigm>('paradigm', { lang: id, label: l }).then((p) => (para = p));
  });

  const bound = $derived(sk ? sk.exponents.filter((e) => e.stage === 'affix' || e.stage === 'clitic') : []);
  const free = $derived(sk ? sk.exponents.filter((e) => e.stage === 'periphrasis' || e.stage === 'fixed construction') : []);
  const absent = $derived(sk ? sk.exponents.filter((e) => e.stage === 'none' || e.stage === 'lost') : []);
  const changes = $derived(sk ? (showAll ? sk.changes : sk.changes.filter((c) => c.kind !== 'sound')) : []);
  const pct = (x: number) => `${Math.round(x / 10)}%`;
  const ratio = (x: number) => (x / 1000).toFixed(2);
  // Lay a paradigm out as a grid: case rows × number columns for nouns,
  // person rows × tense columns for verbs. Categories the language does not
  // inflect for are left out (their cells equal the base form).
  const isBound = (slot: string) => bound.some((e) => e.slot === slot);
  const grid = $derived.by(() => {
    if (!para?.cells) return null;
    const cells = para.cells;
    const find = (label: string) => cells.find((c) => c.label === label);
    if (para.pos === 'noun') {
      const cols = isBound('PL') ? ['singular', 'plural'] : ['form'];
      const rows = ['', ...['ACC', 'GEN', 'DAT', 'LOC', 'DEF'].filter(isBound)];
      return {
        cols,
        rows: rows.map((r) => ({
          label: r === '' ? 'base' : r,
          cells: [find(r ? `${r}.SG` : 'SG'), ...(cols.length > 1 ? [find(r ? `${r}.PL` : 'PL')] : [])],
        })),
      };
    }
    const persons = ['1SG', '2SG', '3SG', '1PL', '2PL', '3PL'];
    const agr = persons.some(isBound);
    const cols = isBound('PST') ? ['present', 'past'] : ['present'];
    const prow = (p: string) => ({ label: p, cells: [find(p), ...(cols.length > 1 ? [find(`PST.${p}`)] : [])] });
    return {
      cols,
      rows: [
        ...(agr ? persons.map(prow) : [prow('3SG')]),
        ...['FUT', 'PRF', 'PROG'].filter(isBound).map((t) => ({ label: `${t}.3SG`, cells: [find(`${t}.3SG`), ...(cols.length > 1 ? [undefined] : [])] })),
      ],
    };
  });
</script>

{#if !sk}
  <p class="hint">Select a hex or a language in the tree to see a sketch of its grammar.</p>
{:else}
  <h2><span class="sw" style:background={langColor(sk.lang.id)}></span>{sk.lang.name}</h2>
  <p class="meta">
    Spoken in {sk.lang.size} {sk.lang.size === 1 ? 'community' : 'communities'}{sk.lang.born > 0 ? `, a separate language since year ${sk.lang.born * 25}` : ''}.
    Morphologically <strong>{sk.typology.type}</strong>; basic order <strong>{sk.order[0].value}</strong>.
  </p>

  <section>
    <h3>Sounds</h3>
    <dl>
      <dt>Consonants</dt><dd class="ipa">{sk.consonants.join(' ')}</dd>
      <dt>Vowels</dt><dd class="ipa">{sk.vowels.join(' ')}</dd>
      <dt>Syllables</dt><dd>{sk.template}, illegal clusters repaired by {sk.repair}</dd>
      <dt>Stress</dt><dd>{sk.stress}</dd>
    </dl>
  </section>

  <section>
    <h3>Morphology</h3>
    <div class="metrics">
      <div><span class="big">{ratio(sk.typology.synthesis)}</span><span class="lab">morphemes per word</span></div>
      <div><span class="big">{pct(sk.typology.fusion)}</span><span class="lab">of affixes fused with the stem</span></div>
      <div><span class="big">{ratio(sk.typology.irregularity)}</span><span class="lab">stem forms per paradigm</span></div>
    </div>
    {#if bound.length}
      <h4>Inflections</h4>
      <ul class="exps">
        {#each bound as e (e.slot)}
          <li><span class="gl">{e.slot}</span> {e.name}: {e.kind}{#if e.marker}<span class="ipa mk">{e.marker}</span>{/if}{#if e.source}, from ‘{e.source}’{:else}, inherited{/if}</li>
        {/each}
      </ul>
    {/if}
    {#if free.length}
      <h4>Separate words</h4>
      <ul class="exps">
        {#each free as e (e.slot)}
          <li><span class="gl">{e.slot}</span> {e.name}: {e.stage} with ‘{e.source}’ <span class="ipa">{e.sourceForm}</span></li>
        {/each}
      </ul>
    {/if}
    {#if absent.length}
      <p class="note">Not marked: {absent.map((e) => e.name + (e.stage === 'lost' ? ' (worn away)' : '')).join(', ')}.</p>
    {/if}
  </section>

  <section>
    <h3>Word order</h3>
    <ul class="order">
      {#each sk.order as o (o.param)}<li><span class="lab">{o.param}</span> {o.value}</li>{/each}
    </ul>
  </section>

  {#if para?.lexemes}
    <section>
      <h3>Paradigms</h3>
      <label class="pick">Word
        <select bind:value={lexeme}>
          {#each ['noun', 'verb'] as pos (pos)}
            <optgroup label={pos === 'noun' ? 'Nouns' : 'Verbs'}>
              {#each para.lexemes.filter((x) => x.pos === pos) as x (x.concept)}
                <option value={x.concept}>{x.concept}{x.irregular ? ' (irregular)' : ''}</option>
              {/each}
            </optgroup>
          {/each}
        </select>
      </label>
      {#if grid && para.cells}
        <table class="para">
          <thead><tr><th></th>{#each grid.cols as c (c)}<th scope="col">{c}</th>{/each}</tr></thead>
          <tbody>
            {#each grid.rows as row (row.label)}
              <tr>
                <th scope="row" class="gl">{row.label}</th>
                {#each row.cells as c, i (i)}
                  {#if c}
                    <td class:irr={c.irregular}>
                      <button class="link ipa" onclick={() => oncell(para!.concept!, c.cell)}
                        title={c.irregular ? `irregular: the regular pattern gives ${c.regular}` : c.gloss}>{c.form}</button>
                      {#if c.irregular}<span class="mark" aria-label="irregular">●</span>{/if}
                    </td>
                  {:else}<td class="na"></td>{/if}
                {/each}
              </tr>
            {/each}
          </tbody>
        </table>
        <p class="note">
          <span class="mark">●</span> irregular: not what the language’s regular pattern would give
          ({para.stems} stem {para.stems === 1 ? 'form' : 'forms'}). Click a form for its history.
        </p>
      {/if}
    </section>
  {/if}

  <section>
    <h3>Changes, oldest first</h3>
    <label class="all"><input type="checkbox" bind:checked={showAll} /> include sound changes</label>
    <ol class="changes">
      {#each changes as c, i (i)}
        <li>
          <span class="t">{c.tick * 25}</span>
          {#if c.innov}
            <button class="link" title={c.rule} onclick={() => oninnov(c.innov!)}>{c.label}</button>
          {:else}<em>{c.label}</em>{/if}
        </li>
      {:else}
        <li class="note">No grammatical or word-order changes yet.</li>
      {/each}
    </ol>
  </section>
{/if}

<style>
  h2 { font-family: var(--serif); font-weight: 400; font-size: var(--t-2xl); margin: 0; display: flex; align-items: center; gap: 0.5rem; }
  h3 { font-family: var(--serif); font-weight: 700; font-size: var(--t-lg); margin: 1.3rem 0 0.4rem; }
  h4 { font-size: var(--t-sm); font-weight: 600; margin: 0.8rem 0 0.2rem; color: var(--ink-soft); text-transform: uppercase; letter-spacing: 0.05em; }
  .sw { width: 1rem; height: 1rem; border-radius: 2px; }
  .meta, .hint, .note { color: var(--ink-soft); }
  .note { font-size: var(--t-sm); }
  dl { display: grid; grid-template-columns: 8rem 1fr; gap: 0.3rem 1rem; margin: 0.4rem 0; }
  dt { color: var(--ink-soft); font-size: var(--t-sm); }
  dd { margin: 0; }
  .metrics { display: flex; flex-wrap: wrap; gap: 0.5rem 1.8rem; margin: 0.3rem 0 0.4rem; }
  .metrics div { display: flex; flex-direction: column; }
  .big { font-family: var(--serif); font-size: var(--t-xl); font-variant-numeric: tabular-nums; }
  .lab { font-size: var(--t-xs); color: var(--ink-soft); }
  .exps, .order { list-style: none; padding: 0; margin: 0; font-size: var(--t-sm); }
  .exps li { padding: 0.12rem 0; }
  .mk { margin-left: 0.35em; }
  .order { display: grid; grid-template-columns: repeat(auto-fill, minmax(13rem, 1fr)); gap: 0.2rem 1rem; }
  .order .lab { display: block; }
  .gl { font-variant-caps: all-small-caps; letter-spacing: 0.04em; color: var(--ink-soft); display: inline-block; min-width: 2.6rem; }
  .pick { font-size: var(--t-sm); color: var(--ink-soft); display: flex; gap: 0.4rem; align-items: center; }
  .para { border-collapse: collapse; margin-top: 0.6rem; font-size: var(--t-md); }
  .para th, .para td { padding: 0.18rem 0.8rem 0.18rem 0; text-align: left; border-bottom: 1px solid var(--rule-faint); }
  .para thead th { font-size: var(--t-sm); color: var(--ink-soft); font-weight: 500; }
  .para td.irr button { color: var(--isogloss); font-weight: 600; }
  .mark { color: var(--isogloss); font-size: 0.6em; vertical-align: super; margin-left: 0.15rem; }
  .all { font-size: var(--t-sm); color: var(--ink-soft); display: flex; gap: 0.35rem; align-items: center; }
  .changes { padding: 0; margin: 0.4rem 0 0; list-style: none; columns: 2; column-gap: 1.5rem; font-size: var(--t-sm); }
  .changes li { break-inside: avoid; padding: 0.1rem 0; }
  .t { display: inline-block; width: 3rem; color: var(--ink-soft); font-variant-numeric: tabular-nums; }
  .link { background: none; border: none; padding: 0; cursor: pointer; text-align: left; }
  .link:hover { text-decoration: underline; }
  @media (max-width: 700px) { .changes { columns: 1; } dl { grid-template-columns: 6.5rem 1fr; } }
</style>
