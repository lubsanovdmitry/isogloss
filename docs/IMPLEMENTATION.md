# Implementation notes

Decisions made while building M0–M2 that refine or depart from the spec.

## Measured gates

| Gate | Result |
| --- | --- |
| M0: `isogloss sca` reproduces fixture sound laws | Grimm + Verner, West Germanic i-umlaut, Latin → Spanish, Slavic first palatalization, toy tonogenesis all pass |
| M1: 1000 years in under 5 s in the browser | ~1.8 s in headless Chromium (seed 42, ~280 communities); ~0.6 s native |
| M1: replay hash stable | Golden digests for seeds 1, 42, 1337 match on amd64 and wasm; CI adds arm64 |
| WASM bundle | ~1.0 MB gzipped (standard Go, `-s -w`; wasm-opt saves only 0.5%) |
| M2: irregular paradigms emerge | `TestIrregularParadigmsEmerge`: 8 of 8 seeds have stem allomorphy in a frequent (class ≤ 2) lexeme after 1000 years, and frequent paradigms are more often irregular than rare ones |
| M2: 1000 years in the browser | 3.7–4.0 s in headless Chromium (seed 42; the machine was loaded), under the 5 s gate; ~1.6 s native on one core |

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

## M2 · Grammar

**Representation.** `engine/morph` defines 16 slots: noun plural, four cases
(accusative, genitive, dative, locative) and definiteness; verb past, future,
perfect and progressive, and six person/number agreement markers. Each slot's
exponent has a stage (none, periphrasis, fixed construction, clitic, affix,
lost), a source concept, and a side. Fusion (stage 5) is not stored: it is
measured, because it is what sound change does to an affix.

**Paradigms are stored, not generated.** Thirty "tracked" lexemes (Swadesh
nouns and verbs, up to six per frequency class) keep every cell form: 12 noun
cells (number × case, plus definite) and 15 verb cells (person × present/past,
plus future, perfect and progressive in the 3rd singular). Cells are stem +
exponents with `+`/`=` boundaries **tagged with their slot**
(`phon.Boundary.Tag`), so the stem can still be found after sound changes
have eroded the affixes; untracked lexemes inflect by the productive pattern
when rendered. Sound changes run over cells exactly as over words (same memo).

**Grammaticalization** is four innovation steps that spread as waves:
recruit a source word (Heine & Kuteva paths, e.g. 'go' → future, 'back' →
locative, pronouns → agreement), fix the construction, cliticize (the source
word's first syllable attaches to every tracked cell with `=`), and fuse
(`=` → `+`). A step applies only where the variety's exponent is at the
previous stage with the same source, so neighbours can grammaticalize
different words for the same job. **Loss** is detected, not proposed: when
three quarters of a slot's cells sound like their partner cells without it,
the exponent is lost and the slot can be renewed.

**The proto-language starts with morphology.** Its typology is drawn as
isolating, agglutinating or synthetic (20/50/30), and each slot starts bound,
periphrastic or absent with slot-specific odds. Affix sides follow word order
(postpositions → case suffixes, OV → tense suffixes, …) with the suffixing
preference. Starting from an isolating proto-language would leave too little
morphology for irregularity to emerge within 1000 years.

**Analogy** levels a whole paradigm to the productive pattern (the affixes
with the highest type frequency in each cell), rebuilding it from the stem of
the base cell (nominative singular, present 3rd singular), not the citation
form: word-final changes hit the citation form but not suffixed cells, and
levelling from it would make every verb look irregular. A paradigm is
irregular when any cell differs from what the productive pattern gives its
base stem, which counts phonologically conditioned alternations (final
devoicing, umlaut) as irregular, as they are until analogy removes them.
Levelling proposals per 100 000 community-generations go from 30 (class 1)
to 10 000 (class 5).

**Word order** has six parameters with WALS priors (`morph.SampleOrder`
samples clause order, then each parameter given the earlier ones). Harmonic
drift weighs a new value by its WALS frequency times its **mean** lift with
the other parameters; the product of lifts made harmonic languages
unchangeable. Losing the accusative triples the pull toward SVO.

**Intelligibility** now mixes core vocabulary (750‰), inventory (100‰), a
sample of inflected forms (100‰) and word order (50‰). The Romance
calibration takes inflected forms to be as similar as words and Romance word
order to agree, and still groups Spanish–Portuguese (561‰) and separates
Spanish–Italian (467‰) at 490‰.

**Replay is exact for morphology too** (`history.Replay`, tested), which is
how inflected forms get etymologies (`history.TraceCell`). The memo of rule
results is kept only for innovations that are still spreading or diffusing.

**Parallel text** is six abstract clauses (concepts, features, roles)
rendered through each variety's paradigms, periphrases and word order, with
Leipzig glosses read off the tagged boundaries (fused exponents are glossed
with "."). A bound article in an oblique case attaches outside the case form.

**Long runs** (3000 years) show the cycle without forcing it: fusion rises
from 0 to 40–60%, some exponents wear away and are renewed, and synthesis
drifts (seed 1002 goes from 1.97 to 1.46 morphemes per word).
