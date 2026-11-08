<script lang="ts">
  import type { CampaignDetail, Engine } from './engine';

  interface Props { engine: Engine; tick: number; hex: number; onscheduled: (msg: string) => void; oninnov: (id: number) => void }
  let { engine, tick, hex, onscheduled, oninnov }: Props = $props();
  let c = $state<CampaignDetail | null>(null);
  let script = $state('alphabet');
  let error = $state('');

  $effect(() => {
    void tick;
    engine.query<CampaignDetail>('campaign').then((x) => (c = x));
  });

  async function act(kind: string, args: Record<string, unknown>, done: string) {
    error = '';
    try {
      const r = await engine.call<{ tick: number }>('intervene', { kind, ...args });
      onscheduled(`${done}: generation ${r.tick}, if you still have the influence then.`);
      c = await engine.query<CampaignDetail>('campaign');
    } catch (e: any) {
      error = e.message ?? String(e);
    }
  }
  const cost = (k: string) => c?.costs?.[k] ?? 0;
  const can = (k: string) => (c?.influence ?? 0) >= cost(k) && (c?.alive ?? false);
</script>

{#if c}
  <h2>{c.name}</h2>
  <div class="metrics">
    <div><span class="big">{c.influence}</span><span class="lab">influence (earned {c.earned}, spent {c.spent})</span></div>
    <div><span class="big">{c.score}</span><span class="lab">score</span></div>
    <div><span class="big">{c.members ?? '–'}</span><span class="lab">communities in your polity</span></div>
  </div>
  {#if !c.alive}<p class="warn">Your polity has fallen. Your language may live on; the score still counts its speakers and daughters.</p>{/if}

  <h3>Goals</h3>
  <ul class="goals">
    {#each c.goals as g (g.kind)}
      <li class:met={g.met}>
        <span class="tick" aria-label={g.met ? 'met' : 'not yet'}>{g.met ? '✓' : '○'}</span>
        <span>{g.text}{g.kind === 'spoken' || g.kind === 'daughters' ? ` (now ${g.value})` : ''}</span>
      </li>
    {/each}
  </ul>
  <p class="note">Score: speakers of your language per hundred, 50 per daughter language, 300 per goal met, 200 while your polity stands.</p>

  <h3>Your polity</h3>
  <div class="actions">
    {#if c.script}<span class="have">✓ writes</span>{:else}
      <select bind:value={script} aria-label="Script type"><option>alphabet</option><option>abjad</option><option>syllabary</option></select>
      <button disabled={!can('script')} onclick={() => act('script', { rule: script }, 'Writing is coming')}>Adopt writing ({cost('script')})</button>
    {/if}
    {#if c.standard}<span class="have">✓ has a standard</span>{:else}
      <button disabled={!can('standard')} onclick={() => act('standard', {}, 'A standard will be fixed')}>Fix a standard ({cost('standard')})</button>
    {/if}
    {#if c.seafaring}<span class="have">✓ seafaring</span>{:else}
      <button disabled={!can('seafaring')} onclick={() => act('seafaring', {}, 'Your people will take to the sea')}>Take to the sea ({cost('seafaring')})</button>
    {/if}
  </div>
  <h3>The selected hex</h3>
  {#if hex < 0}
    <p class="note">Select a hex on the map to expand into it, conquer around it or open a trade route to it.</p>
  {:else}
    <div class="actions">
      <button disabled={!can('expand')} onclick={() => act('expand', { hex }, 'Your polity reaches out')}>Expand into it ({cost('expand')})</button>
      <button disabled={!can('conquer')} onclick={() => act('conquer', { hex }, 'Your warriors march')}>Conquer around it ({cost('conquer')})</button>
      <button disabled={!can('route')} onclick={() => act('route', { hex, from: c!.capital }, 'A trade route will open')}>Trade route from your capital ({cost('route')})</button>
    </div>
  {/if}

  <h3>Changes spreading in your land</h3>
  <p class="note">Encourage a change and it takes hold two and a half times as fast in your communities; suppress it and it barely spreads there.</p>
  {#if c.innovations?.length}
    <ul class="innovs">
      {#each c.innovations as i (i.id)}
        <li>
          <span class="what"><button class="link" title={i.rule} onclick={() => oninnov(i.id)}>{i.label}</button>
            <span class="soft">{i.kind}, in {i.applied} of {c.members}</span></span>
          <span class="do">
            {#if i.steer > 0}<span class="steer up">encouraged</span>{:else if i.steer < 0}<span class="steer down">suppressed</span>{:else}
              <button disabled={!can('encourage')} onclick={() => act('encourage', { innov: i.id }, 'You back ' + i.label)}>Encourage ({cost('encourage')})</button>
              <button disabled={!can('suppress')} onclick={() => act('suppress', { innov: i.id }, 'You frown on ' + i.label)}>Suppress ({cost('suppress')})</button>
            {/if}
          </span>
        </li>
      {/each}
    </ul>
  {:else}
    <p class="note">Nothing is spreading through your land right now.</p>
  {/if}
  {#if error}<p class="warn">{error}</p>{/if}
{/if}

<style>
  h2 { font-family: var(--serif); font-weight: 400; font-size: var(--t-2xl); margin: 0; }
  h3 { font-family: var(--serif); font-weight: 700; font-size: var(--t-lg); margin: 1.2rem 0 0.3rem; }
  .metrics { display: flex; flex-wrap: wrap; gap: 0.5rem 2rem; margin: 0.4rem 0; }
  .metrics div { display: flex; flex-direction: column; }
  .big { font-family: var(--serif); font-size: var(--t-2xl); font-variant-numeric: tabular-nums; }
  .lab, .note, .soft { font-size: var(--t-sm); color: var(--ink-soft); }
  .warn { color: var(--isogloss); }
  .goals, .innovs { list-style: none; padding: 0; margin: 0; }
  .goals li { display: flex; gap: 0.5rem; padding: 0.15rem 0; }
  .goals li.met { font-weight: 600; }
  .tick { width: 1.2rem; }
  .innovs li { display: grid; grid-template-columns: 1fr auto; gap: 0.3rem 0.8rem; align-items: center; padding: 0.3rem 0; border-bottom: 1px solid var(--rule-faint); }
  .what { display: flex; flex-direction: column; }
  .do { display: flex; gap: 0.3rem; justify-content: flex-end; flex-wrap: wrap; }
  .have { font-size: var(--t-sm); color: var(--ink-soft); align-self: center; }
  @media (max-width: 700px) { .innovs li { grid-template-columns: 1fr; } .do { justify-content: flex-start; } }
  .actions { display: flex; flex-wrap: wrap; gap: 0.4rem; }
  .steer { font-size: var(--t-xs); text-transform: uppercase; letter-spacing: 0.05em; }
  .steer.up { color: var(--ink); }
  .steer.down { color: var(--isogloss); }
  .link { background: none; border: none; padding: 0; cursor: pointer; text-align: left; }
  .link:hover { text-decoration: underline; }
</style>
