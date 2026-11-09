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
| M3: loan strata and doublets in the comparative table | `TestLoanStrataAndDoublets` (seeds 1–12): every run's table has substrate words; of the 8 runs that split into languages by year 800, all 8 show loans from at least three donors and a loan doublet (a root present both natively and as a loan; 7 of 8 before court loans) |
| M4: Empire collapse produces a continuum | `TestEmpireCollapseContinuum` (seeds 7–9): under the empire 72–100% of communities understand the capital's speech; 800 years after it falls in year 1000, 22–30% do, and the family forms a continuum of 7–10 languages |
| M4: 1000 years in the browser | ~3.7 s for the default run (seed 42); scenario runs that settle the whole map at year 0 are slower (Empire collapse ~14 s per 1000 years) |
| Follow-ups (2026-11-09): 1000 years | default run ~4.0 s in headless Chromium (machine loaded), ~1.75 s native on one core; Empire collapse 5.2 s native (was 9.6) |
| M3: 1000 years in the browser | ~3.3 s in headless Chromium (seed 42); ~1.5 s native on one core (M2's 1.8 s on the same machine) |

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
- **Word replacement** uses four sources: a meaning shift along a CLICS edge
  (the neighbour concept's word), a compound of native words, a fresh root, or
  (M3) a loan from a foreign neighbour.
- **Senses are concept slots, not a separate table.** The lexicon stays
  indexed by concept; one word with several senses is the same root and form
  in several slots (`Variety.Senses`). This keeps every per-concept path
  (sound changes, traces, intelligibility) unchanged.
- **Etymologies are replayed, not stored.** A variety's history is a persistent
  list of entries; replaying it on the proto word rebuilds the word exactly
  (tested). Diffusing changes record which frequency classes they reached, and
  template relaxations are history entries too, so replay is exact.
- **Golden test hashes a state digest** (`Sim.Digest`) rather than SHA-256 of a
  JSON dump; it covers every community's forms, etymologies, population and
  language plus the language list.

## Calibration knobs (`sim.DefaultConfig`)

Sound-change proposal rate 60‰ per community per generation; word-replacement
rates by stability class (8–240 per 100k community-generations, scaled ×0.45
since the whole Swadesh list got curated stability classes, was ×0.52);
innovations start at 0.2 adoption with a local growth rate in −0.25…0.75 and
spread for 5–15 generations. Across 16 seeds (`TestSeedSweep`) 1000 years give
a median of 7 languages (1–11) and median core retention of 782‰ (2026-11-09; was 5 languages, 2–7); real-world
retention is ~80% per millennium. M3 knobs: meaning drift 30‰, borrowing
×1000, inventions 40‰ per unknown item per generation, 4 substrates leaving a
package at 100‰ of settlements, structural changes crossing language
borders at half strength.

## Known gaps in M1

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
to 3000 (class 5), ×√10 per class: Lieberman et al. (2007) find
regularization ∝ frequency^-1/2. (The first table, 30…10 000, was never
exercised: the old concept data put no tracked lexeme in the rare classes.)
`RegularForms` is memoised on (productive patterns, stem, phonology).

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

## M3 · Meaning and contact

**Senses and drift.** A word gains a neighbouring sense along a CLICS edge
(a `Semantic` innovation that sets `Lexeme.Also`; 30‰ per community per
generation, edges weighted by family count). The concept the word is
gaining becomes five times likelier to be replaced, and its replacement is
most likely that word: a meaning shift (`LexChange.From`), which each
adopting variety applies with **its own** form of the source word, so a shift
spreading across dialects does not drag the origin's pronunciation along.
The proto-language already colexifies 8% of strong CLICS edges (≥ 8
families). Replacements by shift dropped to 20/95 of coinages: at 50 a third
of the lexicon ended up polysemous.

**Homophony clashes.** `Variety.Pressure` (cached per variety) lists pairs
of words from different roots in the same semantic field that have come to
sound alike; the less frequent one is replaced at 2500 per 100k
community-generations, and its etymology says why ("after it came to sound
like 'rooster'").

**Unknown things.** 60% of the 41 cultural items have no word in the
proto-language (`Lexeme.Absent`). Each generation an unknown item may appear
in one community (weighted by population), which names it; same-language
neighbours take the word over and other languages borrow it. Words that fill
a gap apply only where there is still a gap.

**Borrowing.** Words never cross a language border by the wave (structural
changes do, at half strength: areal features). Instead a community borrows
from a foreign neighbour at 400‰ per generation × contact weight, ×0.3 from a
less prestigious one. The concept is drawn by borrowability (nouns > verbs >
adjectives > numerals and function words, unstable meanings first, cultural
items ×4, a sister's form of the same root ×2, and ×6 when the native word
has several senses: the shirt/skirt route to doublets). The borrower adopts
the loan at once and it spreads inward at a positive rate for 8–20
generations. Without that, most loans died at the border and centres had
none.

**Nativization** (`contact.Nativize`) maps each segment to the nearest
recipient phoneme by feature distance, drops morpheme boundaries, repairs
phonotactics, applies the recipient's stress rule and most common tone. Over
a strong contact a loan keeps its foreign segments 20% of the time (marginal
phonemes). It happens once, where the word is borrowed; inside the
recipient language the loan spreads as a word.

**Strata.** `Lexeme.Stratum` records native, borrowed (donor language and
generation) or substrate. A loan only undergoes the changes adopted after it
arrived (`TestLoansOnlyTakeLaterChanges`), so each stratum has its own
correspondences; the correspondence table excludes loans.

**Substrates and place names.** Four substrate languages (generated like
proto-languages) cover the land outside the homeland, by nearest of four
spread-out seeds. Every land hex has a name in the language of its area (or
the proto-language in the homeland): a landscape word for its biome or
river, alone, with a property ("white river") or a personal name ("Tamu's
hill"), in that language's adjective order. A community settling a hex takes
the name over, nativized, and its later sound changes reshape it
(`history.Evolve`, computed on demand). At 100‰ of settlements the local
people's shift leaves a package: one or two words for local plants, animals
or landscape (one root per substrate word, so the same substrate word in two
branches is cognate), at 25% a merger of a phoneme the substrate lacked, at
15% its value of one word-order parameter.

**Traces.** `history.Trace` follows a word through shifts, loans, gap words
and extra senses; a shift depends on the source word's form at that moment,
which is traced to the same point recursively. Still exact (tested).

**Views.** The comparative table marks loans (L), substrate words (S) and
doublets (D) with their source, can show all concepts, and can filter to
loans; Word history shows where a loan came from, the word's other senses,
an emerging sense and doublets; the Grammar tab lists vocabulary layers; the
map shows a hex's place name with its etymology and can outline the
substrate areas; `isogloss grammar` lists loans; the CLDF export has a `Loan`
column and leaves out concepts a language has no word for.

## M4 · Society and scenarios

Society never changes words directly: it changes contact weights, prestige
and who shifts language (`engine/society` for state, `sim/society.go`,
`sim/writing.go` for the tick logic, which runs after population growth).

**Polities.** A community of 900+ people at least three hexes from any
capital may found a polity (15‰ per generation), taking in unaffiliated
same-language neighbours. Each generation a polity may take one
neighbouring hex within seven of its capital: union with speakers of its
language, conquest of other-language land or war with another polity, by
strength (total population). Polities fall more easily as they grow and
age. Capitals carry ×1.5 prestige, members ×1.1, trade hubs ×1.25; a
shared polity strengthens contact ×1.5.

**Foreign rule** applies only to communities that spoke another language
than the capital's when they joined (dialects that drift apart inside a
polity are not foreigners). After four generations the outcome is decided
by the ratio of rulers' to locals' population **within three hexes**:
≥ 600‰ elite dominance (the locals shift, 15% per generation, and their
language becomes a substrate that may leave words, a merger and word
order), < 250‰ elite assimilation (superstrate loans for rule, war, law,
religion), in between bilingualism (contact ×1.5, borrowing ×3), which long
rule turns into dominance. A polity-wide ratio gave only dominance: by the
time herders conquer, their colonists outnumber everyone.

**Disasters, migration, routes, sea.** Famine (5‰ per community, double on
harsh land), plague (25‰ per generation somewhere populous, hitting three
hexes around). A full community with no empty neighbour may send a quarter
of its people to the nearest empty land within six hexes (Dijkstra; across
the sea for seafarers). Every eight generations the eight largest
communities at least four hexes apart become hubs linked to their two
nearest hubs along the cheapest path: route edges double contact, and hubs
borrow along routes (Wanderwörter). A polity with a coastal capital may take
to the sea: its coastal communities link to land up to two sea hexes away
and colonise islands. A group crossing the sea loses its rarest sound, and half
the time the next rarest (a founder effect that does not spread).

**Writing.** A polity of six or more hexes with no writing neighbour may
invent a script (10‰); one within ten hexes of a writing polity borrows its
script (8%; an abjad becomes an alphabet 30% of the time). The orthography
maps every phoneme to its romanized name when adopted; spellings then stay
fixed while words change, new words are spelled as they sound when they
arrive, and a mean gap above 300‰ invites reform. Syllabaries write one
sign per onset+vowel and drop codas. A polity that writes may fix a
standard: its capital takes up innovations at half the rate, communities
level words to the capital's, and once the capital no longer understands
the standard it becomes classical and lends learned loans (stratum
`Learned`) until the vernacular is written instead.

**Why standards unify.** Stronger contact alone made an empire diverge
*more* (more innovations travel, none reaches everywhere: a wave lives
5–15 generations). What holds a standard language together is top-down
spread: once the capital adopts an innovation, every community following
the standard moves 30% of the way toward it each generation, wherever it
is, and innovations from elsewhere take hold at a fifth of the rate. This
continues under diglossia (the capital's speech keeps its prestige).

**Families.** Each family has its own proto-language; roots are
namespaced (`lex.FamilyRoot`), so unrelated words never look cognate, and
each variety records its family so traces replay from the right proto.

**Scenarios.** `engine/scenario` reads a YAML subset (block mappings and
sequences, scalars, folded text, comments) without reflection and rejects
unknown keys. A scenario sets world shape (continent, archipelago,
corridor), land and rivers; proto constraints (template, syllables, share of
final glottal stops); families and their homelands (named places such as
`fertile`, `steppe|west`, or hex numbers; radius or `all`); config
overrides; starting polities (script, standard, seafaring); and events. The
events become interventions in the save (which also stores the scenario
source), so scenario runs replay exactly. Five starters are embedded:
Empire collapse, Island hopping, Steppe riders, Tone delta, Silk road
(`TestStartersShowWhatTheyPromise` checks what each says to watch).

**Performance.** `Rule.MayApply` skips words in which no clause's first
target segment occurs; clash detection sorts instead of hashing strings.
Varieties that differ in history cannot share work, so runs that settle the
whole map at year 0 cost several times a default run.

## M5 · Campaign and storyteller

**Campaign.** A scenario with a `campaign:` section makes the run a
steering game. The player's polity is founded at `people`; influence
accrues each generation (population / 1000 + 3, +5 with writing, +5 with a
standard, scaled by `income`). Player actions are ordinary interventions
marked `Paid`: they are charged when they take effect and refused (with a
chronicle note) when the purse is short, so a campaign replays exactly from
its save. Costs: script 150, standard 200, seafaring 150, route 100,
encourage 40, suppress 60, expand 30, conquer 120. Encouraging a spreading
change multiplies its local rate and its pull from neighbours ×4 in the
player's communities, suppressing ×0.1. God-mode actions (forcing sound changes, plagues) are
refused in a campaign. Goals: `spoken: N` (hexes intelligible with the
capital), `unity` (the farthest member still understands the capital),
`tonal` (≥ 10% of the capital's vowels carry tone), `daughters: N` (living
languages of the people's family), `survive`. Score: speakers / 100 + 50 per
daughter + 300 per goal met + 200 while the polity stands. The player's
polity collapses a quarter as often as others.

**Storyteller.** Off by default; lives entirely in `web/` and never
touches simulation state. The provider interface (`web/src/lib/llm.ts`) has
one implementation, the Anthropic SDK called from the browser with the
player's own key (kept in sessionStorage unless they ask to remember it),
loaded on first use with the Stories tab so runs without it download
nothing extra. Default model Claude Opus 5.5 at low effort (the tasks are
short), structured outputs (zod schemas), server-side refusal fallbacks on
the models that support them. Three jobs (`web/src/lib/story.ts`):

- *Names*: the engine builds 5–8 candidates per language from its own
  words ('river person', 'good person', its central place name), so every
  name obeys its phonotactics; the model picks one by index (an index out of
  range falls back to the first candidate) and adds an English exonym. All
  unnamed languages go in one request.
- *Chronicles*: the engine gives an era (250 years) as structured events,
  counts and the languages and polities at its end; the model writes
  120–220 words.
- *Myths*: the model writes an abstract sentence tree from the concept
  labels, tenses and pronouns the engine lists; the engine validates it
  (`render.BuildClauses`) and renders it into every language with its own
  morphology and word order. One retry quotes the engine's complaint.

Outputs are annotations keyed `seed/job/prompt-version`, stored in the run
(`Sim.Annotations`, carried by saves, excluded from the digest), so a
replay or a reloaded save never asks again.

## Follow-ups (2026-11-09)

**Canonical bundles.** Sound changes could leave combinations no symbol
shows (an unrounded but labial vowel, a vowel without tense, a voiced
glottal stop), so two segments of an inventory rendered alike, and rules
built from rendered segments (substrate mergers, founder losses, template
slots) silently matched nothing, because a literal matches only its exact
bundle. `phon.Canon` applies the table's redundancy rules after every
feature change (bundles in the table are left alone); `RenderFeatures`
searches the 48 nearest bases with up to three diacritics for an exact
spelling, writes labial-velars with a tie bar (ɣ͡β), and only then
approximates; `phon.Spell` gives rule source (IPA if it parses back exactly,
else a full feature matrix); `phon.Labels` marks what still collides.

**Timeline.** `Sim.Clone` copies what a run mutates (communities, waves,
innovations and their curves, polities and their spellings, the campaign,
the intern table) and shares the rest: the world, the proto-languages and
every variety (varieties are persistent). Caches start empty. A
snapshot pins that moment's varieties, ~10 MB in a default run and ~30 MB
when the whole map is settled, so `Timeline` keeps at most ten and doubles
the spacing when full. `At(t)` clones the latest snapshot ≤ t and steps on;
it equals `Replay(Fork(t))` (tested with thinning, a fork that runs on, and
a clone that diverges while the original must not notice). Forks keep a
scenario's own later events (`Intervention.Scripted`; replay recognises
them in older saves). The worker answers queries from the moment being
looked at.

**Reference varieties and court loans.** A language is recorded (word
lists, tree, naming) at the capital of its largest standing polity, else at
its central community, as a standard language is its capital's speech.
Capitals borrow from foreign capitals within ten hexes or along a trade
route (one word per generation from a more prestigious court, ×0.3 from a
less prestigious one). Loans at the reference varieties rose 40%.

**Calques and substrate sounds.** Coined compound roots record modifier and
head; a borrower translates a foreign compound with its own words 35% of
the time. A substrate package may (20%) turn the variety's nearest phoneme
into a sound only the substrate had, in the first context of four that
changes at least three words.

**Rendering grammar.** Only the 3SG of future, perfect and progressive is
stored. Other persons cut the 3SG agreement (by its tagged boundary) and
add the person's own material from the verb's present-tense productive
pattern. A periphrastic tense built on a verb inflects the auxiliary for
the subject and leaves the main verb plain. A definite noun in an oblique
case is the stored definite form plus the case's own material outside it.

**Measured, not changed.** Polysemy (concepts sharing a word with another)
is a median 16% at 1000 years and no higher at 3000: replacements remove
it as fast as shifts create it. Syncretism counts as loss at two thirds of
cells; 14% of languages lose a slot within 1000 years, still slower than
real attrition, because affix erosion drives it.

**Campaign balance.** `TestCampaignBalance` plays idle, steering and
balanced strategies over ten seeds of both campaigns (writer and expander
too with `ISOGLOSS_BALANCE=1`). Income was ~8000 per run against costs of
30–200 and an idle player met 75% of goals; now income is ~2100, river
people need 200 hexes and four daughter languages, the delta 180 hexes,
and steering scales a change's arrival from neighbours as well as its
local growth (suppressing did nothing before). Idle meets 50% of the river
goals and the balanced player 75%; steering makes the delta tonal in 5 of
10 runs, against 3 idle. A standard slows change at the capital, so it
works against the tone goal.

**Browser smoke test.** `npm run e2e` (after `make web`) serves `dist/`
and drives Chromium in one Node process: place labels, looking back and
forking, and the storyteller with the real SDK against a mocked API
(checking model, beta header, fallbacks, effort and schema). It found that
language naming never sent a request: the query's arguments overwrote its
name.

**Structural interventions.** Besides forced sound changes, the sandbox
can force a grammatical construction (recruit a word for a slot, which
may replace a free construction but not a clitic or affix), push one a
stage along the cycle, set a word-order value, or loosen or tighten the
syllable template by one consonant at the onset or coda (onsets 1–3, as
every language allows CV; codas 0–3). Each is an ordinary innovation
(`Intervention` kinds `gram`, `order`, `syllables`; new kind
`history.Syllable`) that starts at a hex or everywhere and spreads; a
tighter template repairs the lexicon and paradigm cells, and traces show
the repair step. Repair with no codas allowed now puts the epenthetic
vowel after the consonant (it used to loop, as nothing had tightened a
template before). The Change rules tab also offers sound changes: every
template filled from the community's sounds, scored as the simulation
scores its own proposals.

**Dialect views.** The comparative table shows one variety per language;
two views show the variation inside. The map's "A word" mode colours each
settled hex by its form of a concept (or by root), with a legend of the
ten commonest; Word lists → Dialects lists, for one language, every local
form of each word with the number of communities using it, roots shaded
and the word-list form in bold (`render.DialectMapJSON`,
`DialectTableJSON`).
