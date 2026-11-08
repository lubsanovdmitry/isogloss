# Implementation notes

Decisions made while building M0–M1 that refine or depart from the spec.

## Measured gates

| Gate | Result |
| --- | --- |
| M0: `isogloss sca` reproduces fixture sound laws | Grimm + Verner, West Germanic i-umlaut, Latin → Spanish, Slavic first palatalization, toy tonogenesis all pass |
| M1: 1000 years in under 5 s in the browser | ~1.8 s in headless Chromium (seed 42, ~280 communities); ~0.6 s native |
| M1: replay hash stable | Golden digests for seeds 1, 42, 1337 match on amd64 and wasm; CI adds arm64 |
| WASM bundle | ~0.9 MB gzipped (standard Go, `-s -w`, no wasm-opt) |

## Departures from the spec

- **Clustering threshold is 0.49, not ~0.7.** Calibrated on IE-CoR Romance word
  lists (`engine/pop/intel_test.go`): Spanish–Portuguese scores 0.54,
  Spanish–Italian 0.44, Spanish–Romanian 0.22, Spanish–French 0.09. The metric's
  zero point (0.39) is the measured similarity of unrelated word pairs.
- **Stress lives on the vowel**, like tone, instead of a per-syllable array, so
  lexical stress survives deletions. `Word.Stress()` still gives the per-syllable view.
- **Phonation is the glottal features** of the nucleus (spread = breathy,
  constricted = creaky), not a separate field.
- **Affricates are written with a tie bar** (t͡s) or as a ligature (ʦ); plain
  `ts` is two segments.
- **Expansion from a homeland** is in M1: communities grow and colonise empty
  neighbouring land, carrying their variety and in-progress waves. Without it
  every hex starts identical and trees are uninteresting. Migration and
  conquest proper are still M4.
- **Word replacement in M1** uses three sources: a meaning shift along a CLICS
  edge (the neighbour concept's word), a compound of native words, or a fresh
  root. Polysemy bookkeeping (many-to-many senses) arrives with M3.
- **Etymologies are replayed, not stored.** A variety's history is a persistent
  list of entries; replaying it on the proto word rebuilds the word exactly
  (tested). Diffusing changes record which frequency classes they reached, and
  template relaxations are history entries too, so replay is exact.
- **Golden test hashes a state digest** (`Sim.Digest`) rather than SHA-256 of a
  JSON dump; it covers every community's forms, etymologies, population and
  language plus the language list.

## Calibration knobs (`sim.DefaultConfig`)

Sound-change proposal rate 60‰ per community per generation; word-replacement
rates by stability class (8–240 per 100k community-generations, scaled ×0.8);
innovations start at 0.2 adoption with a local growth rate in −0.25…0.75 and
spread for 5–15 generations. Across 16 seeds (`TestSeedSweep`) 1000 years give
a median of 5 languages (2–10) and median core retention of 787‰; real-world
retention is ~80% per millennium.

## Known gaps in M1

- Homophony clashes do not yet trigger replacement (M3).
- Regular correspondences are counted over core words that *every* living
  language inherited; with many languages that set gets small.
