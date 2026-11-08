<script lang="ts">
  import type { Engine, Sketch } from './engine';
  import { langColor } from './palette';

  interface Props { engine: Engine; lang: number; tick: number; oninnov: (id: number) => void }
  let { engine, lang, tick, oninnov }: Props = $props();
  let sk = $state<Sketch | null>(null);

  $effect(() => {
    void tick;
    const id = lang;
    if (id < 0) { sk = null; return; }
    engine.query<Sketch>('lang', { id }).then((s) => (sk = s));
  });
</script>

{#if !sk}
  <p class="hint">Select a hex or a language in the tree to see a sketch of its sound system.</p>
{:else}
  <h2><span class="sw" style:background={langColor(sk.lang.id)}></span>{sk.lang.name}</h2>
  <p class="meta">
    Spoken in {sk.lang.size} {sk.lang.size === 1 ? 'community' : 'communities'}{sk.lang.born > 0 ? `, a separate language since year ${sk.lang.born * 25}` : ''}.
  </p>
  <dl>
    <dt>Consonants</dt><dd class="ipa">{sk.consonants.join(' ')}</dd>
    <dt>Vowels</dt><dd class="ipa">{sk.vowels.join(' ')}</dd>
    <dt>Syllables</dt><dd>{sk.template}, illegal clusters repaired by {sk.repair}</dd>
    <dt>Stress</dt><dd>{sk.stress}</dd>
    <dt>Words replaced</dt><dd>{sk.replacements}</dd>
  </dl>
  <h3>Sound changes, oldest first</h3>
  <ol class="changes">
    {#each sk.changes as c, i (i)}
      <li>
        <span class="t">{c.tick * 25}</span>
        {#if c.innov}
          <button class="link" title={c.rule} onclick={() => oninnov(c.innov!)}>{c.label}</button>
        {:else}<em>{c.label}</em>{/if}
      </li>
    {/each}
  </ol>
{/if}

<style>
  h2 { font-family: var(--serif); font-weight: 400; font-size: var(--t-2xl); margin: 0; display: flex; align-items: center; gap: 0.5rem; }
  h3 { font-family: var(--serif); font-weight: 700; font-size: var(--t-lg); margin: 1.2rem 0 0.4rem; }
  .sw { width: 1rem; height: 1rem; border-radius: 2px; }
  .meta, .hint { color: var(--ink-soft); }
  dl { display: grid; grid-template-columns: 8rem 1fr; gap: 0.3rem 1rem; margin: 0.8rem 0; }
  dt { color: var(--ink-soft); font-size: var(--t-sm); }
  dd { margin: 0; }
  .changes { padding: 0; margin: 0; list-style: none; columns: 2; column-gap: 1.5rem; font-size: var(--t-sm); }
  .changes li { break-inside: avoid; padding: 0.1rem 0; }
  .t { display: inline-block; width: 3rem; color: var(--ink-soft); font-variant-numeric: tabular-nums; }
  .link { background: none; border: none; padding: 0; cursor: pointer; text-align: left; }
  .link:hover { text-decoration: underline; }
  @media (max-width: 700px) { .changes { columns: 1; } }
</style>
