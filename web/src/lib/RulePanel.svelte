<script lang="ts">
  import type { Engine } from './engine';
  import StructurePanel from './StructurePanel.svelte';

  interface Props {
    engine: Engine; hex: number; tick: number; onscheduled: (msg: string) => void;
    polity?: { id: number; name: string } | null; // the selected hex's polity
  }
  let { engine, hex, tick, onscheduled, polity = null }: Props = $props();
  let thing = $state('wine');
  let script = $state('alphabet');

  async function act(kind: string, args: Record<string, unknown>, done: string) {
    error = '';
    try {
      const r = await engine.call<{ tick: number }>('intervene', { kind, ...args });
      onscheduled(`${done} (generation ${r.tick}).`);
    } catch (e: any) {
      error = e.message ?? String(e);
    }
  }

  let src = $state(`rule my-voicing:
    [stop -voice] => [+voice] / V _ V`);
  let preview = $state<{ changes: { concept: string; before: string; after: string }[]; count: number } | null>(null);
  let error = $state('');

  async function doPreview() {
    error = '';
    preview = null;
    try {
      preview = await engine.query('preview', { hex, rule: src });
    } catch (e: any) {
      error = e.message ?? String(e);
    }
  }
  interface Suggestion { name: string; family: string; rule: string; score: number; count: number; examples: { concept: string; before: string; after: string }[] }
  let suggestions = $state<Suggestion[] | null>(null);
  let salt = $state(0);
  let suggestedFor = $state(-1);

  async function suggest(again = false) {
    error = '';
    if (again) salt++;
    try {
      suggestions = await engine.query<Suggestion[]>('suggest', { hex, n: 8, salt });
      suggestedFor = hex;
    } catch (e: any) {
      error = e.message ?? String(e);
    }
  }
  // A different hex has different sounds: drop stale suggestions.
  $effect(() => {
    if (hex !== suggestedFor) suggestions = null;
  });

  function use(sg: Suggestion) {
    src = sg.rule.trimEnd();
    doPreview();
  }

  async function inject(where: number) {
    error = '';
    try {
      const r = await engine.call<{ tick: number }>('intervene', { rule: src, hex: where });
      onscheduled(where < 0 ? `Change scheduled everywhere at tick ${r.tick}.` : `Change scheduled in hex ${where} at tick ${r.tick}; it will spread from there.`);
    } catch (e: any) {
      error = e.message ?? String(e);
    }
  }
</script>

<div class="actions">
  <button onclick={() => suggest()} disabled={hex < 0}>Suggest changes here</button>
  {#if suggestions}<button onclick={() => suggest(true)}>Other suggestions</button>{/if}
</div>
{#if suggestions}
  {#if suggestions.length === 0}
    <p class="hint">No sound change fits this community's sounds right now.</p>
  {:else}
    <p class="hint">Changes the simulation itself would find plausible here, best first. Use one to load it into the editor below.</p>
    <ul class="sugg">
      {#each suggestions as sg (sg.rule)}
        <li>
          <div class="sg-head">
            <strong>{sg.name.replaceAll('-', ' ')}</strong> <span class="fam">{sg.family}</span>
            <span class="cnt">{sg.count} {sg.count === 1 ? 'word' : 'words'}</span>
            <button class="use" onclick={() => use(sg)}>Use</button>
          </div>
          <div class="ex">
            {#each sg.examples as e (e.concept)}<span><span class="c">{e.concept}</span> <span class="ipa">{e.before} → {e.after}</span></span>{/each}
          </div>
        </li>
      {/each}
    </ul>
  {/if}
{/if}

<p class="hint">
  Write a sound change in Isogloss rule syntax. Classes <code>C V stop affr fric obs nasal liquid glide son sib</code> are predefined.
</p>
<textarea bind:value={src} rows="6" spellcheck="false" aria-label="Rule source"></textarea>
<div class="actions">
  <button onclick={doPreview} disabled={hex < 0}>Preview on selected hex</button>
  <button onclick={() => inject(hex)} disabled={hex < 0}>Start it in this hex</button>
  <button onclick={() => inject(-1)}>Apply everywhere</button>
</div>
{#if hex < 0}<p class="hint">Select a settled hex to preview or seed the change there.</p>{/if}
{#if error}<p class="error" role="alert">{error}</p>{/if}
{#if preview}
  <p>{preview.count === 0 ? 'This rule changes no words here.' : `Changes ${preview.count} words here:`}</p>
  <ul class="pv">
    {#each preview.changes as c, i (i)}
      <li><span class="c">{c.concept}</span><span class="ipa">{c.before}</span><span class="ipa">{c.after}</span></li>
    {/each}
  </ul>
{/if}

<details>
  <summary>Syntax</summary>
  <pre>{`target => replacement / left _ right, other_left _ other_right
    except / left _ right

[+syl -back]   feature matrix      p t k => f θ x   positional mapping
{a e o}        list                 * => e / # _ s C  insertion
#  .  +  =     word, syllable, morpheme, clitic boundary
C*  (C)        zero or more, optional
stress:1  tone:rise  phon:breathy   prosody`}</pre>
</details>

<StructurePanel {engine} {hex} {tick} {onscheduled} />

<h3 class="ev">Events</h3>
<p class="hint">Change who talks to whom. Events act at the start of the next generation and go into the save, so a replay repeats them.</p>
{#if hex < 0}
  <p class="hint">Select a hex on the map first.</p>
{:else}
  <div class="actions">
    <button onclick={() => act('plague', { hex }, 'Plague breaks out here')}>Plague here</button>
    <button onclick={() => act('famine', { hex }, 'Famine strikes here')}>Famine here</button>
    <label class="inl">Invent <input bind:value={thing} size="8" aria-label="Thing to invent" /></label>
    <button onclick={() => act('invent', { hex, rule: thing.trim() }, `'${thing.trim()}' will appear here`)}>here</button>
  </div>
  {#if polity}
    <p class="hint">This hex belongs to <strong>{polity.name}</strong>.</p>
    <div class="actions">
      <button onclick={() => act('collapse', { polity: polity!.id }, `${polity!.name} will break up`)}>Break it up</button>
      <select bind:value={script} aria-label="Script type"><option>alphabet</option><option>abjad</option><option>syllabary</option></select>
      <button onclick={() => act('script', { polity: polity!.id, rule: script }, `${polity!.name} will start writing`)}>Give it writing</button>
      <button onclick={() => act('standard', { polity: polity!.id }, `${polity!.name} will fix a standard`)}>Fix a standard</button>
      <button onclick={() => act('seafaring', { polity: polity!.id }, `${polity!.name} will take to the sea`)}>Take to the sea</button>
    </div>
  {:else}
    <p class="hint">This hex belongs to no polity.</p>
  {/if}
{/if}

<style>
  .sugg { list-style: none; padding: 0; margin: 0.3rem 0 1rem; }
  .sugg li { padding: 0.35rem 0; border-bottom: 1px solid var(--rule-faint); }
  .sg-head { display: flex; gap: 0.5rem; align-items: baseline; flex-wrap: wrap; }
  .sg-head .fam, .sg-head .cnt { color: var(--ink-soft); font-size: var(--t-sm); }
  .sg-head .use { margin-left: auto; padding: 0.1rem 0.6rem; }
  .ex { display: flex; flex-wrap: wrap; gap: 0.2rem 1rem; font-size: var(--t-sm); margin-top: 0.15rem; }
  .ex .c { color: var(--ink-soft); }
  h3.ev { font-family: var(--serif); font-weight: 700; font-size: var(--t-lg); margin: 1.4rem 0 0.2rem; }
  .inl { display: inline-flex; gap: 0.3rem; align-items: center; font-size: var(--t-sm); color: var(--ink-soft); }
  textarea { width: 100%; font-family: var(--serif); font-size: var(--t-lg); line-height: 1.4; }
  .actions { display: flex; gap: 0.5rem; flex-wrap: wrap; margin: 0.5rem 0; }
  .hint { color: var(--ink-soft); font-size: var(--t-sm); }
  code { font-size: var(--t-sm); white-space: normal; overflow-wrap: anywhere; }
  .error { color: var(--isogloss); }
  .pv { list-style: none; padding: 0; columns: 2; font-size: var(--t-sm); }
  .pv li { display: grid; grid-template-columns: 6rem auto auto; gap: 0.5rem; break-inside: avoid; }
  .pv li .ipa:last-child::before { content: '→ '; color: var(--ink-soft); }
  .c { color: var(--ink-soft); }
  pre { font-size: var(--t-xs); white-space: pre-wrap; background: var(--paper-raised); padding: 0.6rem; border: 1px solid var(--rule-faint); }
  details { margin-top: 1rem; }
</style>
