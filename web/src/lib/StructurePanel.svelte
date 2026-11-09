<script lang="ts">
  import type { Engine } from './engine';

  interface Slot { slot: string; name: string; pos: string; stage: string; source?: string; sources: string[] }
  interface Param { param: string; value: string; values: string[] }
  interface Structure { slots: Slot[]; order: Param[]; template: string; maxOnset: number; maxCoda: number }
  interface Props { engine: Engine; hex: number; tick: number; onscheduled: (msg: string) => void }
  let { engine, hex, tick, onscheduled }: Props = $props();

  let st = $state<Structure | null>(null);
  let everywhere = $state(false);
  let slotName = $state('FUT');
  let source = $state('');
  let param = $state('clause order');
  let value = $state('');
  let error = $state('');

  $effect(() => {
    void tick;
    const h = hex;
    if (h < 0) { st = null; return; }
    engine.query<Structure>('structure', { hex: h }).then((s) => (st = s)).catch(() => (st = null));
  });

  const slot = $derived(st?.slots.find((s) => s.slot === slotName) ?? null);
  const par = $derived(st?.order.find((p) => p.param === param) ?? null);
  const next: Record<string, string> = { periphrasis: 'fixed construction', 'fixed construction': 'clitic', clitic: 'affix' };
  const bound = $derived(slot ? slot.stage === 'clitic' || slot.stage === 'affix' : false);
  $effect(() => {
    if (slot && !slot.sources.includes(source)) source = slot.sources[0] ?? '';
  });
  $effect(() => {
    if (par && !par.values.includes(value)) value = par.values.find((v) => v !== par.value) ?? par.value;
  });

  async function act(kind: string, rule: string, what: string) {
    error = '';
    try {
      const r = await engine.call<{ tick: number }>('intervene', { kind, rule, hex: everywhere ? -1 : hex });
      onscheduled(`${what} ${everywhere ? 'everywhere' : 'here, spreading from this hex'} (generation ${r.tick}).`);
    } catch (e: any) {
      error = e.message ?? String(e);
    }
  }
</script>

<h3 class="ev">Grammar and syllables</h3>
<p class="hint">
  Force a structural change. Like a sound change, it starts here (or everywhere) at the next generation and spreads to
  neighbours that accept it; it goes into the save.
</p>
{#if hex < 0 || !st}
  <p class="hint">Select a settled hex on the map first.</p>
{:else}
  <div class="where" role="group" aria-label="Where it starts">
    <label><input type="radio" bind:group={everywhere} value={false} /> start in this hex</label>
    <label><input type="radio" bind:group={everywhere} value={true} /> everywhere</label>
  </div>

  <h4>A grammatical construction</h4>
  <div class="row">
    <select bind:value={slotName} aria-label="Grammatical category">
      {#each st.slots as s (s.slot)}<option value={s.slot}>{s.name} ({s.slot})</option>{/each}
    </select>
    {#if slot}
      <span class="now">now: {slot.stage === 'none' ? 'not marked' : slot.stage}{slot.source ? ` from ‘${slot.source}’` : ''}</span>
    {/if}
  </div>
  {#if slot}
    <div class="row">
      <label>build it from
        <select bind:value={source} aria-label="Source word">
          {#each slot.sources as x (x)}<option value={x}>‘{x}’</option>{/each}
        </select>
      </label>
      <button disabled={bound || !source} onclick={() => act('gram', `${slot.slot} ${source}`, `A ${slot.name} built on ‘${source}’ starts`)}
        title={bound ? 'Already a clitic or affix: push it, or wait for it to wear away' : ''}>Start construction</button>
      {#if next[slot.stage]}
        <button onclick={() => act('gram', `${slot.slot} +`, `The ${slot.name} becomes a ${next[slot.stage]}`)}>Push: {slot.stage} → {next[slot.stage]}</button>
      {/if}
    </div>
    <p class="note">The cycle: a separate word (periphrasis) → fixed construction → clitic (=) → affix (+); affixes later wear away and are renewed.</p>
  {/if}

  <h4>Word order</h4>
  <div class="row">
    <select bind:value={param} aria-label="Word-order parameter">
      {#each st.order as p (p.param)}<option value={p.param}>{p.param}</option>{/each}
    </select>
    {#if par}
      <span class="now">now: {par.value}</span>
      <select bind:value={value} aria-label="New value">
        {#each par.values as x (x)}<option value={x}>{x}</option>{/each}
      </select>
      <button disabled={value === par.value} onclick={() => act('order', `${par.param}=${value}`, `${par.param} becomes ${value}`)}>Set</button>
    {/if}
  </div>

  <h4>Syllables</h4>
  <div class="row">
    <span class="now">now: <span class="ipa">{st.template}</span></span>
    <button disabled={st.maxOnset >= 3} onclick={() => act('syllables', 'onset+1', 'Longer onsets are allowed')}>Longer onsets</button>
    <button disabled={st.maxOnset <= 1} onclick={() => act('syllables', 'onset-1', 'Onsets shrink')}>Shorter onsets</button>
    <button disabled={st.maxCoda >= 3} onclick={() => act('syllables', 'coda+1', 'Longer codas are allowed')}>Longer codas</button>
    <button disabled={st.maxCoda <= 0} onclick={() => act('syllables', 'coda-1', 'Codas shrink')}>Shorter codas</button>
  </div>
  <p class="note">Shorter limits repair the words that no longer fit (an inserted vowel or a dropped consonant, as the language does); longer ones only let later changes keep their clusters.</p>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
{/if}

<style>
  h3.ev { font-family: var(--serif); font-weight: 700; font-size: var(--t-lg); margin: 1.4rem 0 0.2rem; }
  h4 { font-size: var(--t-md); margin: 0.8rem 0 0.2rem; }
  .row { display: flex; flex-wrap: wrap; gap: 0.4rem 0.6rem; align-items: center; margin: 0.25rem 0; }
  .row label { display: inline-flex; gap: 0.35rem; align-items: center; font-size: var(--t-sm); color: var(--ink-soft); }
  .where { display: flex; gap: 1rem; font-size: var(--t-sm); color: var(--ink-soft); margin: 0.3rem 0; }
  .where label { display: inline-flex; gap: 0.3rem; align-items: center; }
  .now { font-size: var(--t-sm); color: var(--ink-soft); }
  .note { font-size: var(--t-xs); color: var(--ink-soft); margin: 0.2rem 0 0; max-width: 60ch; }
  .hint { color: var(--ink-soft); font-size: var(--t-sm); }
  .error { color: var(--isogloss); }
</style>
