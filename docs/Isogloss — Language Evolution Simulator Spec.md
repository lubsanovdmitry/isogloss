# Isogloss — Language Evolution Simulator Spec

Nov 7, 2026 · @Bazar · Draft v0.1

## Overview

Isogloss is a seeded, deterministic simulation of a language family over 1–3 thousand years on a procedural continent, run in the browser. The aim is emergence: irregular plurals, loanword layers, dialect continua and tonal languages should appear from a few interacting systems, not from scripted events.

**Goals**

- Plausible change, weighted by how often things happen in real languages, with rare oddities still possible
- Every form explainable: each word carries a full etymology trace back to the proto-language
- Readable output: comparative tables, paradigms, grammar sketches, parallel texts, chronicles
- Fun to poke: god-mode interventions, scenarios and a steering campaign on one engine

**Non-goals for now:** full phrase-structure syntax, audio, simulating individual speakers, the reconstruction puzzle mode, sprachbunds and creoles. Each one is an extension point (see Open questions).

**What it adds over [the-tongue](https://github.com/JasonWarrenUK/the-tongue/)**

| Area | the-tongue | Isogloss |
| --- | --- | --- |
| Sounds | Strings, 17 fixed rules | Feature bundles; \~150 rule templates instantiated per language; stress, length, tone, phonation |
| Lexicon | 32 words, form only | \~300 concepts with frequency; meanings drift along a colexification graph |
| Grammar | None | Inflection, grammaticalization, analogy, word-order parameters |
| Map | 4×3 grid | \~300 procedural hexes with rivers, coasts, mountains |
| Spread | Branch-level drift | Changes spread hex to hex as S-curves (wave model); the tree is derived afterwards |
| Contact | Assimilation only | Borrowing with nativization, loan strata, substrate, toponyms |
| Society | None | Migration, conquest, prestige, standard languages, writing, trade routes |
| Distance | Edit distance | Feature-weighted alignment + morphology + word order |

## Decisions

These came from the kickoff Q&A. Rows marked "default" were left open and take the recommended option.

| Topic | Decision | Source |
| --- | --- | --- |
| Stack | Go engine compiled to WASM, running in a Web Worker; TS/Svelte UI; same engine as a native CLI | chosen |
| Player role | God-mode sandbox as the base layer | default |
| Tick length | One generation, \~25 years | default |
| Realism | Plausible and data-weighted; rare oddities allowed | default |
| Prosody | Stress, length, tone and phonation (breathy/creaky, register) | chosen |
| Rule source | \~150 curated templates, instantiated procedurally over each language's natural classes | chosen |
| Rule syntax | User-facing SCA/Lexurgy-style syntax; the engine's own rules use it too | chosen |
| Grammar | Noun and verb inflection, grammaticalization, analogy, word order; full syntax later | chosen |
| Lexicon | \~300 concepts with frequencies | chosen |
| Semantics | Drift along the CLICS colexification graph | chosen |
| Texts | Template parallel texts from an abstract tree | chosen |
| Population | Region-level speech communities on hexes; wave-model spread | chosen |
| Contact (v1 spec) | Borrowing with loan strata; substrate effects | chosen |
| Society | Migration and conquest, prestige and standards, writing and spelling, trade routes | chosen |
| Game modes | Sandbox, scenarios, steering campaign | chosen |
| LLM | Naming, flavour, myths and chronicles; never changes simulation state | chosen |
| Map | \~300 procedural hexes | default |
| Audio | Later, via espeak-ng; IPA output hook from day one | chosen |
| First playable | Phonology, lexicon, tree, etymologies | chosen |
| WASM toolchain | Standard Go (\~1.5–3 MB compressed); strip with -s -w, run wasm-opt -Oz, keep fmt/reflection out of the engine, load the worker after the UI shell. Revisit TinyGo only if mobile becomes a target | chosen |
| Data sources | Ship small derived tables from PHOIBLE, CLICS and WALS with attribution (licences to confirm); hand-curate sound-change template frequencies, using Index Diachronica as reference only | chosen |
| Numerics | Hybrid: integers (fixed-point) for anything that feeds a decision (adoption levels, probabilities, thresholds); floats only for display. Golden tests on amd64, arm64 and wasm | chosen |
| Community-size biases | Sliders, default moderate; one scenario with them turned up | chosen |
| Intelligibility calibration | Check the lexical metric on a few Romance pairs with known intelligibility (Spanish–Portuguese high, Spanish–Romanian lower); set the clustering threshold so groupings match accepted ones | chosen |
| Scripts | Romanized grapheme names only; no glyph generation | chosen |
| Project name | Isogloss ("Drift" collides with a popular Dart/Flutter library and a SaaS company) | chosen |

## Architecture

One pure-Go engine with no I/O runs in two places: compiled to WASM inside a Web Worker for the browser, and natively as the `isogloss` CLI. The UI never computes linguistics. It sends commands and renders snapshots.

&#91;embedded content: architecture · engine in a worker, three consumers\]

The worker owns all simulation state. The UI, the save file and the CLI all go through the same engine packages.

**Packages** (`engine/`)

| Package | Owns |
| --- | --- |
| `rng` | Keyed PCG streams, one per subsystem and entity, so adding a subsystem never reshuffles the others |
| `phon` | Feature table, segments, IPA parse/render, syllabification, phonotactics |
| `rules` | Rule syntax parser, matcher, applier, template library, weighting |
| `lex` | Concepts, lexemes, frequency, CLICS graph, replacement and coinage |
| `morph` | Categories, exponents, paradigms, grammaticalization, analogy, word order |
| `world` | Hex map generation, terrain, rivers, passability |
| `pop` | Communities, growth, contact weights, wave spread, clustering into languages |
| `contact` | Borrowing, nativization, strata, language shift, substrate |
| `society` | Polities, migration, conquest, prestige, standards, writing, trade |
| `history` | Innovations, event log, etymology traces, snapshots |
| `render` | Comparative tables, paradigms, grammar sketch, parallel texts, CLDF export |
| `sim` | Tick loop, interventions, save/replay |

**Entry points**

- `cmd/isogloss`: `isogloss run --seed 42 --ticks 120 --out run.json`, `isogloss sca rules.isg words.txt` (standalone sound-change applier), `isogloss etym run.json kaft`
- `wasm/`: a `syscall/js` bridge in a Web Worker; JSON messages (`step`, `intervene`, `query`, `snapshot`). Start with standard Go and measure the bundle; try TinyGo once the engine stabilises.
- `web/`: Svelte + TS views. LLM calls go from here with a user-supplied key.

**Determinism rules**

- A save is `seed + config + intervention log + LLM annotation cache`. Replay rebuilds everything else.
- Never range over a Go `map` where order matters: Go randomises map iteration order. Iterate sorted keys or slices.
- Go may fuse multiply-add into FMA on arm64 but not on amd64 or wasm, so the same float code can differ by platform. Keep simulation maths in fixed-point integers (probabilities as `uint32` per-mille or Q16), or force rounding with explicit `float64(...)` conversions.
- Golden tests: fixed seed → hash of the run dump, checked in CI on amd64 and wasm.

## Data model

The central object is the **innovation**, not the language. A community's variety is the proto-language plus the innovations it has adopted, in the order it adopted them. Languages and the family tree are computed from that.

```go
type Segment struct {
    F    FeatureSet // bitset of +/- values, plus a mask for unspecified features
    Tone Tone       // on syllable nuclei only
    Phon Phonation  // modal, breathy, creaky
}

type Word struct {
    Segs     []Segment
    Bounds   []Boundary // morpheme '+', clitic '=', word '#' positions
    Syll     []Span     // recomputed after every change
    Stress   []int8     // per syllable: 0 none, 1 primary, 2 secondary
}

type Lexeme struct {
    ID      LexID
    Senses  []Sense    // concept + weight; many-to-many with concepts
    Form    Word
    Freq    uint16     // frequency rank class
    Stratum Stratum    // native, or donor variety + tick borrowed
    Etym    EtymID     // head of the trace chain
}

type Innovation struct {
    ID      InnovID
    Kind    Kind       // Sound, Lexical, Semantic, Morph, Gram, Order, Loan, Spelling
    Origin  CommunityID
    Tick    int
    Payload any        // a compiled Rule, a replacement, a stage change...
}

type Community struct {
    Hex      HexID
    Pop      int
    Prestige Q16
    Variety  *Variety  // copy-on-write; identical varieties share one pointer
    Adoption map[InnovID]Q16 // wave progress 0..1 (iterate sorted!)
}

type Variety struct {
    Adopted   []InnovID  // ordered
    Inventory Inventory
    Phonotax  Phonotactics
    Lexicon   []Lexeme
    Morph     Morphology
    Order     OrderParams
    Script    *Orthography
}

type TraceStep struct {
    Tick   int
    Innov  InnovID
    Before, After Word
}
```

**Sizing:** 300 hexes × 300 lexemes × \~6 segments is about 540k segments if every hex had its own variety. Copy-on-write sharing keeps the real number far lower, since neighbours are identical most of the time.

**Languages** are recomputed every K ticks. Varieties are clustered by mutual intelligibility; a chain of intelligible neighbours whose ends are not intelligible is reported as a dialect continuum.

## Phonology

Every sound is a bundle of distinctive features, so rules target natural classes and the IPA is just a rendering.

| Group | Features |
| --- | --- |
| Major class | syllabic, consonantal, sonorant, approximant |
| Laryngeal | voice, spread glottis (aspiration), constricted glottis (ejective, glottalized) |
| Manner | continuant, nasal, strident, lateral, delayed release |
| Place | labial, round, coronal (anterior, distributed), dorsal (high, low, back, front), pharyngeal, ATR |
| Vowel | high, low, back, round, ATR, nasal |
| Segmental prosody | long |
| Syllable prosody | stress (0/1/2), tone (H, M, L levels and contours), phonation (modal, breathy, creaky) |

**IPA.** A table of \~400 base symbols plus diacritic composition. A bundle with no exact symbol renders as the nearest base plus diacritics, so the engine never invents a sound it cannot display.

**Syllables and phonotactics.** Each language has a template such as `(C)(C)V(C)`, a sonority-based syllabifier with onset maximisation, and a list of banned clusters. After every change the word is re-syllabified. Illegal results trigger weighted repairs (epenthesis, deletion, cluster simplification). Phonotactics drift too: after enough syncope, a language may start to allow clusters instead of repairing them.

**Stress.** A per-language rule: initial, penultimate, final, weight-sensitive or lexical. Stress drives unstressed vowel reduction, syncope and apocope, which is how most erosion happens.

**Inventory pressure** reweights the rule templates every tick:

- **Functional load:** count minimal pairs per contrast. Low-load contrasts merge easily; mergers that would create many homophones are dispreferred.
- **Dispersion:** crowded vowel spaces push vowels apart; gaps in the system (a /p t \_/ series) make gap-filling changes likelier.
- **Chain shifts:** after a vowel moves, the template that moves its neighbour into the vacated space gets a temporary boost.
- **Phonologization:** a conditioned variant (umlauted vowel, palatalized consonant) becomes its own phoneme once its trigger is lost.

**Tone and phonation paths** (each a set of linked templates):

- Final glottal stop lost → rising tone; final *-h/-s* lost → falling tone
- Voiced onset → breathy following vowel → low register → low tone, with the onset devoicing
- Tone contrasts can later merge, or spread and shift as tone sandhi (a later template family)

**Proto-language generation.** Sample an inventory using PHOIBLE segment frequencies and implicational constraints (voiced stops imply voiceless ones; nasal vowels imply oral ones). Pick a syllable template and stress rule. Generate roots so that more frequent concepts get shorter forms.

## Sound-change rules

All sound changes, built-in or user-written, use one text syntax. The template library emits rules in that syntax, so every change the sim makes can be read, copied and replayed in `isogloss sca`.

**Syntax**

```text
# classes: feature matrices or IPA lists
class stop = [-cont -son -syl]
class V    = [+syl]
class C    = [-syl]

# intervocalic voicing
rule intervocalic-voicing:
    [stop -voice] => [+voice] / V _ V

# positional IPA mapping
rule grimm-1:
    p t k => f θ x

# umlaut: back vowel fronts before a later high front vowel
rule i-umlaut:
    [+syl +back] => [-back] / _ C* [+syl +high -back stress:0]

# syncope of short unstressed vowels
rule syncope:
    [+syl stress:0 -long] => * / [stress:1] C _ C V

# tonogenesis from a lost final glottal stop
rule tono-rise:
    [+syl] ʔ => [tone:rise] * / _ #

# exceptions and lexical diffusion
rule final-devoicing @diffuse(freq-first):
    [-son] => [-voice] / _ #
    except / _ + [+syl]   # not before a vowel-initial suffix
```

| Token | Meaning |
| --- | --- |
| `=>` | becomes |
| `/` | environment follows; `_` marks the target |
| `#` `.` `+` `=` | word, syllable, morpheme, clitic boundary |
| `*` | deletion (on the right) or insertion point (on the left) |
| `C*` `(x)` | zero or more; optional |
| `stress:1` `tone:H` `phon:breathy` | prosodic conditions or outputs |
| `except /` | blocking environment |
| `@diffuse(...)` | apply word by word over ticks instead of all at once |

**Templates.** About 150 templates in families: lenition, fortition, assimilation (place, voice, nasal), palatalization, umlaut and vowel harmony, raising, fronting and chain shifts, mergers and splits, apocope and syncope, cluster simplification, epenthesis and prothesis, metathesis, dissimilation, stress shift, lengthening and shortening, tonogenesis, phonation and register. Each template has slots (which class, which environment). At generation time the engine fills the slots only with classes the language actually has.

**Weighting** for a candidate rule:

```latex
w = w_{template} \times f(\text{words affected}) \times p_{inventory} \times p_{terrain/contact} \times d_{recent}
```

The first factor is the template's typological frequency. The words-affected factor is zero below a minimum, so changes always matter. Then come the inventory pressures above, a contact or isolation bias, and a decay factor that discourages the same template firing twice in a row.

**Validation.** Reject no-op rules and rules that would leave no vowels or no consonant contrasts. Penalise, without rejecting, rules that would make more than \~3% of core vocabulary homophonous.

**Regularity.** Changes are regular (Neogrammarian) by default. About 10% of templates are marked diffusible: they creep through the lexicon by frequency over several ticks and can stall, leaving exceptions for analogy to clean up or not.

## Lexicon and semantics

Each variety tracks \~300 concepts. Words and meanings are many-to-many, so a word can gain and lose senses over time.

**Concept list**

- The Swadesh-207 core
- \~60 grammaticalization sources: *go, come, want, finish, have, hold, give, take, say, sit, stand, one, this, that, thing, person*, and body parts used for spatial terms (*back, face, belly, head, side*)
- \~30 cultural items that enter the world through events: *horse, iron, wheel, wine, writing, king, coin*

Each concept has a frequency class, a semantic field, a base replacement rate (pronouns and low numerals are very stable) and an optional taboo flag (*bear, death, wolf*).

**Meaning drift along CLICS.** The CLICS colexification graph records which pairs of meanings real languages express with one word (*tree/wood, hand/arm, see/know*). Drift happens in two steps through polysemy:

1. A lexeme with sense A gains sense B along a CLICS edge, with probability proportional to the edge weight.
2. Later it may lose A, if a competitor covers A better or the drift continues.

**Filling a gap.** When a concept loses its word, the replacement comes from one of:

- an existing word shifting along CLICS
- derivation or compounding from native roots (modifier order follows the word-order parameters)
- a loan from a contact variety
- a euphemism, for taboo concepts ("the brown one" for *bear*)

**Frequency effects**

- Frequent words erode faster phonetically, resist analogy (so they stay irregular) and resist replacement.
- Rare words regularise and get replaced more easily.

**Homophony clashes.** When sound change merges two words in the same semantic field, the less frequent one becomes a candidate for replacement (Gascon *gat* 'cat' vs *gat* 'rooster' is the classic case).

## Morphology and grammar

Inflections are born from ordinary words and worn down by the same sound changes as everything else. Irregularity is never authored; it comes out of that erosion.

**Representation.** Each variety has grammatical categories (noun: number, case, definiteness; verb: tense-aspect, mood, person-number agreement). Each category value has an **exponent** of one of these kinds: a free word, a clitic, a prefix, a suffix, a stem change, or zero. Paradigm cells are built as stem + exponents with `+` and `=` boundaries, so sound rules can see and cross them.

**The grammaticalization cycle.** Every exponent sits at a stage, and moves on probabilistically each tick:

1. **Periphrasis:** a free construction (*I go to eat* → future)
2. **Fixed construction:** its word order freezes and the meaning bleaches
3. **Clitic:** it loses its own stress and leans on its host (`=`)
4. **Affix:** the boundary hardens to `+`; it now belongs to the word
5. **Fusion:** sound change across the boundary erodes it into the stem (umlaut, mutation, portmanteau endings)
6. **Loss:** it erodes to zero, and a new periphrasis (stage 1) takes over the job

No special erosion code is needed after stage 3. An unstressed, word-final clitic is exactly what reduction and apocope templates hit first.

**Grammaticalization paths** (from Heine & Kuteva's *World Lexicon of Grammaticalization*):

| Source | Becomes |
| --- | --- |
| *go*, *come*, *want* | future |
| *have*, *hold* | perfect, then past |
| *finish* | completive, then past |
| *sit*, *be at* | progressive |
| *one* | indefinite article |
| *that* (demonstrative) | definite article, 3rd-person pronoun |
| *back*, *face*, *belly*, *head* | adposition, then case affix |
| *give* | dative, benefactive |
| pronouns | person agreement affixes |
| *say* | quotative, complementizer |
| *person*, *thing* | agentive, nominalizer |

**Analogy** runs every tick on paradigms with more than one stem form:

- **Levelling:** collapse stem alternants inside one paradigm
- **Extension:** move a lexeme into the inflection class with the highest type frequency
- Probability scales with 1 / frequency, so *foot/feet*-style survivors are the frequent words

**Typology metrics** shown per variety: synthesis (morphemes per word), fusion (share of exponents fused with stems), and irregularity (stem allomorphs per paradigm). Over long runs these should cycle: isolating → agglutinative → fusional → isolating.

**Word order** is a set of parameters: clause order (SOV, SVO, VSO…), adposition side, adjective-noun, genitive-noun, demonstrative-noun, relative clause. They change by:

- **Harmonic drift** toward WALS correlations (OV goes with postpositions and genitive-noun order)
- **Case loss** pushing toward fixed order
- **Contact** with prestige neighbours
- **Grammaticalization:** a body-part noun after a genitive becomes a postposition, which can become a case suffix; in VO languages the same path gives prepositions, which rarely become prefixes

**Later: full syntax.** Parallel texts are already abstract trees (see Outputs), so phrase-structure rules and agreement can be added as a new renderer without changing the model.

## World and populations

Languages split because contact thins out, not because a rule says "fracture". Every change starts in one hex and spreads to neighbours along contact links.

**Map.** About 300 hexes from seeded noise: elevation, rivers that flow downhill to the sea, biomes (coast, plains, forest, steppe, desert, mountain, island, sea). Each hex edge has a crossing cost. Rivers are cheap to travel along and cost something to cross; mountains are expensive; sea is closed until a polity has seafaring.

**Communities.** Each settled hex holds one speech community: population (grows toward the biome's carrying capacity), prestige, and a variety. Contact weight between two communities combines edge cost, trade routes, a shared polity and bilingualism.

**Wave spread.** Each innovation has an adoption level *a* (0 to 1) in every community. Each tick it grows locally as a logistic curve and takes in influence from neighbours:

```latex
a_i \leftarrow a_i + r\,a_i(1-a_i) + \sum_j c_{ij}\,\pi_j\,(a_j - a_i)^{+}
```

Here *r* is the innovation's local growth rate, *c* is contact weight and π is the source's prestige. A community applies the innovation when *a* crosses 0.5. Innovations can stall and die, which leaves isoglosses ending mid-map.

**Where innovations start.** Each community proposes new innovations at a base rate. Two tunable biases (hypotheses, not facts):

- Small, isolated communities favour rarer templates and keep morphological complexity.
- Large, contact-heavy communities favour simplification (analogy, loss of irregularity) and borrow more.

**Intelligibility** between two varieties is a weighted mix of:

- lexical similarity on the core list, using feature-weighted alignment (*p* vs *b* is closer than *p* vs *a*), not plain edit distance
- shared inventory
- shared grammatical exponents
- word-order agreement

**Languages and the tree.** Every 4 ticks, varieties are clustered on the intelligibility graph (threshold \~0.7) into languages. The family tree is built from shared innovations, as in the real comparative method, and shown next to the isogloss map so conflicts between tree and wave are visible.

**Population events:** growth, famine and plague, and migration pressure when a hex is over capacity (which feeds Society).

## Contact

v1 covers borrowing (with loan strata) and substrate. Both produce ordinary innovations, so they spread by the same wave model.

**Borrowing**

- Each tick, communities in contact can borrow. Words flow mostly from higher prestige to lower, and into gaps (a new cultural item, a replaced word).
- Borrowability follows the usual hierarchy: nouns > verbs > adjectives > function words > affixes. Core vocabulary and affixes need high bilingualism.
- Calques (morpheme-by-morpheme translations of compounds) are an optional loan type.

**Nativization** of a loan:

1. Map each donor segment to the nearest recipient phoneme by feature distance.
2. Repair phonotactics (epenthesis or deletion).
3. Assign stress by the recipient's rule and a default tone.
4. Under heavy bilingualism, keep a foreign segment as a marginal phoneme instead (like English /ʒ/).

**Loan strata.** Every lexeme records its stratum (native, or donor + tick). A loan only undergoes changes adopted after it arrived. That gives layered correspondences and doublets on their own, e.g. a native reflex and a later loan from a sister language of the same proto-word (*shirt/skirt*).

**Substrate.** When a community shifts language (after conquest, absorption or prestige pressure), the old language leaves a package of innovations in the new one, scaled by how fast the shift happened and how many people shifted:

- **Phonology:** transfer of features (retroflexes, a missing contrast that merges)
- **Lexicon:** local flora, fauna, landscape and culture words
- **Grammar:** word-order or category pressure (for example, evidentials)

**Toponyms.** Every hex has a place name with its own etymology. Names survive language replacement and are reshaped by each new language's sound changes, so place names on the map are a record of who lived there.

## Society

Society systems don't change words directly. They change who talks to whom and whose speech carries weight: contact weights, prestige, and when languages shift.

**Polities.** Groups of communities under one ruler, formed by conquest or union. The capital gets prestige, and a shared polity raises contact weight between its hexes.

**Migration.** Triggered by overpopulation, famine, climate events or a scenario script. A migrating group takes its variety with it and either settles empty hexes, forms a bilingual zone, or conquers.

**Conquest outcomes**, decided by population ratio, prestige and how long rule lasts:

| Outcome | What happens linguistically |
| --- | --- |
| Elite dominance | Conquerors' language gains prestige; locals shift over generations, leaving a substrate |
| Elite assimilation | Conquerors shift to the local language, leaving a superstrate of loans (rule, war, law) |
| Stable bilingualism | Both persist; heavy two-way borrowing |

**Prestige and standards.** Prestige comes from capitals, trade hubs and centres of writing. It weights wave spread, borrowing direction and language shift. A polity with writing and a capital can fix a **standard**:

- The standard changes more slowly (a conservatism factor on adoption).
- It spreads by prestige and levels dialects around it.
- **Diglossia:** a frozen classical high variety sits above diverging vernaculars until a vernacularization event.

**Writing.** Scripts are invented rarely and usually borrowed from a neighbouring polity. Types: alphabet, abjad, syllabary.

- The orthography maps phonemes to graphemes at the moment of adoption.
- Spelling then freezes while pronunciation drifts, so the spelling vs pronunciation gap is measurable per word.
- Spelling reforms happen occasionally. Learned loans from a written prestige language keep bookish forms (doublets like *fragile/frail*).
- Display is romanized grapheme names; real glyph generation is out of scope.

**Trade routes** link prosperous communities along cheap paths. They raise contact weight along the route, carry culture words across the map as Wanderwörter, and make hubs prestigious.

## Game modes

All three modes run the same engine. They differ only in which interventions are allowed and what they cost.

**Sandbox (god mode)**

- Run, pause, step and change speed; inspect any hex, word, innovation or event.
- Interventions: force a sound change (write it in the rule syntax or pick a template), start a migration, trigger a conquest, introduce writing, set prestige, add a trade route, cause a plague or famine, freeze a standard, introduce a cultural item.
- Every intervention goes into the replay log. **Fork** from any past tick to try a what-if.

**Scenarios** are preset files (YAML): map constraints, starting languages, scripted events, optional goals.

| Scenario | Setup | What to watch |
| --- | --- | --- |
| Empire collapse | One prestige language across the continent; the polity breaks up at tick 40 | A dialect continuum forming out of one standard |
| Island hopping | A seafaring founder group, islands only | Inventory shrinking through serial founder effects |
| Steppe riders | Herders conquer settled farmers | Elite dominance vs assimilation; substrate |
| Tone delta | Monosyllabic proto-language with final glottals, on a river delta | Tonogenesis and register |
| Silk road | Long trade route across mixed families | Wanderwörter and loan strata |

**Steering campaign.** You guide one people. Influence comes from population and prestige, and you spend it to:

- promote a standard or adopt a script
- encourage or suppress specific innovations in your territory
- expand, trade or conquer

Goals per campaign, for example: still spoken in 40 hexes after 1000 years; keep the capital and frontier mutually intelligible; or challenge goals like "become tonal". The score combines speakers, prestige, survival and number of daughter languages.

## LLM layer

The LLM adds names and story but never touches simulation state, so runs stay reproducible and work offline.

**Jobs**

| Job | Engine supplies | LLM returns |
| --- | --- | --- |
| Names (peoples, languages, polities, places) | 5–10 candidate forms built from the variety's own lexicon ("river" + "people"), with glosses | A pick, plus an English exonym and a one-line gloss |
| Chronicle | The event log for an era, as structured JSON | A short in-world history of that era |
| Myths and proverbs | The concept list and grammar categories available | An **abstract text tree** using concept IDs, which the engine then renders into each language |

The myth job is the important trick. The LLM writes meaning, not words. The engine produces the actual forms, so a myth exists in every daughter language with correct sound changes and a proper gloss.

**Rules**

- Off by default. Bring your own API key, used from the browser; the provider sits behind a small interface.
- Outputs are annotations keyed by `(seed, event id, prompt version)` and cached in the save file. Replays read the cache and never call the LLM again.
- Any name string the LLM returns is re-checked against phonotactics; invalid ones are rejected and the first candidate is used instead.
- Requests are batched per era to keep cost down.

## Outputs and UI

The point of the sim is what you can read out of it. Every view is drawn from the snapshot plus the trace log.

**Etymology trace** (the word inspector). Example of an irregular plural that nobody scripted:

```text
kep  'heads'   Late Ossan
  < *ˈkap-i   Proto-Ossan   head-PL
  t15  intervocalic voicing   *kapi → *kabi
  t22  i-umlaut               *kabi → *kebi
  t31  apocope                *kebi → *keb
  t36  final devoicing        *keb  → kep
  cf. sg kap < *ˈkap-u: same rules, no umlaut trigger
  analogy: resisted (frequency class 1)
```

**Parallel text.** A short fable stored as an abstract tree: concept IDs, grammatical features, roles. The renderer applies each variety's lexicon, morphology and word order, then outputs IPA, the native spelling if there is a script, and a Leipzig-style gloss:

```text
ˈmira-n   ˈtolu   seβ-ˈdai
child-PL  river   see-PST
'The children saw the river.'
```

**Views**

| View | Shows |
| --- | --- |
| Map | Hexes coloured by language; overlays for one innovation's isogloss, polities, trade routes, toponyms |
| Family tree | Tree from shared innovations; click a node to see its defining changes |
| Isogloss / wave | Adoption levels of chosen innovations over time; tree vs wave conflicts |
| Word inspector | Etymology trace, stratum, senses, doublets |
| Comparative table | Concepts × languages, cognate sets coloured, regular correspondences listed |
| Paradigms | Inflection tables with irregular cells highlighted |
| Grammar sketch | Auto-written per language: inventory, phonotactics, stress, morphology type, word order, example paradigms |
| Parallel text | The fable in every language, interlinear |
| Timeline | Event log with LLM chronicle beside it |
| Intelligibility | Heatmap of language pairs |
| Rule editor | Write a rule, preview it on the live lexicon, inject it as an intervention |
| Spelling | Spelling vs pronunciation per word, the gap over time |

**Exports**

- JSON run dump; Markdown grammar sketches; CSV word lists
- **CLDF** word lists, the standard format of computational historical linguistics. That lets real tools (LingPy, phylogenetic software) analyse a simulated family, and lets you check how well they recover the true tree.
- An IPA output hook for espeak-ng audio later

## Tick loop

One tick is one generation (\~25 years), so 1000 years is 40 ticks. Each tick runs in a fixed order:

1. **Interventions:** apply scheduled scenario events and player actions from the log.
2. **World:** population growth, famine and plague, migration, conquest, polity changes, trade routes.
3. **Contact weights:** recompute the community graph (terrain, routes, polity, bilingualism).
4. **Propose:** each community may propose new innovations (sound, lexical, semantic, morphological, grammaticalization stage, word order), with the origin seeded at a small adoption level.
5. **Spread:** update adoption levels of every live innovation (wave model); drop dead ones.
6. **Apply:** in each community, apply the innovations that crossed 0.5, in the order they crossed; write trace steps.
7. **Repair and pressure:** phonotactic repairs, homophony checks (which queue replacements), analogy.
8. **Contact effects:** borrowing, language shift, substrate packages.
9. **Writing and standards:** script adoption, spelling reforms, standard conservatism.
10. **Every 4 ticks:** recompute intelligibility, cluster languages, update the tree, queue naming requests for the LLM.
11. **Snapshot:** write the event log entry and, every N ticks, a cached snapshot for fast scrubbing.

**Performance target:** 40 ticks (1000 years) on the default map in under 5 seconds in the browser. Identical varieties share one pointer (copy-on-write), and innovations are applied only to varieties that change.

## Milestones

&#91;embedded content: roadmap · 6 phases, 5 gates\]

Each phase ends at a gate that a seeded run can check. M1 is the agreed first playable; extensions from Open questions come after M5.

## Open questions and extensions

**Open questions**

- [x] Licences and formats for PHOIBLE, CLICS, WALS and Index Diachronica. Index Diachronica is a wiki, not structured data, so the template frequencies may need hand curation.
- [x] Is the standard Go WASM bundle acceptable, or move to TinyGo? Measure at M1.
- [x] Fixed-point maths everywhere, or floats with forced rounding? Decide before the first golden test.
- [x] How strong should the community-size biases be (small and isolated = complex, large and connected = simple)? Make them sliders and tune by eye.
- [x] Intelligibility weights and the 0.7 clustering threshold need calibrating against something real (e.g. Romance or Slavic word lists).
- [x] Scripts: decided, romanized grapheme names only (no glyph generation).
- [x] Final project name: Isogloss.

**Extension points** (designed for, not built)

| Extension | Hooks into |
| --- | --- |
| Reconstruction puzzle mode | The engine already knows the true proto-forms and rules; grade player reconstructions against them |
| Full syntax | A new renderer for the abstract text trees |
| Sprachbund convergence | An areal innovation kind that spreads across language borders |
| Pidgins and creoles | A new-language birth event at trade hubs and plantations |
| Audio | espeak-ng on the IPA output hook |
| Agent-based speakers | Replace a community's adoption level with a simulated speaker network |
| Phylogenetic validation | Run real phylogenetic tools on the CLDF export and compare with the true tree |
