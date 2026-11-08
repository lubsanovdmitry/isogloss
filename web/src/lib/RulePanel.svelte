<script lang="ts">
  import type { Engine } from './engine';

  interface Props { engine: Engine; hex: number; onscheduled: (msg: string) => void }
  let { engine, hex, onscheduled }: Props = $props();

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

<style>
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
