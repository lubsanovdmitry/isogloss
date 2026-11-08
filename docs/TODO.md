# What remains

Status as of 2026-11-08: **M0 and M1 are done** (gates met; see
[IMPLEMENTATION.md](IMPLEMENTATION.md)). This list is ordered by when it is
needed.

## M1 follow-ups (small, worth doing before M2)

- [ ] Push to a remote and confirm CI is green on all three jobs. The arm64 golden digest has not run yet: only amd64 and wasm have been checked locally.
- [ ] Set the real Go module path (it is the placeholder `isogloss`; needs the GitHub owner).
- [ ] List regular sound correspondences in the comparative table (spec: "regular correspondences listed").
- [ ] Isogloss overlay: optionally show the live wave level (`InnovJSON` already returns `level`), and adoption over time (the spec's "Isogloss / wave" view).
- [ ] Intelligibility heatmap view (pairwise matrix of living languages; `Clusterer.Intel` exists).
- [ ] Timeline: snapshots every N ticks for fast scrubbing. Today "look back" shows language maps only, and forking replays from tick 0.
- [ ] Concept frequency classes/stability are hand-curated for ~120 concepts; the rest use defaults. Review `cmd/datagen/concepts_meta.tsv`.
- [ ] Compounding picks heads and modifiers loosely (e.g. 'night' = 'straight'+'year'); constrain by CLICS/field until M3 drift lands.
- [ ] Install binaryen and enable `wasm-opt -Oz` (bundle is 0.9 MB gz without it).
- [ ] Watch `npm audit`: low-severity `deepmerge` advisory via `@sveltejs/vite-plugin-svelte` (dev-only, no fix upstream yet).
- [ ] Tune calibration knobs across many seeds (target ~80% core retention per millennium; 4–12 languages). Add a seed-sweep test that reports distributions.

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
