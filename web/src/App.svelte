<script lang="ts">
  import { onMount } from 'svelte';
  import { Engine, type Pending, type MapLabel, type Concept, type Event, type Frame, type InnovInfo, type InnovSummary, type Snapshot, type TreeNode, type World, type ScenarioEntry, type LanguageEntry } from './lib/engine';
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
  import CampaignPanel from './lib/CampaignPanel.svelte';
  import { langColor } from './lib/palette';
  import SettingsPanel from './lib/SettingsPanel.svelte';

  const engine = new Engine();
  const YEARS = 25;

  let world = $state<World | null>(null);
  let snap = $state<Snapshot | null>(null);
  let events = $state<Event[]>([]);
  let concepts = $state<Concept[]>([]);
  let seed = $state('42');
  // Run settings that differ from the engine defaults (kept in this browser).
  let cfgDefaults = $state<Record<string, number>>({});
  let cfg = $state<Record<string, number>>(loadCfg());
  let showSettings = $state(false);
  function loadCfg(): Record<string, number> {
    try {
      return JSON.parse(localStorage.getItem('isogloss.settings') ?? '{}') ?? {};
    } catch {
      return {};
    }
  }
  function setCfg(o: Record<string, number>) {
    cfg = o;
    try {
      localStorage.setItem('isogloss.settings', JSON.stringify(o));
    } catch {
      /* storage unavailable: settings last for this page */
    }
  }
  let scenarioID = $state('');
  let scenarios = $state<ScenarioEntry[]>([]);
  let languageID = $state('');
  let languages = $state<LanguageEntry[]>([]);
  const chosenScenario = $derived(scenarios.find((s) => s.id === scenarioID));
  const ownLanguage = $derived(languages.find((l) => l.id === chosenScenario?.language)?.name);
  const chosenLanguage = $derived(languages.find((l) => l.id === languageID));
  let selected = $state(-1);
  let showSubs = $state(false);
  let showNames = $state(true);
  let labels = $state<MapLabel[]>([]);
  let mapMode = $state<'langs' | 'polities' | 'word'>('langs');
  interface WordMap { concept: string; variants: { form: string; root: number; count: number; loan?: string; donor?: string }[]; hex: number[] }
  let wordMap = $state<WordMap | null>(null);
  let mapBy = $state<'form' | 'root'>('form');
  const SHOWN = 10; // variants with a colour of their own; the rest share grey
  let concept = $state('water');
  let tab = $state<'campaign' | 'tree' | 'word' | 'table' | 'intel' | 'lang' | 'fable' | 'stories' | 'rules' | 'log'>('tree');
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
  let past = $state<Snapshot | null>(null); // the full state at the frame looked at, once loaded
  let viewReq = 0;
  let focusLang = $state(-1);

  const viewing = $derived(viewIdx >= 0 && viewIdx < frames.length ? frames[viewIdx] : null);
  // What the panels show: the past moment once loaded, else the live run.
  const shown = $derived(past ?? snap);
  // While the slider moves only the language frames are drawn.
  const framesOnly = $derived(!!viewing && !past);
  const langOf = $derived.by(() => {
    const n = world ? world.hexes.length : 0;
    const a = new Int32Array(n).fill(-1);
    if (framesOnly) viewing!.lang.forEach((l, i) => (a[i] = l));
    else if (shown) for (const [h, , l] of shown.comms) a[h] = l;
    return a;
  });
  const polityOf = $derived.by(() => {
    if (mapMode !== 'polities' || !shown || !world || framesOnly) return null;
    const a = new Int32Array(world.hexes.length).fill(-1);
    for (const [h, , , p] of shown.comms) a[h] = p;
    return a;
  });
  $effect(() => {
    void shown?.tick;
    const c = concept;
    if (mapMode !== 'word' || !snap) { wordMap = null; return; }
    engine.query<WordMap>('dialectmap', { label: c }).then((m) => (wordMap = m)).catch(() => (wordMap = null));
  });
  // Legend entries: forms, or roots with their forms, each with a colour.
  const legend = $derived.by(() => {
    if (!wordMap) return [];
    if (mapBy === 'form') return wordMap.variants.map((v, i) => ({ key: i, label: v.form, count: v.count, loan: v.loan, donor: v.donor }));
    const roots: { key: number; label: string; count: number; loan?: string; donor?: string }[] = [];
    for (const v of wordMap.variants) {
      let r = roots.find((x) => x.key === v.root);
      if (!r) roots.push((r = { key: v.root, label: '', count: 0, loan: v.loan, donor: v.donor }));
      r.label += (r.label ? ', ' : '') + v.form;
      r.count += v.count;
    }
    return roots.sort((a, b) => a.key - b.key);
  });
  const wordColor = (k: number) => (k < SHOWN ? langColor(k) : null);
  const colorOf = $derived.by(() => {
    if (mapMode !== 'word' || !wordMap || framesOnly) return null;
    const m = wordMap;
    return (h: number) => {
      const i = m.hex[h];
      if (i === undefined || i < 0) return null;
      return wordColor(mapBy === 'form' ? i : m.variants[i].root);
    };
  });
  const capitals = $derived(shown && !framesOnly ? shown.polities.filter((p) => p.fell < 0).map((p) => p.capital) : []);
  const isoSet = $derived(overlay && !viewing ? new Set(overlay.applied) : null);
  const nodes = $derived(new Map<number, TreeNode>((shown?.tree ?? []).map((n) => [n.id, n])));
  const living = $derived(shown ? shown.langs.filter((l) => l.died < 0) : []);
  const selPolity = $derived.by(() => {
    if (!shown || selected < 0) return null;
    const c = shown.comms.find((x) => x[0] === selected);
    const p = c && c[3] >= 0 ? shown.polities.find((q) => q.id === c[3] && q.fell < 0) : undefined;
    return p ? { id: p.id, name: p.name } : null;
  });
  const selLang = $derived(selected >= 0 ? langOf[selected] : -1);
  const shownLang = $derived(focusLang >= 0 ? focusLang : selLang);

  function accept(s: Snapshot, reset = false) {
    snap = s;
    events = reset ? s.events : [...events, ...s.events];
    status = '';
  }

  // Interventions queued for coming generations (the engine applies them all).
  let pending = $state<Pending[]>([]);
  async function refreshPending() {
    pending = await engine.query<Pending[]>('pending').catch(() => []);
  }
  function scheduled(msg: string) {
    status = msg;
    refreshPending();
  }
  async function cancelPending(idx: number) {
    await engine.call('cancel', { idx }).catch((e) => (status = e.message ?? String(e)));
    await refreshPending();
  }

  async function refreshSide() {
    await refreshPending();
    innovs = (await engine.query<InnovSummary[]>('innovs', { min: 4 })).slice(0, 60);
    frames = await engine.query<Frame[]>('frames');
    await refreshLabels();
    if (overlay) overlay = await engine.query<InnovInfo>('innov', { id: overlay.id });
  }

  async function refreshLabels() {
    labels = showNames ? await engine.query<MapLabel[]>('labels', { max: 14 }) : [];
  }

  async function newRun() {
    running = false;
    busy = true;
    status = 'Generating a continent and its proto-language…';
    try {
      const r = await engine.call<{ world: World; snapshot: Snapshot }>('init', {
        seed: seed.trim() || '1', scenario: scenarioID, config: { ...cfg },
        language: chosenScenario?.fixedLanguages ? '' : languageID,
      });
      labels = []; // the old world's hexes
      wordMap = null;
      selected = -1;
      world = r.world;
      accept(r.snapshot, true);
      if (r.snapshot.campaign) tab = 'campaign';
      else if (tab === 'campaign') tab = 'tree';
      selected = -1;
      overlay = null;
      viewIdx = -1;
      past = null;
      viewReq++;
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
    past = null;
    viewReq++;
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
    status = `Continuing from year ${viewing.tick * YEARS}…`;
    const r = await engine.call<{ world: World; snapshot: Snapshot }>('fork', { tick: viewing.tick });
    labels = [];
    world = r.world;
    accept(r.snapshot, true);
    viewIdx = -1;
    past = null;
    viewReq++;
    await refreshSide();
    status = `Forked at year ${r.snapshot.tick * YEARS}. Your later actions were dropped; the scenario's own events still come.`;
    busy = false;
  }

  // Loads the full state of the frame looked at (or returns to the live run).
  async function loadView() {
    const req = ++viewReq;
    const t = viewing ? viewing.tick : -1;
    if (t >= 0) status = `Rebuilding year ${t * YEARS}…`;
    try {
      const s = await engine.call<Snapshot>('view', { tick: t });
      if (req !== viewReq) return;
      past = t >= 0 ? s : null;
      status = '';
      await refreshLabels();
    } catch (e: any) {
      if (req === viewReq) status = e.message ?? String(e);
    }
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
      labels = [];
      world = r.world;
      seed = String(sv.save?.Seed ?? sv.seed ?? seed);
      accept(r.snapshot, true);
      viewIdx = -1;
      past = null;
      viewReq++;
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
    cfgDefaults = await engine.query<Record<string, number>>('config');
    concepts = await engine.query<Concept[]>('concepts');
    scenarios = await engine.query<ScenarioEntry[]>('scenarios');
    languages = await engine.query<LanguageEntry[]>('languages');
    cells = await engine.query<Record<string, string[]>>('cells');
    await newRun();
  });

  const allTabs = [
    ['campaign', 'Campaign'], ['tree', 'Family tree'], ['word', 'Word history'], ['table', 'Word lists'], ['intel', 'Intelligibility'],
    ['lang', 'Grammar'], ['fable', 'Parallel text'], ['stories', 'Stories'], ['rules', 'Change rules'], ['log', 'Chronicle'],
  ] as const;
  const tabs = $derived(allTabs.filter(([id]) => id !== 'campaign' || snap?.campaign));
</script>

<header>
  <h1>Isogloss</h1>
  {#if snap}
    <p class="where">
      <span class="proto">{snap.proto}</span>
      {#if viewing}<strong> as of year {viewing.tick * YEARS}</strong>{#if framesOnly} <span class="soft">(map only)</span>{/if}{:else} · year {snap.tick * YEARS}, generation {snap.tick}{/if}
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
    <label title={chosenScenario?.fixedLanguages ? 'This scenario sets the language of each family.' : chosenLanguage ? `${chosenLanguage.summary} ${chosenLanguage.forms} words from ${chosenLanguage.source}; the rest are coined.` : 'A proto-language invented for this run, or a real one to start from.'}>Language
      <select bind:value={languageID} aria-label="Proto-language" disabled={chosenScenario?.fixedLanguages}>
        <option value="">{chosenScenario?.fixedLanguages ? 'set by the scenario' : ownLanguage ?? 'invented'}</option>
        {#each languages as l (l.id)}<option value={l.id}>{l.name}</option>{/each}
      </select>
    </label>
    <button onclick={() => (showSettings = !showSettings)} aria-expanded={showSettings}>Settings{Object.keys(cfg).length ? ` (${Object.keys(cfg).length} changed)` : ''}</button>
    <button onclick={newRun} disabled={busy && !running}>New world</button>
    <span class="sep"></span>
    <button onclick={() => step(1)} disabled={busy || running}>Next generation{pending.length ? ` (${pending.length} queued)` : ''}</button>
    <button onclick={() => step(4)} disabled={busy || running}>+100 years</button>
    <button class="primary" onclick={play} aria-pressed={running}>{running ? 'Pause' : 'Run'}</button>
    <label class="speed">Speed
      <input type="range" min="1" max="10" bind:value={speed} aria-label="Generations per second" />
    </label>
  </div>
  {#if showSettings}
    <SettingsPanel defaults={cfgDefaults} overrides={cfg} onchange={setCfg} />
  {/if}
</header>

<main>
  <section class="atlas" aria-label="Map">
    {#if world}
      <Atlas {world} {langOf} {selected} isogloss={isoSet} level={!viewing ? overlay?.level : null} substrates={showSubs} {polityOf} {capitals} routes={framesOnly ? [] : (shown?.routes ?? [])} labels={framesOnly ? [] : labels} {colorOf} onselect={select} />
      <div class="under">
        {#if selected >= 0}<PlaceCard {engine} hex={selected} tick={shown?.tick ?? 0} />{/if}
        <div class="seg mapmode" role="group" aria-label="Map colours">
          <button aria-pressed={mapMode === 'langs'} onclick={() => (mapMode = 'langs')}>Languages</button>
          <button aria-pressed={mapMode === 'polities'} onclick={() => (mapMode = 'polities')}>Polities</button>
          <button aria-pressed={mapMode === 'word'} onclick={() => (mapMode = 'word')}>A word</button>
        </div>
        {#if mapMode === 'word'}
          <div class="wordmap">
            <label>Word <input list="map-concepts" value={concept} size="10" onchange={(e) => { concept = e.currentTarget.value.trim(); cell = -1; }} aria-label="Word to map" /></label>
            <datalist id="map-concepts">{#each concepts as c (c.id)}<option value={c.label}>{c.gloss}</option>{/each}</datalist>
            <span class="seg">
              <button aria-pressed={mapBy === 'form'} onclick={() => (mapBy = 'form')}>by form</button>
              <button aria-pressed={mapBy === 'root'} onclick={() => (mapBy = 'root')}>by root</button>
            </span>
            {#if wordMap}
              <span class="maplegend">{wordMap.variants.length} {wordMap.variants.length === 1 ? 'form' : 'forms'} of ‘{wordMap.concept}’; grey: rarer forms or no word</span>
              <ul class="wlegend">
                {#each legend.slice(0, SHOWN) as e (e.key)}
                  <li title={e.loan ? (e.loan === 'substrate' ? `from the ${e.donor} substrate` : `borrowed from ${e.donor}`) : undefined}>
                    <span class="sw" style:background={wordColor(e.key)}></span><span class="ipa">{e.label}</span> <span class="n">{e.count}</span>{#if e.loan}<span class="n"> ({e.loan === 'substrate' ? 'substrate' : 'loan'})</span>{/if}
                  </li>
                {/each}
                {#if legend.length > SHOWN}<li class="n">+{legend.length - SHOWN} more</li>{/if}
              </ul>
            {:else}
              <span class="maplegend">No such word: pick one from the list.</span>
            {/if}
          </div>
        {/if}
        <span class="maplegend">○ capital · ┈ trade route{mapMode === 'polities' ? ' · grey: no polity' : ''}</span>
        <label><input type="checkbox" bind:checked={showNames} onchange={refreshLabels} /> Place names</label>
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
              oninput={(e) => { const v = +e.currentTarget.value; viewIdx = v >= frames.length ? -1 : v; past = null; }}
              onchange={loadView}
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
      {#if snap && shown}
        {#if tab === 'campaign'}
          <CampaignPanel {engine} tick={shown.tick} hex={selected} onscheduled={scheduled} oninnov={showInnov} />
        {:else if tab === 'stories'}
          {#await import('./lib/StoryPanel.svelte') then { default: StoryPanel }}
            <StoryPanel {engine} seed={snap.seed} tick={shown.tick} langs={living} polities={shown.polities.filter((p) => p.fell < 0)} />
          {/await}
        {:else if tab === 'tree'}
          {#if nodes.get(0)}
            <ul class="tree"><TreeItem node={nodes.get(0)!} {nodes} langs={shown.langs} proto={shown.proto}
              onlang={(id) => { focusLang = id; tab = 'lang'; }} oninnov={showInnov} /></ul>
          {/if}
          <p class="hint">Subgroups are defined by the changes all their members share (sounds, grammar, word order), as in the comparative method. Click a change to draw its isogloss.</p>
        {:else if tab === 'word'}
          <WordPanel {engine} hex={selected} tick={shown.tick} {concepts} {concept} {cell} {cells}
            onconcept={(c) => { concept = c; cell = -1; }} oncell={(k) => (cell = k)} />
        {:else if tab === 'table'}
          <TablePanel {engine} tick={shown.tick} langs={living} lang={shownLang} onpick={(c) => { concept = c; cell = -1; tab = 'word'; }} />
        {:else if tab === 'intel'}
          <IntelPanel {engine} tick={shown.tick} onlang={(id) => { focusLang = id; tab = 'lang'; }} />
        {:else if tab === 'lang'}
          <LangPanel {engine} lang={shownLang} tick={shown.tick} oninnov={showInnov}
            oncell={(c, k) => { concept = c; cell = k; if (selected < 0 || langOf[selected] !== shownLang) selected = living.find((l) => l.id === shownLang)?.centre ?? selected; tab = 'word'; }} />
        {:else if tab === 'fable'}
          <FablePanel {engine} tick={shown.tick} onlang={(id) => { focusLang = id; tab = 'lang'; }} />
        {:else if tab === 'rules'}
          <RulePanel {engine} hex={selected} tick={shown.tick} polity={selPolity} onscheduled={scheduled} {pending} oncancel={cancelPending} />
        {:else}
          <Chronicle events={past ? events.filter((e) => e.tick <= past!.tick) : events} oninnov={showInnov} onhex={select} />
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
  .where .soft { font-size: var(--t-xs); }
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
  .mapmode button:last-child { border-radius: 0 3px 3px 0; }
  .mapmode button + button { margin-left: -1px; }
  .mapmode button[aria-pressed='true'] { background: var(--ink); color: var(--paper); border-color: var(--ink); }
  .maplegend { font-size: var(--t-xs); color: var(--ink-soft); }
  .wordmap { flex-basis: 100%; display: flex; flex-wrap: wrap; gap: 0.4rem 0.8rem; align-items: center; }
  .wordmap .seg { display: inline-flex; }
  .wordmap .seg button { border-radius: 0; padding: 0.15rem 0.5rem; }
  .wordmap .seg button:first-child { border-radius: 3px 0 0 3px; }
  .wordmap .seg button:last-child { border-radius: 0 3px 3px 0; margin-left: -1px; }
  .wordmap .seg button[aria-pressed='true'] { background: var(--ink); color: var(--paper); border-color: var(--ink); }
  .wlegend { flex-basis: 100%; list-style: none; padding: 0; margin: 0; display: flex; flex-wrap: wrap; gap: 0.2rem 0.9rem; font-size: var(--t-sm); }
  .wlegend .sw { display: inline-block; width: 0.75rem; height: 0.75rem; border-radius: 2px; margin-right: 0.3rem; vertical-align: -0.05rem; }
  .wlegend .n { color: var(--ink-soft); font-size: var(--t-xs); }
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
