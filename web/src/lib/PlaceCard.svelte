<script lang="ts">
  import type { Engine, Place } from './engine';

  interface Props { engine: Engine; hex: number; tick: number }
  let { engine, hex, tick }: Props = $props();
  let place = $state<Place | null>(null);

  $effect(() => {
    void tick;
    const h = hex;
    if (h < 0) { place = null; return; }
    engine.query<Place>('place', { hex: h }).then((p) => (place = p)).catch(() => (place = null));
  });
</script>

{#if place}
  <div class="place">
    {#if place.name}
      <p class="nm"><strong>{place.name}</strong> <span class="ipa">{place.form}</span></p>
      {#if place.polity}<p class="from">{place.capital ? 'Capital of' : 'Part of'} the polity of {place.polity}{place.rule ? `; under foreign rule: ${place.rule}` : ''}.</p>{/if}
      <p class="from">
        from {place.namedBy} <span class="ipa">*{place.original}</span> ‘{place.gloss}’{place.substrate ? ', the language spoken here before the family arrived' : ''}{#if place.since}; taken over in year {place.since * 25}{/if}.
      </p>
      {#if place.steps?.length}
        <details>
          <summary>{place.steps.length} {place.steps.length === 1 ? 'change' : 'changes'} since</summary>
          <ol>
            <li><span class="t"></span><span class="what">as taken over</span><span class="ipa">{place.adopted}</span></li>
            {#each place.steps as s, i (i)}
              <li><span class="t">{s.tick * 25}</span><span class="what">{s.label}</span><span class="ipa">{s.after}</span></li>
            {/each}
          </ol>
        </details>
      {/if}
    {:else}
      <p class="nm"><strong>{place.originalName}</strong> <span class="ipa">{place.original}</span></p>
      <p class="from">‘{place.gloss}’ in {place.namedBy}. Nobody from the family lives here yet.</p>
    {/if}
  </div>
{/if}

<style>
  .place { margin-top: 0.6rem; font-size: var(--t-sm); }
  .nm { margin: 0; font-family: var(--serif); font-size: var(--t-lg); }
  .nm .ipa { font-family: var(--ipa, inherit); font-size: var(--t-md); color: var(--ink-soft); margin-left: 0.3rem; }
  .from { margin: 0.1rem 0 0; color: var(--ink-soft); }
  summary { cursor: pointer; color: var(--ink-soft); margin-top: 0.2rem; }
  ol { list-style: none; padding: 0; margin: 0.3rem 0 0; }
  li { display: grid; grid-template-columns: 3rem 1fr auto; gap: 0.6rem; padding: 0.15rem 0; border-bottom: 1px solid var(--rule-faint); }
  .t { color: var(--ink-soft); font-variant-numeric: tabular-nums; }
</style>
