<script lang="ts">
  import type { Engine, Fable, Lang } from './engine';
  import { MODELS, loadSettings, provider, saveSettings, type StorySettings } from './llm';
  import { annotations, chronicle, key, nameLanguages, tellMyth, type Chronicle, type Myth, type NameNote } from './story';
  import Gloss from './Gloss.svelte';
  import { langColor } from './palette';

  interface Props { engine: Engine; seed: string; tick: number; langs: Lang[] }
  let { engine, seed, tick, langs }: Props = $props();

  let settings = $state<StorySettings>(loadSettings());
  let draftKey = $state(loadSettings().apiKey);
  let notes = $state(new Map<string, string>());
  let busy = $state('');
  let error = $state('');
  let theme = $state('');
  let shown = $state<{ myth: Myth; text: Fable } | null>(null);
  const llm = $derived(provider(settings));
  const ERA = 10; // generations per era (250 years)

  $effect(() => {
    void tick;
    annotations(engine).then((m) => (notes = m));
  });

  function save() {
    settings = { ...settings, apiKey: draftKey.trim() };
    saveSettings(settings);
  }
  function forget() {
    draftKey = '';
    settings = { ...settings, apiKey: '' };
    saveSettings(settings);
  }
  async function run(label: string, f: () => Promise<unknown>) {
    error = '';
    busy = label;
    try {
      await f();
      notes = await annotations(engine);
    } catch (e: any) {
      error = e.message ?? String(e);
    }
    busy = '';
  }
  const parse = <T,>(k: string): T | null => {
    const v = notes.get(k);
    return v ? (JSON.parse(v) as T) : null;
  };
  const eras = $derived(Array.from({ length: Math.ceil(Math.max(tick, 1) / ERA) }, (_, i) => [i * ERA, Math.min((i + 1) * ERA, tick + 1)]));
  const myths = $derived(
    [...notes.keys()].filter((k) => k.startsWith(seed + '/myth/')).sort().map((k) => ({ k, m: parse<Myth>(k)! })),
  );
  async function show(m: Myth) {
    error = '';
    try {
      shown = { myth: m, text: await engine.query<Fable>('rendertext', { clauses: m.clauses }) };
    } catch (e: any) {
      error = e.message ?? String(e);
    }
  }
</script>

<details class="settings" open={!settings.apiKey}>
  <summary>Storyteller {llm ? `· on (${llm.label})` : '· off'}</summary>
  <p class="note">
    An optional AI storyteller names the peoples, writes chronicles of each era and tells myths. It never changes the simulation:
    the engine builds every name from each language's own words and renders every myth into every language itself.
    Answers are kept in the run's save file, so it is asked once. Use your own Anthropic API key; it is sent only to the
    Anthropic API, from this browser, and is kept in this tab unless you ask to remember it.
  </p>
  <div class="row">
    <label>API key <input type="password" bind:value={draftKey} autocomplete="off" spellcheck="false" placeholder="sk-ant-…" /></label>
    <label>Model
      <select bind:value={settings.model} onchange={() => saveSettings(settings)}>
        {#each MODELS as m (m.id)}<option value={m.id}>{m.label}</option>{/each}
      </select>
    </label>
    <label class="chk"><input type="checkbox" bind:checked={settings.remember} /> remember on this device</label>
    <button onclick={save}>Save</button>
    {#if settings.apiKey}<button onclick={forget}>Forget key</button>{/if}
  </div>
</details>

{#if error}<p class="warn">{error}</p>{/if}
{#if busy}<p class="note">{busy}…</p>{/if}

<section>
  <h3>Peoples</h3>
  <table class="names">
    <thead><tr><th scope="col">Language</th><th scope="col">They call themselves</th><th scope="col">Meaning</th><th scope="col">In English</th></tr></thead>
    <tbody>
      {#each langs as l (l.id)}
        {@const n = parse<NameNote>(key(seed, `name/lang/${l.id}`))}
        <tr>
          <th scope="row"><span class="sw" style:background={langColor(l.id)}></span>{l.name}</th>
          {#if n}
            <td><strong>{n.native}</strong> <span class="ipa soft">{n.form}</span></td>
            <td class="soft">‘{n.meaning}’</td>
            <td title={n.note}>{n.exonym}</td>
          {:else}<td class="soft" colspan="3">not named yet</td>{/if}
        </tr>
      {/each}
    </tbody>
  </table>
  <button disabled={!llm || !!busy} onclick={() => run('Naming the peoples', () => nameLanguages(engine, llm!, seed, langs, notes))}>Name the peoples</button>
</section>

<section>
  <h3>Chronicles</h3>
  <ol class="eras">
    {#each eras as [from, to] (from)}
      {@const c = parse<Chronicle>(key(seed, `era/${from}-${to}`))}
      <li>
        <span class="yrs">Years {from * 25}–{Math.min(to, tick) * 25}</span>
        {#if c}
          <h4>{c.title}</h4>
          <p class="prose">{c.text}</p>
        {:else}
          <button disabled={!llm || !!busy} onclick={() => run('Writing the chronicle', () => chronicle(engine, llm!, seed, from, to))}>Write this era's chronicle</button>
        {/if}
      </li>
    {/each}
  </ol>
</section>

<section>
  <h3>Myths</h3>
  <div class="row">
    <label>Theme <input bind:value={theme} placeholder="e.g. how the river was made" size="28" /></label>
    <button disabled={!llm || !!busy} onclick={() => run('Telling a myth', async () => show(await tellMyth(engine, llm!, seed, myths.length, theme.trim())))}>Tell a myth</button>
  </div>
  {#if myths.length}
    <ul class="mythlist">
      {#each myths as { k, m } (k)}<li><button class="link" onclick={() => show(m)}>{m.title}</button></li>{/each}
    </ul>
  {/if}
  {#if shown}
    <h4>{shown.myth.title}</h4>
    {#each shown.myth.clauses as c, i (i)}
      <div class="clause">
        <p class="en">‘{c.english}’</p>
        {#each shown.text.langs as l (l.id)}
          <div class="ilg">
            <span class="ln">{l.name}</span>
            {#each l.lines[i] as w, k (k)}<span class="w"><span class="f ipa">{w.f}</span><span class="g"><Gloss text={w.g} /></span></span>{/each}
          </div>
        {/each}
      </div>
    {/each}
  {/if}
</section>

<style>
  .settings { border: 1px solid var(--rule); border-radius: 4px; padding: 0.4rem 0.7rem; margin-bottom: 0.8rem; }
  .settings summary { cursor: pointer; font-weight: 600; }
  .row { display: flex; flex-wrap: wrap; gap: 0.5rem 0.8rem; align-items: center; margin: 0.4rem 0; }
  label { font-size: var(--t-sm); color: var(--ink-soft); display: inline-flex; gap: 0.35rem; align-items: center; }
  .note, .soft { color: var(--ink-soft); font-size: var(--t-sm); }
  .warn { color: var(--isogloss); }
  h3 { font-family: var(--serif); font-weight: 700; font-size: var(--t-lg); margin: 1.2rem 0 0.4rem; }
  h4 { font-family: var(--serif); font-weight: 400; font-style: italic; font-size: var(--t-lg); margin: 0.2rem 0; }
  .names { border-collapse: collapse; font-size: var(--t-sm); margin-bottom: 0.5rem; }
  .names th, .names td { padding: 0.2rem 0.8rem 0.2rem 0; text-align: left; border-bottom: 1px solid var(--rule-faint); }
  .names thead th { color: var(--ink-soft); font-weight: 500; }
  .sw { display: inline-block; width: 0.7rem; height: 0.7rem; border-radius: 2px; margin-right: 0.35rem; }
  .eras { list-style: none; padding: 0; margin: 0; }
  .eras li { padding: 0.4rem 0; border-bottom: 1px solid var(--rule-faint); }
  .yrs { font-size: var(--t-xs); color: var(--ink-soft); text-transform: uppercase; letter-spacing: 0.05em; display: block; }
  .prose { font-family: var(--serif); line-height: 1.55; margin: 0.2rem 0; max-width: 42rem; }
  .mythlist { padding-left: 1.1rem; margin: 0.3rem 0; }
  .clause { padding: 0.4rem 0; border-top: 1px solid var(--rule-faint); }
  .en { font-family: var(--serif); font-style: italic; margin: 0 0 0.3rem; }
  .ilg { display: flex; flex-wrap: wrap; gap: 0.2rem 0.9rem; align-items: flex-start; margin: 0.15rem 0; }
  .ln { min-width: 8rem; font-size: var(--t-sm); color: var(--ink-soft); }
  .w { display: inline-flex; flex-direction: column; }
  .g { font-size: var(--t-xs); color: var(--ink-soft); }
  .link { background: none; border: none; padding: 0; cursor: pointer; text-align: left; }
  .link:hover { text-decoration: underline; }
</style>
