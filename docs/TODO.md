# What remains

Status as of 2026-11-08: **M0–M5 are done** (gates met; see
[IMPLEMENTATION.md](IMPLEMENTATION.md)). This list is ordered by when it is
needed.

## M1 follow-ups (small, worth doing before M2)

- [ ] Push to a remote and confirm CI is green on all three jobs. The arm64 golden digest has not run yet: only amd64 and wasm have been checked locally.
- [ ] Set the real Go module path (it is the placeholder `isogloss`; needs the GitHub owner).
- [x] List regular sound correspondences in the comparative table (Word lists → Sound correspondences; `render.Correspondences`).
- [x] Isogloss overlay shows the live wave level, plus an adoption-over-time chart (`Innovation.Curve`).
- [x] Intelligibility heatmap view (Intelligibility tab; `render.IntelJSON`).
- [ ] Timeline: snapshots every N ticks for fast scrubbing. Today "look back" shows language maps only, and forking replays from tick 0.
- [ ] Concept frequency classes/stability are hand-curated for ~120 concepts; the rest use defaults. Review `cmd/datagen/concepts_meta.tsv`.
- [x] Compounds: only things are compounded; heads are CLICS neighbours (else same field), modifiers a small classifier set.
- [x] `wasm-opt -Oz` (binaryen 117 from emsdk) saves only 0.5% gzipped on the Go bundle; not worth a dependency. The Makefile still uses it when it is on PATH.
- [ ] Watch `npm audit`: low-severity `deepmerge` advisory via `@sveltejs/vite-plugin-svelte` (dev-only, no fix upstream yet).
- [x] Seed sweep (`TestSeedSweep`, 16 seeds): median 5 languages (2–10), median core retention 787‰. Knobs left as they were.

## M2 · Grammar (done)

- [x] `engine/morph`: 16 slots, six stages, cells with tagged boundaries, WALS word order.
- [x] Grammaticalization as spreading innovations; loss detected by syncretism; renewal.
- [x] Analogy (levelling + extension to the productive pattern), rate ∝ 1/frequency.
- [x] Harmonic word-order drift; case loss pushes toward SVO.
- [x] Typology metrics; views: paradigms, grammar sketch, parallel text, inflected-form etymologies; `isogloss grammar` Markdown export.
- [x] Gate: `TestIrregularParadigmsEmerge`.

### M2 follow-ups

- [ ] Some inventories show two segments with the same symbol (e.g. two "ʔ"): distinct feature bundles render alike. Make `RenderFeatures` disambiguate, or merge near-identical bundles.
- [ ] Only 3SG cells exist for future, perfect and progressive; the parallel text uses them for every person.
- [ ] Definiteness is not crossed with case in stored cells (the parallel text attaches a bound article outside the case form).
- [ ] Periphrastic auxiliaries are rendered uninflected.
- [ ] Performance: analogy and inflected cells doubled the cost of a run (0.8 → 1.6 s native on one core; 3.7–4.0 s in the browser). Ideas: cheaper productive-pattern updates, sampling cells in `MorphSim`.
- [ ] Full syncretism (loss) is rare within 1000 years; check the 75% threshold against real attrition rates.

## M3 · Meaning and contact (done)

- [x] Polysemy as shared words across concept slots; drift along CLICS edges in two steps (gain a sense, then the shift).
- [x] Homophony clashes queue replacements.
- [x] Cultural items unknown to the proto-language are invented and spread as words and loans.
- [x] Borrowing with nativization, loan strata, substrate packages, place names; a language barrier for words.
- [x] Gate: `TestLoanStrataAndDoublets`.

### M3 follow-ups

- [ ] Loans start only once the family splits, and families stay one language for 700+ years, so runs that split late (2 of 12 seeds) have no loans between languages. Polities, trade and prestige centres (M4) should drive borrowing more than border contact does.
- [ ] Loans enter at borders and many never reach the language's centre, which the table shows. A standard (M4) would carry them.
- [ ] Calques (loan translations of compounds) are not modelled.
- [ ] Substrate phonology is a single merger; feature transfer (e.g. retroflexes) is not modelled.
- [ ] Place names are not shown on the map itself, only for the selected hex; long compound names are common.
- [ ] Polysemy is still high (30–80 concepts per language share a word with another). Real languages colexify perhaps 15–25% of a 300-concept list.

## M4 · Society and scenarios (done)

- [x] Polities, migration, conquest outcomes (local ruler/local ratio), prestige, trade routes, seafaring, famine, plague.
- [x] Writing (alphabet, abjad, syllabary), frozen spelling and reforms, standards (top-down spread from the capital, norm damps local innovations), diglossia and learned loans.
- [x] Several language families (namespaced roots); scenario files (YAML subset, `engine/scenario`), five starters, `isogloss run --scenario`, scenario picker and sandbox events in the UI.
- [x] Gate: `TestEmpireCollapseContinuum` (seeds 7–9: 72–100% understand the capital under the empire, 22–30% 800 years after the fall, continua of 7–10 languages).
- [x] `make test-wasm` passes. The intermittent wasm crash was out of memory: the intern table kept every intermediate variety between clusterings (now pruned each generation; peak live heap 422 → 121 MB in Empire collapse), and long statistical tests fragment wasm memory, which never shrinks, so they are skipped under wasm (`heavy`); the golden digests still run there.
- [ ] Scenario runs that settle the whole map at year 0 are slow: Empire collapse takes ~14 s per 1000 years in the browser (default run ~3.7 s).
- [ ] Island hopping's founder effect is real but small (inventories 1–7 phonemes smaller after 3+ founder losses).

## M5 · Campaign and LLM

- [x] Steering campaign: `campaign:` section in scenario files (people, income, start, goals: spoken, unity, tonal, daughters, survive); influence from population, writing and a standard; paid actions (script, standard, seafaring, route, encourage/suppress a change in your land, expand, conquer) charged when they happen and refused when unaffordable; score; Campaign tab; two campaign starters (The river people, Song of the delta).
- [x] Storyteller in `web/`: names picked from engine-built candidates, era chronicles, myths as abstract text trees rendered by the engine; annotations keyed by (seed, job, prompt version) stored in the save.
- [ ] The storyteller's live API calls have not been exercised end to end (no key in development); the engine side (candidates, era data, tree validation and rendering, annotations through replay) is tested.
- [ ] Polity names from the storyteller (`namecands?kind=polity` exists; the UI names only languages).
- [ ] Campaign balance: costs and income are first guesses; one test plays a few actions.

## Extension points (after M5)

Reconstruction puzzle mode, full syntax renderer, sprachbund convergence,
pidgins and creoles, espeak-ng audio, agent-based speakers, phylogenetic
validation on the CLDF export (`isogloss export --cldf` already writes the true
tree as `tree.nwk`).
