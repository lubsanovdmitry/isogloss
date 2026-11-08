// The storyteller's jobs. The engine supplies material and renders the
// result; the model only picks names, writes prose, and writes myths as
// abstract sentence trees (concept labels, never words). Every output is
// stored as an annotation in the run, keyed by (seed, job, prompt version),
// so it is asked for once and replays read it back.
import { z } from 'zod';
import type { Engine } from './engine';
import type { StoryProvider } from './llm';

export const PROMPT_VERSION = 'v1';

export interface Candidate { form: string; name: string; gloss: string }
export interface NameNote { native: string; form: string; meaning: string; exonym: string; note: string }
export interface Chronicle { title: string; text: string }
export interface NP { noun?: string; pron?: string; adj?: string; pl?: boolean; def?: boolean; gen?: NP }
export interface ClauseTree { subj: NP; verb: string; tense: string; obj?: NP; dat?: NP; loc?: NP; english: string }
export interface Myth { title: string; clauses: ClauseTree[] }

export const key = (seed: string, job: string) => `${seed}/${job}/${PROMPT_VERSION}`;

export async function annotations(engine: Engine): Promise<Map<string, string>> {
  const list = await engine.query<{ key: string; value: string }[]>('annotations');
  return new Map(list.map((a) => [a.key, a.value]));
}

async function remember(engine: Engine, k: string, value: unknown) {
  await engine.call('annotate', { key: k, value: JSON.stringify(value) });
}

const WORLD =
  'You are the storyteller for Isogloss, a simulation of a family of languages evolving over centuries on an invented continent. ' +
  'Everything you are given is simulated; treat it as the history of an invented world, with no real peoples or places.';

// ---- Names

const NamesOut = z.object({
  names: z.array(
    z.object({
      lang: z.number().int().describe('the language id you are naming'),
      pick: z.number().int().describe('index of the chosen candidate'),
      exonym: z.string().describe('an English name for the people, one or two words, evocative but plain'),
      note: z.string().describe('one line on why this name suits them'),
    }),
  ),
});

/** Names every living language not named yet, in one request. */
export async function nameLanguages(
  engine: Engine, llm: StoryProvider, seed: string, langs: { id: number; name: string }[], notes: Map<string, string>,
): Promise<number> {
  const todo: { id: number; cands: Candidate[] }[] = [];
  for (const l of langs) {
    if (notes.has(key(seed, `name/lang/${l.id}`))) continue;
    const cands = await engine.query<Candidate[]>('namecands', { kind: 'lang', id: l.id });
    if (cands.length) todo.push({ id: l.id, cands });
  }
  if (!todo.length) return 0;
  const out = await llm.json({
    system: WORLD + ' You name its peoples. Choose names from the candidates only: they are built from each language\'s own words.',
    prompt:
      'For each language below, pick the candidate that would make the best name for its speakers, and give an English exonym and a one-line note. ' +
      'Prefer names that differ from each other. Candidates are numbered from 0.\n\n' +
      todo.map((t) => `Language ${t.id}:\n` + t.cands.map((c, i) => `  ${i}. ${c.name} [${c.form}] '${c.gloss}'`).join('\n')).join('\n\n'),
    schema: NamesOut,
  });
  let n = 0;
  for (const t of todo) {
    const got = out.names.find((x) => x.lang === t.id);
    // A pick outside the candidates is rejected: the first candidate stands in.
    const c = got && got.pick >= 0 && got.pick < t.cands.length ? t.cands[got.pick] : t.cands[0];
    const note: NameNote = { native: c.name, form: c.form, meaning: c.gloss, exonym: got?.exonym ?? '', note: got?.note ?? '' };
    await remember(engine, key(seed, `name/lang/${t.id}`), note);
    notes.set(key(seed, `name/lang/${t.id}`), JSON.stringify(note));
    n++;
  }
  return n;
}

// ---- Chronicles

const ChronicleOut = z.object({
  title: z.string().describe('a short title for the era'),
  text: z.string().describe('the chronicle: 120-220 words of plain prose, past tense'),
});

/** Writes the chronicle of an era (ticks from..to). */
export async function chronicle(engine: Engine, llm: StoryProvider, seed: string, from: number, to: number): Promise<Chronicle> {
  const era = await engine.query<unknown>('era', { from, to });
  const out = await llm.json({
    system: WORLD + ' You write short in-world chronicles, like an annalist: concrete, unhurried, no modern idiom, no headings.',
    prompt:
      'Write the chronicle of this era from the record below. Mention languages, polities and places by the names given. ' +
      'Do not invent events that contradict the record; you may add colour (weather, mood, what people said).\n\n' +
      JSON.stringify(era),
    schema: ChronicleOut,
  });
  await remember(engine, key(seed, `era/${from}-${to}`), out);
  return out;
}

// ---- Myths

// Noun phrases nest one level (a possessor has no possessor of its own):
// structured outputs work best with non-recursive schemas.
const NPBase = z.object({
  noun: z.string().optional(),
  pron: z.string().optional(),
  adj: z.string().optional(),
  pl: z.boolean().optional(),
  def: z.boolean().optional(),
});
const NPOut = NPBase.extend({ gen: NPBase.optional() });
const MythOut = z.object({
  title: z.string(),
  clauses: z.array(
    z.object({
      subj: NPOut,
      verb: z.string(),
      tense: z.string(),
      obj: NPOut.optional(),
      dat: NPOut.optional(),
      loc: NPOut.optional(),
      english: z.string().describe('the clause in plain English'),
    }),
  ),
});

interface MythSpec { nouns: string[]; verbs: string[]; adjectives: string[]; pronouns: string[]; tenses: string[] }

/**
 * Asks for a myth as an abstract text tree and checks that the engine can
 * render it in every language; one retry with the engine's complaint.
 */
export async function tellMyth(engine: Engine, llm: StoryProvider, seed: string, n: number, theme: string): Promise<Myth> {
  const spec = await engine.query<MythSpec>('mythspec');
  const base =
    'Write a short myth (4-8 simple clauses) as an abstract sentence tree. You do not write words in any language: ' +
    'the engine renders each clause into every language with its own grammar. Each clause has a subject noun phrase, a verb, a tense, ' +
    'and optionally an object (obj), a recipient or goal (dat) and a place (loc). A noun phrase has either a noun or a pronoun (pron), ' +
    'optionally an adjective (adj), plural (pl), definite (def) and a possessor (gen, another noun phrase).\n' +
    `Use only these labels.\nnouns: ${spec.nouns.join(', ')}\nverbs: ${spec.verbs.join(', ')}\nadjectives: ${spec.adjectives.join(', ')}\n` +
    `pronouns: ${spec.pronouns.join(', ')}\ntenses: ${spec.tenses.join(', ')}\n` +
    (theme ? `\nTheme: ${theme}\n` : '');
  let prompt = base;
  for (let attempt = 0; attempt < 2; attempt++) {
    const myth = await llm.json({ system: WORLD + ' You tell its myths.', prompt, schema: MythOut });
    try {
      await engine.query('rendertext', { clauses: myth.clauses });
      await remember(engine, key(seed, `myth/${n}`), myth);
      return myth;
    } catch (e: any) {
      prompt = base + `\nYour previous attempt could not be rendered: ${e.message ?? e}. Use only the labels listed.`;
    }
  }
  throw new Error('The myth used words the languages cannot render.');
}
