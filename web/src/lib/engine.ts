// Typed client for the engine worker.
export interface Hex { id: number; col: number; row: number; biome: string; elev: number; river: boolean; down: number; sub: number }
export interface World { w: number; h: number; hexes: Hex[]; substrates: { name: string; seed: number }[] }
export interface Lang { id: number; name: string; centre: number; size: number; continuum: number; parent: number; born: number; died: number }
export interface TreeNode { id: number; lang: number; parent: number; children: number[]; shared: number; defining: { id: number; label: string; kind?: string }[]; definingLex: number }
export interface Event { tick: number; kind: string; hex: number; innov: number; lang: number; text: string }
export interface Snapshot {
  tick: number; seed: string; proto: string; innovations: number; live: number;
  comms: [number, number, number, number][]; // hex, population, language, polity (-1 none)
  polities: Polity[]; routes: number[][]; // routes: hex paths
  families: number; scenario: ScenarioInfo | null; campaign: CampaignInfo | null;
  langs: Lang[]; tree: TreeNode[]; eventCount: number; events: Event[];
}
export interface CampaignInfo {
  polity: number; name: string; alive: boolean; capital: number; influence: number; earned: number; spent: number; score: number;
  script: boolean; standard: boolean; seafaring: boolean;
  goals: { kind: string; n: number; value: number; met: boolean; text: string }[];
}
export interface CampaignDetail extends CampaignInfo {
  costs: Record<string, number>; members: number;
  innovations: { id: number; label: string; kind: string; rule: string; applied: number; steer: number }[];
}
export interface ScenarioInfo { name: string; summary: string; watch: string; ticks: number; events?: { tick: number; do: string }[] }
export interface ScenarioEntry extends ScenarioInfo { id: string; families: number }
export interface Polity { id: number; name: string; capital: number; founded: number; fell: number; seafaring: boolean }
export interface Step { tick: number; innov: number; label: string; before: string; after: string }
export interface Etym {
  concept: string; gloss: string; lang: string; hex: number; proto: string; protoLang: string; form: string; root: string; steps: Step[]; text: string;
  loan: string; also: string; senses: string[]; doublets: { concept: string; form: string; loan: boolean }[];
  spelling: string; gap: number; script: string;
}
export interface TableRow {
  concept: string; id: number; forms: string[]; cog: number[];
  loans: [number, 'loan' | 'substrate' | 'learned', string, number][]; // language index, kind, donor, tick
  dbl: [number, string][]; // language index, concept sharing the root
}
export interface Table { protos: number; langs: string[]; rows: TableRow[] }
export interface Writing {
  polity: string; script: string; since: number; reformed: number; from: string; classical: boolean; gap: number; standard?: number;
  examples: { concept: string; spelling: string; form: string; now: string; gap: number }[];
}
export interface Stratum { kind: 'loan' | 'substrate' | 'learned'; donor: string; count: number; first: number; last: number; examples: string[] }
export interface MapLabel { hex: number; name: string; short: string; capital: boolean }
export interface Place {
  original: string; originalName: string; gloss: string; namedBy: string; substrate: boolean; shortened?: boolean;
  form?: string; name?: string; since?: number; adopted?: string; lang?: string; steps?: Step[];
  polity?: string; capital?: boolean; rule?: string;
}
export interface Exponent { slot: string; name: string; pos: string; stage: string; kind: string; since: number; source?: string; sourceForm?: string; marker?: string }
export interface Sketch {
  lang: Lang; vowels: string[]; consonants: string[]; template: string; stress: string; repair: string;
  changes: { tick: number; innov?: number; label: string; rule?: string; kind?: string }[]; replacements: number;
  native: number; unknown: number; strata: Stratum[]; writing?: Writing;
  exponents: Exponent[]; typology: { synthesis: number; fusion: number; irregularity: number; type: string };
  order: { param: string; value: string }[]; examples: string[];
}
export interface ParaCell { cell: number; label: string; form: string; gloss: string; stem: string; irregular: boolean; regular: string }
export interface Paradigm {
  lang: string; lexemes: { concept: string; pos: string; freq: number; irregular: boolean }[];
  concept?: string; pos?: string; stems?: number; cells?: ParaCell[];
}
export interface Fable { english: string[]; langs: { name: string; id: number; order: string; lines: { f: string; g: string }[][] }[] }
export interface CellEtym {
  concept: string; cell: string; form: string; gloss: string; proto: string; protoGloss: string; protoLang: string; lang?: string;
  steps: Step[]; baseLabel?: string; base?: string; freq: number; irregular: boolean;
}
export interface InnovInfo { id: number; kind: string; label: string; rule: string; origin: number; tick: number; applied: number[]; curve: [number, number][]; level?: number[] }
export interface Corresp { proto: string; langs: string[]; sets: { proto: string; count: number; reflexes: string[]; examples: string[] }[] }
export interface InnovSummary { id: number; label: string; tick: number; origin: number; hexes: number }
export interface Concept { id: number; label: string; gloss: string; core: boolean; field: string; pos?: string }
export interface Frame { tick: number; lang: number[] }
export interface Save { version: number; seed: string; tick: number; threshold: number; interventions: { tick: number; kind: string; hex: number; rule: string }[] }

export class EngineError extends Error {}

export class Engine {
  private worker = new Worker(new URL('../worker.ts', import.meta.url), { type: 'module' });
  private next = 1;
  private pending = new Map<number, (out: string) => void>();
  readonly ready: Promise<void>;

  constructor() {
    this.worker.onmessage = (e: MessageEvent<{ id: number; out: string }>) => {
      const done = this.pending.get(e.data.id);
      if (done) {
        this.pending.delete(e.data.id);
        done(e.data.out);
      }
    };
    this.ready = this.send({ base: new URL('./', document.baseURI).href }).then(() => undefined);
  }

  private send(msg: Record<string, unknown>): Promise<any> {
    const id = this.next++;
    return new Promise((resolve, reject) => {
      this.pending.set(id, (out) => {
        const v = JSON.parse(out);
        if (v && typeof v === 'object' && 'error' in v) reject(new EngineError(v.error));
        else resolve(v);
      });
      this.worker.postMessage({ id, ...msg });
    });
  }

  call<T = any>(cmd: string, args: Record<string, unknown> = {}): Promise<T> {
    return this.ready.then(() => this.send({ cmd: { cmd, ...args } }));
  }

  query<T = any>(kind: string, args: Record<string, unknown> = {}): Promise<T> {
    return this.call<T>('query', { kind, ...args });
  }
}
