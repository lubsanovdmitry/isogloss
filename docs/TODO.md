# What remains

Status as of 2026-11-09: **M0–M5 are done** (gates met; see
[IMPLEMENTATION.md](IMPLEMENTATION.md)), and the follow-ups below are done
except pushing to a remote and the module path. This list is ordered by when
it is needed.

## M1 follow-ups (small, worth doing before M2)

- [ ] Push to a remote and confirm CI is green on all three jobs. The arm64 golden digest has not run yet: only amd64 and wasm have been checked locally.
- [ ] Set the real Go module path (it is the placeholder `isogloss`; needs the GitHub owner).
- [x] List regular sound correspondences in the comparative table (Word lists → Sound correspondences; `render.Correspondences`).
- [x] Isogloss overlay shows the live wave level, plus an adoption-over-time chart (`Innovation.Curve`).
- [x] Intelligibility heatmap view (Intelligibility tab; `render.IntelJSON`).
- [x] Timeline: `sim.Timeline` keeps clones (every 4 generations, thinned to at most ten); looking back shows every panel as it was, and forking continues from a snapshot. Forks keep a scenario's own later events.
- [x] Concept frequency classes/stability: all of Swadesh-207 and the kin terms are curated (Leipzig–Jakarta list for the stable class); grammar sources and cultural items keep role defaults.
- [x] Compounds: only things are compounded; heads are CLICS neighbours (else same field), modifiers a small classifier set.
- [x] `wasm-opt -Oz` (binaryen 117 from emsdk) saves only 0.5% gzipped on the Go bundle; not worth a dependency. The Makefile still uses it when it is on PATH.
- [ ] Watch `npm audit`: low-severity `deepmerge` advisory via `@sveltejs/vite-plugin-svelte` (dev-only, no fix upstream yet; still open 2026-11-09).
- [x] Seed sweep (`TestSeedSweep`, 16 seeds): median 5 languages (2–10), median core retention 787‰. Knobs left as they were.

## M2 · Grammar (done)

- [x] `engine/morph`: 16 slots, six stages, cells with tagged boundaries, WALS word order.
- [x] Grammaticalization as spreading innovations; loss detected by syncretism; renewal.
- [x] Analogy (levelling + extension to the productive pattern), rate ∝ 1/frequency.
- [x] Harmonic word-order drift; case loss pushes toward SVO.
- [x] Typology metrics; views: paradigms, grammar sketch, parallel text, inflected-form etymologies; `isogloss grammar` Markdown export.
- [x] Gate: `TestIrregularParadigmsEmerge`.

### M2 follow-ups

- [x] Two segments with one symbol: sound changes now leave canonical bundles (`phon.Canon`), `RenderFeatures` finds exact base+diacritic spellings (tie bar for labial-velars), and `phon.Labels` marks anything left. No collisions in seeds 1–30. Rules built from segments use `phon.Spell`, so mergers no longer miss.
- [x] Future, perfect and progressive agree with every person in the parallel text and myths (3SG agreement swapped for the person's own; still only 3SG is stored).
- [x] Definite + oblique: rendered with the article inside the case marker (stem-DEF-CASE), from the stored definite form (not stored as cells).
- [x] Periphrastic auxiliaries built on verbs agree with the subject; the main verb stays plain.
- [x] Performance: single-word memo keys, remembered MayApply misses, memoised `RegularForms`, GC tuning in the worker. Default run ~1.75 s native, ~4.0 s in the browser; Empire collapse 9.6 → 5.2 s native per 1000 years (analogy now runs at realistic rates, which costs back part of the gain).
- [x] Syncretism: loss at two thirds of cells (14% of languages lose a slot in 1000 years, was 9%). Still slower than real attrition (Latin lost most noun case in ~800 years): affix erosion is the driver, not the threshold.

## M3 · Meaning and contact (done)

- [x] Polysemy as shared words across concept slots; drift along CLICS edges in two steps (gain a sense, then the shift).
- [x] Homophony clashes queue replacements.
- [x] Cultural items unknown to the proto-language are invented and spread as words and loans.
- [x] Borrowing with nativization, loan strata, substrate packages, place names; a language barrier for words.
- [x] Gate: `TestLoanStrataAndDoublets`.

### M3 follow-ups

- [x] Capitals borrow from foreign capitals within ten hexes or along routes (court loans). Loans between languages still need two languages: a family that splits late has none before it splits (as it should).
- [x] A language is recorded at the capital of its largest polity (else its centre); with court loans and standards, loans at reference varieties +40% (29% → 37% of a language's loans).
- [x] Calques: compound roots record their parts; a borrowed compound is translated part by part 35% of the time.
- [x] Substrates can pass on a sound the family lacked (20% of packages): the nearest phoneme becomes it in one context, leaving a new contrast.
- [x] Place names on the map (capitals and the largest communities, three hexes apart, a toggle); names of four or more syllables are worn down to three when a community takes them over.
- [x] Polysemy measured: median 16% of concepts (9–24%) at 1000 years and no higher at 3000; `TestPolysemyInRange` keeps it there.

## M4 · Society and scenarios (done)

- [x] Polities, migration, conquest outcomes (local ruler/local ratio), prestige, trade routes, seafaring, famine, plague.
- [x] Writing (alphabet, abjad, syllabary), frozen spelling and reforms, standards (top-down spread from the capital, norm damps local innovations), diglossia and learned loans.
- [x] Several language families (namespaced roots); scenario files (YAML subset, `engine/scenario`), five starters, `isogloss run --scenario`, scenario picker and sandbox events in the UI.
- [x] Gate: `TestEmpireCollapseContinuum` (seeds 7–9: 72–100% understand the capital under the empire, 22–30% 800 years after the fall, continua of 7–10 languages).
- [x] `make test-wasm` passes. The intermittent wasm crash was out of memory: the intern table kept every intermediate variety between clusterings (now pruned each generation; peak live heap 422 → 121 MB in Empire collapse), and long statistical tests fragment wasm memory, which never shrinks, so they are skipped under wasm (`heavy`); the golden digests still run there.
- [x] Empire collapse: 5.2 s native per 1000 years (was 9.6).
- [x] Island hopping: a crossing loses its rarest sound and half the time the next; 80% of communities with two or more founder losses still lack most of those sounds at the end. Mean inventories are not clearly smaller: mainland simplification drowns it out.

## M5 · Campaign and LLM

- [x] Steering campaign: `campaign:` section in scenario files (people, income, start, goals: spoken, unity, tonal, daughters, survive); influence from population, writing and a standard; paid actions (script, standard, seafaring, route, encourage/suppress a change in your land, expand, conquer) charged when they happen and refused when unaffordable; score; Campaign tab; two campaign starters (The river people, Song of the delta).
- [x] Storyteller in `web/`: names picked from engine-built candidates, era chronicles, myths as abstract text trees rendered by the engine; annotations keyed by (seed, job, prompt version) stored in the save.
- [x] Storyteller end to end in a browser against a mocked API (`npm run e2e`): names, realms, a chronicle and a myth, and the request shape. It found and fixed a bug: naming never sent a request.
- [ ] One storyteller session with a real API key (names, a chronicle, a myth), to confirm the live API accepts the requests the mock checks.
- [x] Polity names from the storyteller (Stories tab, Realms).
- [x] Campaign balance: `TestCampaignBalance` plays strategies over ten seeds. Income cut ~4× (it bought everything), goals made harder (idle met 75%), steering now acts on a change's arrival too. River people: idle 50% of goals, balanced 75%; delta: tonal in 3/10 runs idle, 5/10 steered.

## Extension points (after M5)

Reconstruction puzzle mode, full syntax renderer, sprachbund convergence,
pidgins and creoles, espeak-ng audio, agent-based speakers, phylogenetic
validation on the CLDF export (`isogloss export --cldf` already writes the true
tree as `tree.nwk`).
