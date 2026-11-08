# What remains

Status as of 2026-11-08: **M0, M1 and M2 are done** (gates met; see
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

## M3 · Meaning and contact

- [ ] Polysemy: lexemes with many senses; drift along CLICS edges (`engine/data/colex.tsv` is loaded by `lex.Colex`).
- [ ] Homophony clashes queue replacements.
- [ ] Borrowing with nativization (feature-distance mapping, phonotactic repair, stress/tone), loan strata (loans only undergo later changes), substrate packages, toponyms.
- [ ] Gate: loan strata and doublets show up in the comparative table.

## M4 · Society and scenarios

- [ ] Polities, migration, conquest outcomes, prestige and standards, diglossia, writing and spelling (romanized grapheme names), trade routes, seafaring.
- [ ] Scenario files (YAML) and the five starter scenarios.
- [ ] Gate: the Empire collapse scenario produces a dialect continuum (clustering already reports continua).

## M5 · Campaign and LLM

- [ ] Steering campaign: influence, goals, score.
- [ ] LLM layer in `web/`: names, chronicles, myths as abstract text trees; annotation cache keyed by (seed, event id, prompt version) stored in the save; phonotactic validation of names.

## Extension points (after M5)

Reconstruction puzzle mode, full syntax renderer, sprachbund convergence,
pidgins and creoles, espeak-ng audio, agent-based speakers, phylogenetic
validation on the CLDF export (`isogloss export --cldf` already writes the true
tree as `tree.nwk`).
