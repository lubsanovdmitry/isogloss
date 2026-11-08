<script lang="ts">
  import { onMount } from 'svelte';
  import { Engine, type Concept, type Event, type Frame, type InnovInfo, type InnovSummary, type Snapshot, type TreeNode, type World, type ScenarioEntry } from './lib/engine';
  import Atlas from './lib/Atlas.svelte';
  import TreeItem from './lib/TreeItem.svelte';
  import WordPanel from './lib/WordPanel.svelte';
  import TablePanel from './lib/TablePanel.svelte';
  import LangPanel from './lib/LangPanel.svelte';
  import RulePanel from './lib/RulePanel.svelte';
  import Chronicle from './lib/Chronicle.svelte';
  import IntelPanel from './lib/IntelPanel.svelte';
  import FablePanel from './lib/FablePanel.svelte';
  import WaveChart from './lib/WaveChart.svelte';
  import PlaceCard from './lib/PlaceCard.svelte';

  const engine = new Engine();
  const YEARS = 25;

  let world = $state<World | null>(null);
  let snap = $state<Snapshot | null>(null);
  let events = $state<Event[]>([]);
  let concepts = $state<Concept[]>([]);
  let seed = $state('42');
  let scenarioID = $state('');
  let scenarios = $state<ScenarioEntry[]>([]);
  let selected = $state(-1);
  let showSubs = $state(false);
  let mapMode = $state<'langs' | 'polities'>('langs');
  let concept = $state('water');
  let tab = $state<'tree' | 'word' | 'table' | 'intel' | 'lang' | 'fable' | 'rules' | 'log'>('tree');
  let cell = $state(-1); // paradigm cell shown in Word history (-1 = dictionary form)
  let cells = $state<Record<string, string[]>>({});
  let running = $state(false);
  let speed = $state(2); // ticks per second
  let busy = $state(false);
  let status = $state('Loading the engine…');
  let innovs = $state<InnovSummary[]>([]);
  let overlay = $state<InnovInfo | null>(null);
  let frames = $state<Frame[]>([]);
  let viewIdx = $state(-1); // -1 = live
  let focusLang = $state(-1);

  const viewing = $derived(viewIdx >= 0 && viewIdx < frames.length ? frames[viewIdx] : null);
  const langOf = $derived.by(() => {
    const n = world ? world.hexes.length : 0;
    const a = new Int32Array(n).fill(-1);
    if (viewing) viewing.lang.forEach((l, i) => (a[i] = l));
    else if (snap) for (const [h, , l] of snap.comms) a[h] = l;
    return a;
  });
  const polityOf = $derived.by(() => {
    if (mapMode !== 'polities' || !snap || !world || viewing) return null;
    const a = new Int32Array(world.hexes.length).fill(-1);
    for (const [h, , , p] of snap.comms) a[h] = p;
    return a;
  });
  const capitals = $derived(snap && !viewing ? snap.polities.filter((p) => p.fell < 0).map((p) => p.capital) : []);
  const isoSet = $derived(overlay && !viewing ? new Set(overlay.applied) : null);
  const nodes = $derived(new Map<number, TreeNode>((snap?.tree ?? []).map((n) => [n.id, n])));
  const living = $derived(snap ? snap.langs.filter((l) => l.died < 0) : []);
  const selPolity = $derived.by(() => {
    if (!snap || selected < 0) return null;
    const c = snap.comms.find((x) => x[0] === selected);
    const p = c && c[3] >= 0 ? snap.polities.find((q) => q.id === c[3] && q.fell < 0) : undefined;
    return p ? { id: p.id, name: p.name } : null;
  });
  const selLang = $derived(selected >= 0 ? langOf[selected] : -1);
  const shownLang = $derived(focusLang >= 0 ? focusLang : selLang);

  function accept(s: Snapshot, reset = false) {
    snap = s;
    events = reset ? s.events : [...events, ...s.events];
    status = '';
  }

  async function refreshSide() {
    innovs = (await engine.query<InnovSummary[]>('innovs', { min: 4 })).slice(0, 60);
    frames = await engine.query<Frame[]>('frames');
    if (overlay) overlay = await engine.query<InnovInfo>('innov', { id: overlay.id });
  }

  async function newRun() {
    running = false;
    busy = true;
    status = 'Generating a continent and its proto-language…';
    try {
      const r = await engine.call<{ world: World; snapshot: Snapshot }>('init', { seed: seed.trim() || '1', scenario: scenarioID });
      world = r.world;
      accept(r.snapshot, true);
      selected = -1;
      overlay = null;
      viewIdx = -1;
      focusLang = -1;
      await refreshSide();
    } catch (e: any) {
      status = e.message ?? String(e);
    }
    busy = false;
  }

  async function step(n = 1) {
    if (!snap) return;
    busy = true;
    viewIdx = -1;
    try {
      accept(await engine.call<Snapshot>('step', { n, since: snap.eventCount }));
      await refreshSide();
    } catch (e: any) {
      status = e.message ?? String(e);
      running = false;
    }
    busy = false;
  }

  async function play() {
    running = !running;
    while (running) {
      const t0 = performance.now();
      await step(1);
      const wait = 1000 / speed - (performance.now() - t0);
      if (wait > 0) await new Promise((r) => setTimeout(r, wait));
    }
  }

  async function fork() {
    if (!viewing) return;
    running = false;
    busy = true;
    status = `Replaying to year ${viewing.tick * YEARS}…`;
    const r = await engine.call<{ world: World; snapshot: Snapshot }>('fork', { tick: viewing.tick });
    world = r.world;
    accept(r.snapshot, true);
    viewIdx = -1;
    await refreshSide();
    status = `Forked at year ${r.snapshot.tick * YEARS}. Later interventions were dropped.`;
    busy = false;
  }

  async function showInnov(id: number) {
    overlay = await engine.query<InnovInfo>('innov', { id });
  }

  async function bench() {
    busy = true;
    status = 'Timing a fresh 1000-year run…';
    const r = await engine.call<{ ms: number; languages: number }>('bench', { seed: seed.trim() || '1', ticks: 40 });
    status = `1000 years (40 generations) simulated in ${r.ms} ms in this browser, ending with ${r.languages} languages.`;
    busy = false;
  }

  async function exportSave() {
    const sv = await engine.call('save');
    const blob = new Blob([JSON.stringify(sv, null, 1)], { type: 'application/json' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `isogloss-seed${sv.seed}-t${sv.tick}.json`;
    a.click();
    URL.revokeObjectURL(a.href);
  }

  async function importSave(e: globalThis.Event) {
    const f = (e.currentTarget as HTMLInputElement).files?.[0];
    if (!f) return;
    running = false;
    busy = true;
    status = 'Replaying the saved run…';
    try {
      const sv = JSON.parse(await f.text());
      const r = await engine.call<{ world: World; snapshot: Snapshot }>('load', { save: sv.save ?? sv });
      world = r.world;
      seed = String(sv.save?.Seed ?? sv.seed ?? seed);
      accept(r.snapshot, true);
      await refreshSide();
    } catch (err: any) {
      status = 'Could not load that file: ' + (err.message ?? err);
    }
    busy = false;
  }

  function select(h: number) {
    selected = h;
    focusLang = -1;
    if (tab === 'tree' || tab === 'log') tab = 'word';
  }

  onMount(async () => {
    concepts = await engine.query<Concept[]>('concepts');
    scenarios = await engine.query<ScenarioEntry[]>('scenarios');
    cells = await engine.query<Record<string, string[]>>('cells');
    await newRun();
  });

  const tabs = [
    ['tree', 'Family tree'], ['word', 'Word history'], ['table', 'Word lists'], ['intel', 'Intelligibility'],
    ['lang', 'Grammar'], ['fable', 'Parallel text'], ['rules', 'Change rules'], ['log', 'Chronicle'],
  ] as const;
</script>

<header>
  <h1>Isogloss</h1>
  {#if snap}
    <p class="where">
      <span class="proto">{snap.proto}</span>
      {#if viewing}<strong> as of year {viewing.tick * YEARS}</strong>{:else} · year {snap.tick * YEARS}, generation {snap.tick}{/if}
      · {living.length} {living.length === 1 ? 'language' : 'languages'}
    </p>
  {/if}
  {#if snap?.scenario}
    <p class="scen"><strong>{snap.scenario.name}.</strong> {snap.scenario.summary} <em>Watch:</em> {snap.scenario.watch}
      {#if snap.tick < snap.scenario.ticks}<span class="soft">Runs for {snap.scenario.ticks * YEARS} years.</span>{/if}</p>
  {/if}
  <div class="controls">
    <label>Seed <input bind:value={seed} size="6" aria-label="Seed" /></label>
    <label>Start
      <select bind:value={scenarioID} aria-label="Scenario">
        <option value="">one family, one homeland</option>
        {#each scenarios as sc (sc.id)}<option value={sc.id}>{sc.name}</option>{/each}
      </select>
    </label>
    <button onclick={newRun} disabled={busy && !running}>New world</button>
    <span class="sep"></span>
    <button onclick={() => step(1)} disabled={busy || running}>Next generation</button>
    <button onclick={() => step(4)} disabled={busy || running}>+100 years</button>
    <button class="primary" onclick={play} aria-pressed={running}>{running ? 'Pause' : 'Run'}</button>
    <label class="speed">Speed
      <input type="range" min="1" max="10" bind:value={speed} aria-label="Generations per second" />
    </label>
  </div>
</header>

<main>
  <section class="atlas" aria-label="Map">
    {#if world}
      <Atlas {world} {langOf} {selected} isogloss={isoSet} level={!viewing ? overlay?.level : null} substrates={showSubs} {polityOf} {capitals} routes={viewing ? [] : (snap?.routes ?? [])} onselect={select} />
      <div class="under">
        {#if selected >= 0}<PlaceCard {engine} hex={selected} tick={snap?.tick ?? 0} />{/if}
        <div class="seg mapmode" role="group" aria-label="Map colours">
          <button aria-pressed={mapMode === 'langs'} onclick={() => (mapMode = 'langs')}>Languages</button>
          <button aria-pressed={mapMode === 'polities'} onclick={() => (mapMode = 'polities')}>Polities</button>
        </div>
        <span class="maplegend">○ capital · ┈ trade route{mapMode === 'polities' ? ' · grey: no polity' : ''}</span>
        {#if world.substrates?.length}
          <label class="subs"><input type="checkbox" bind:checked={showSubs} /> Show the languages spoken here before ({world.substrates.map((x) => x.name).join(', ')})</label>
        {/if}
        <label class="iso">
          Isogloss
          <select
            value={overlay?.id ?? ''}
            onchange={(e) => { const v = e.currentTarget.value; v ? showInnov(+v) : (overlay = null); }}
          >
            <option value="">none</option>
            {#if overlay && !innovs.some((i) => i.id === overlay!.id)}<option value={overlay.id}>{overlay.label}</option>{/if}
            {#each innovs as i (i.id)}<option value={i.id}>{i.label} (year {i.tick * YEARS}, {i.hexes} communities)</option>{/each}
          </select>
        </label>
        {#if overlay}
          <p class="rule ipa">{overlay.rule.split('\n').slice(1).join(' ').trim() || overlay.label}</p>
          <p class="wavenote">{overlay.level ? 'Still spreading: shading shows how far it has got in each community; hatched hexes have adopted it.' : 'No longer spreading.'}</p>
          <div class="chartrow"><WaveChart curve={overlay.curve} born={overlay.tick} /></div>
        {/if}
        {#if frames.length > 1}
          <label class="scrub">
            Look back
            <input
              type="range" min="0" max={frames.length} value={viewIdx < 0 ? frames.length : viewIdx}
              oninput={(e) => { const v = +e.currentTarget.value; viewIdx = v >= frames.length ? -1 : v; }}
              aria-label="Show the language map at an earlier time"
            />
          </label>
          {#if viewing}<button onclick={fork} disabled={busy}>Continue from year {viewing.tick * YEARS}</button>{/if}
        {/if}
      </div>
    {:else}
      <p class="loading">{status}</p>
    {/if}
  </section>

  <section class="inspector" aria-label="Inspector">
    <div class="tabs" role="tablist">
      {#each tabs as [id, label] (id)}
        <button role="tab" aria-selected={tab === id} class:on={tab === id} onclick={() => (tab = id)}>{label}</button>
      {/each}
    </div>
    <div class="panel" role="tabpanel">
      {#if snap}
        {#if tab === 'tree'}
          {#if nodes.get(0)}
            <ul class="tree"><TreeItem node={nodes.get(0)!} {nodes} langs={snap.langs} proto={snap.proto}
              onlang={(id) => { focusLang = id; tab = 'lang'; }} oninnov={showInnov} /></ul>
          {/if}
          <p class="hint">Subgroups are defined by the changes all their members share (sounds, grammar, word order), as in the comparative method. Click a change to draw its isogloss.</p>
        {:else if tab === 'word'}
          <WordPanel {engine} hex={selected} tick={snap.tick} {concepts} {concept} {cell} {cells}
            onconcept={(c) => { concept = c; cell = -1; }} oncell={(k) => (cell = k)} />
        {:else if tab === 'table'}
          <TablePanel {engine} tick={snap.tick} onpick={(c) => { concept = c; cell = -1; tab = 'word'; }} />
        {:else if tab === 'intel'}
          <IntelPanel {engine} tick={snap.tick} onlang={(id) => { focusLang = id; tab = 'lang'; }} />
        {:else if tab === 'lang'}
          <LangPanel {engine} lang={shownLang} tick={snap.tick} oninnov={showInnov}
            oncell={(c, k) => { concept = c; cell = k; if (selected < 0 || langOf[selected] !== shownLang) selected = living.find((l) => l.id === shownLang)?.centre ?? selected; tab = 'word'; }} />
        {:else if tab === 'fable'}
          <FablePanel {engine} tick={snap.tick} onlang={(id) => { focusLang = id; tab = 'lang'; }} />
        {:else if tab === 'rules'}
          <RulePanel {engine} hex={selected} polity={selPolity} onscheduled={(m) => (status = m)} />
        {:else}
          <Chronicle {events} oninnov={showInnov} onhex={select} />
        {/if}
      {/if}
    </div>
  </section>
</main>

<footer>
  <p class="status" role="status">{status}</p>
  <div class="files">
    <button onclick={exportSave} disabled={!snap}>Save run</button>
    <label class="file">Open run <input type="file" accept="application/json" onchange={importSave} /></label>
    <button onclick={bench} disabled={busy || running}>Time 1000 years</button>
  </div>
</footer>

<style>
  .scen { margin: 0.2rem 0 0.4rem; font-size: var(--t-sm); color: var(--ink-soft); max-width: 70rem; }
  .scen strong { color: var(--ink); }
  .scen .soft { margin-left: 0.3rem; }
  header {
    display: grid;
    grid-template-columns: auto 1fr;
    align-items: baseline;
    gap: 0.2rem 1.25rem;
    padding: 0.8rem 1.25rem 0.6rem;
    border-bottom: 1px solid var(--rule);
  }
  h1 { font-family: var(--serif); font-weight: 700; font-size: var(--t-2xl); margin: 0; letter-spacing: 0.01em; }
  .where { margin: 0; color: var(--ink-soft); }
  .proto { font-family: var(--serif); font-style: italic; font-size: var(--t-lg); color: var(--ink); }
  .controls { grid-column: 1 / -1; display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: center; }
  .controls label { font-size: var(--t-sm); color: var(--ink-soft); display: flex; align-items: center; gap: 0.35rem; }
  .sep { width: 1px; height: 1.4rem; background: var(--rule); margin: 0 0.25rem; }
  .speed input { width: 6rem; }

  main {
    display: grid;
    grid-template-columns: minmax(0, 1.25fr) minmax(0, 1fr);
    gap: 1.25rem;
    padding: 1rem 1.25rem;
    align-items: start;
  }
  .atlas { position: sticky; top: 0.5rem; }
  .under { display: flex; flex-wrap: wrap; gap: 0.5rem 1rem; align-items: center; margin-top: 0.5rem; font-size: var(--t-sm); }
  .under label { display: flex; gap: 0.4rem; align-items: center; color: var(--ink-soft); min-width: 0; }
  .iso { min-width: 0; max-width: 100%; }
  .iso select { max-width: min(22rem, 100%); min-width: 0; }
  .iso { color: var(--isogloss) !important; font-weight: 600; }
  .rule { margin: 0; color: var(--isogloss); flex-basis: 100%; }
  .chartrow { flex-basis: 100%; }
  .under :global(.place) { flex-basis: 100%; }
  .mapmode { display: inline-flex; }
  .mapmode button { border-radius: 0; }
  .mapmode button:first-child { border-radius: 3px 0 0 3px; }
  .mapmode button:last-child { border-radius: 0 3px 3px 0; margin-left: -1px; }
  .mapmode button[aria-pressed='true'] { background: var(--ink); color: var(--paper); border-color: var(--ink); }
  .maplegend { font-size: var(--t-xs); color: var(--ink-soft); }
  .subs { flex-basis: 100%; font-size: var(--t-sm); color: var(--ink-soft); display: flex; gap: 0.35rem; align-items: center; }
  .wavenote { margin: 0; color: var(--ink-soft); flex-basis: 100%; font-size: var(--t-xs); }
  .scrub input { width: 12rem; max-width: 50vw; }
  .loading { color: var(--ink-soft); padding: 3rem 0; }

  .tabs { display: flex; flex-wrap: wrap; gap: 0; border-bottom: 1px solid var(--rule); }
  .tabs button { border: none; background: none; border-radius: 0; padding: 0.45rem 0.8rem; color: var(--ink-soft); border-bottom: 2px solid transparent; margin-bottom: -1px; }
  .tabs button.on { color: var(--ink); border-bottom-color: var(--ink); font-weight: 600; }
  .panel { padding: 0.9rem 0.1rem; }
  .tree { padding: 0; margin: 0; }
  .hint { color: var(--ink-soft); font-size: var(--t-sm); max-width: 60ch; }

  footer { display: flex; justify-content: space-between; align-items: center; gap: 1rem; flex-wrap: wrap; padding: 0.5rem 1.25rem 1rem; border-top: 1px solid var(--rule-faint); }
  .status { margin: 0; color: var(--ink-soft); font-size: var(--t-sm); min-height: 1.3em; }
  .files { display: flex; gap: 0.5rem; align-items: center; }
  .file { font-size: var(--t-sm); border: 1px solid var(--rule); border-radius: 3px; padding: 0.3rem 0.7rem; cursor: pointer; background: var(--paper-raised); }
  .file input { position: absolute; opacity: 0; width: 1px; height: 1px; }
  .file:focus-within { outline: 2px solid var(--focus); }

  @media (max-width: 900px) {
    main { grid-template-columns: minmax(0, 1fr); padding: 0.75rem 1rem; }
    .atlas { position: static; }
    header, footer { padding-left: 1rem; padding-right: 1rem; }
  }
</style>
