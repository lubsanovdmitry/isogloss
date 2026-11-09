<script lang="ts">
  // Run settings: named engine config values (see sim.ApplySetting). Only
  // the ones that differ from the defaults are kept; they apply when a new
  // world is made and are saved with the run.
  interface Props { defaults: Record<string, number>; overrides: Record<string, number>; onchange: (o: Record<string, number>) => void }
  let { defaults, overrides, onchange }: Props = $props();

  const MULT = [0.25, 0.5, 0.75, 1, 1.5, 2, 3];
  const rates = [
    ['sound-rate', 'Sound changes'],
    ['lex-rate', 'Word replacement'],
    ['sem-rate', 'Meaning shifts'],
    ['loan-rate', 'Borrowing'],
    ['gram-rate', 'Grammaticalization'],
    ['analogy-rate', 'Analogy (irregular forms regularize)'],
    ['order-rate', 'Word-order change'],
  ] as const;
  const society = [
    ['polity-rate', 'New polities'],
    ['conquest-rate', 'Conquest'],
    ['collapse-rate', 'Collapse'],
    ['script-rate', 'Invention of writing'],
  ] as const;
  const sizes = [
    { id: 'small', label: 'small (~170 land hexes)', width: 20, height: 15, land: 170 },
    { id: 'default', label: 'default (~300)', width: 26, height: 20, land: 300 },
    { id: 'large', label: 'large (~520, slower)', width: 34, height: 26, land: 520 },
    { id: 'huge', label: 'huge (~800, much slower)', width: 42, height: 32, land: 800 },
  ];

  const val = (k: string) => overrides[k] ?? defaults[k] ?? 0;
  function set(k: string, v: number) {
    const o = { ...overrides };
    if (v === defaults[k]) delete o[k];
    else o[k] = v;
    onchange(o);
  }
  // A multiplier slider stores default × m.
  const multIndex = (k: string) => {
    const d = defaults[k];
    if (!d) return 3;
    const m = val(k) / d;
    let best = 0;
    MULT.forEach((x, i) => { if (Math.abs(x - m) < Math.abs(MULT[best] - m)) best = i; });
    return best;
  };
  const setMult = (k: string, i: number) => set(k, Math.round((defaults[k] ?? 0) * MULT[i]));
  const size = $derived(sizes.find((z) => z.width === val('width') && z.height === val('height') && z.land === val('land'))?.id ?? 'custom');
  function setSize(id: string) {
    const z = sizes.find((x) => x.id === id);
    if (!z) return;
    const o = { ...overrides };
    for (const k of ['width', 'height', 'land'] as const) {
      if (z[k] === defaults[k]) delete o[k];
      else o[k] = z[k];
    }
    onchange(o);
  }
  const rivers = $derived(val('rivers') || 9); // 0 means the default, 9
</script>

<div class="settings">
  <p class="note">These apply when you press <strong>New world</strong> and are saved with the run. The defaults are calibrated
    (about 80% of core words kept per 1000 years, as in real families); faster rates make languages drift apart sooner.
    Scenarios may override some of them.</p>

  <section>
    <h4>How fast things change</h4>
    {#each rates as [k, label] (k)}
      <label class="sl">
        <span class="lb">{label}</span>
        <input type="range" min="0" max={MULT.length - 1} step="1" value={multIndex(k)} oninput={(e) => setMult(k, +e.currentTarget.value)} />
        <span class="v" class:changed={k in overrides}>×{MULT[multIndex(k)]}</span>
      </label>
    {/each}
  </section>

  <section>
    <h4>What counts as one language</h4>
    <label class="sl">
      <span class="lb">Same language at or above</span>
      <input type="range" min="300" max="700" step="10" value={val('threshold') || 490} oninput={(e) => set('threshold', +e.currentTarget.value === 490 ? 0 : +e.currentTarget.value)} />
      <span class="v" class:changed={'threshold' in overrides}>{Math.round((val('threshold') || 490) / 10)}%</span>
    </label>
    <p class="note">Mutual intelligibility on the Romance-calibrated scale (Spanish–Portuguese 54%, Spanish–Italian 44%). Lower: fewer, broader languages; higher: more, smaller ones.</p>
  </section>

  <section>
    <h4>Society</h4>
    <label class="chk"><input type="checkbox" checked={val('society') !== 0} onchange={(e) => set('society', e.currentTarget.checked ? 1 : 0)} /> polities, conquest, trade and writing</label>
    {#if val('society') !== 0}
      {#each society as [k, label] (k)}
        <label class="sl">
          <span class="lb">{label}</span>
          <input type="range" min="0" max={MULT.length - 1} step="1" value={multIndex(k)} oninput={(e) => setMult(k, +e.currentTarget.value)} />
          <span class="v" class:changed={k in overrides}>×{MULT[multIndex(k)]}</span>
        </label>
      {/each}
    {/if}
  </section>

  <section>
    <h4>World</h4>
    <label class="row">Map
      <select value={size} onchange={(e) => setSize(e.currentTarget.value)}>
        {#each sizes as z (z.id)}<option value={z.id}>{z.label}</option>{/each}
        {#if size === 'custom'}<option value="custom">custom</option>{/if}
      </select>
    </label>
    <label class="row">Shape
      <select value={val('shape')} onchange={(e) => set('shape', +e.currentTarget.value)}>
        <option value={0}>continent</option><option value={1}>archipelago</option><option value={2}>corridor</option>
      </select>
    </label>
    <label class="sl">
      <span class="lb">Rivers</span>
      <input type="range" min="1" max="30" step="1" value={rivers} oninput={(e) => set('rivers', +e.currentTarget.value === 9 ? 0 : +e.currentTarget.value)} />
      <span class="v" class:changed={'rivers' in overrides}>{rivers}</span>
    </label>
    <label class="sl">
      <span class="lb">Earlier languages (substrates)</span>
      <input type="range" min="0" max="6" step="1" value={val('substrates')} oninput={(e) => set('substrates', +e.currentTarget.value)} />
      <span class="v" class:changed={'substrates' in overrides}>{val('substrates')}</span>
    </label>
  </section>

  <div class="foot">
    <button onclick={() => onchange({})} disabled={Object.keys(overrides).length === 0}>Reset to defaults</button>
    {#if Object.keys(overrides).length}<span class="note">{Object.keys(overrides).length} changed</span>{/if}
  </div>
</div>

<style>
  .settings { grid-column: 1 / -1; display: grid; grid-template-columns: repeat(auto-fit, minmax(17rem, 1fr)); gap: 0.4rem 1.6rem;
    border: 1px solid var(--rule); border-radius: 4px; padding: 0.6rem 0.9rem; margin-top: 0.3rem; background: var(--paper-raised); }
  .settings > .note { grid-column: 1 / -1; margin: 0; }
  h4 { margin: 0.2rem 0 0.3rem; font-size: var(--t-md); font-family: var(--serif); }
  .sl { display: grid; grid-template-columns: minmax(8rem, 1fr) 7rem 2.6rem; gap: 0.5rem; align-items: center; font-size: var(--t-sm); color: var(--ink-soft); margin: 0.1rem 0; }
  .sl input { width: 100%; }
  .v { font-variant-numeric: tabular-nums; text-align: right; }
  .v.changed { color: var(--ink); font-weight: 700; }
  .row, .chk { display: flex; gap: 0.4rem; align-items: center; font-size: var(--t-sm); color: var(--ink-soft); margin: 0.15rem 0; }
  .note { font-size: var(--t-xs); color: var(--ink-soft); margin: 0.2rem 0 0; }
  .foot { grid-column: 1 / -1; display: flex; gap: 0.6rem; align-items: center; }
</style>
