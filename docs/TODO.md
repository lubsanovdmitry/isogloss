# What remains

Status as of 2026-11-08: **M0 and M1 are done** (gates met; see
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

## M2 · Grammar

- [ ] `engine/morph`: categories (noun number/case/definiteness; verb TAM, mood, agreement), exponents (free word, clitic, prefix, suffix, stem change, zero).
- [ ] Paradigm cells built as stem + exponents with `+`/`=` boundaries so sound rules cross them (rule engine already supports boundaries).
- [ ] Grammaticalization cycle (6 stages) as innovations (`history.Gram`), with Heine & Kuteva paths over the gram concepts already in the lexicon.
- [ ] Analogy: levelling and extension, probability ∝ 1/frequency.
- [ ] Word-order parameters with WALS priors and correlations (`engine/data/wals.tsv` is already generated) and harmonic drift.
- [ ] Typology metrics (synthesis, fusion, irregularity).
- [ ] Views: paradigms (irregular cells highlighted), parallel texts from abstract trees, grammar sketch (extend `render.LangJSON`).
- [ ] Gate: irregular paradigms emerge in seeded runs (test: stem allomorphy in a frequent lexeme).

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
