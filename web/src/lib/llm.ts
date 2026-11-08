// The storyteller's model provider. It only ever returns text and JSON for
// names, chronicles and myths; the simulation never depends on it. The SDK
// is loaded on first use so runs without a storyteller don't download it.
import type { z } from 'zod';

export interface Task<T> {
  system: string;
  prompt: string;
  schema: z.ZodType<T>;
}

export interface StoryProvider {
  readonly label: string;
  json<T>(task: Task<T>): Promise<T>;
}

export class StoryError extends Error {}

/** Models offered in settings; the first is the default. */
export const MODELS = [
  { id: 'claude-opus-5-5', label: 'Claude Opus 5.5' },
  { id: 'claude-sonnet-5-5', label: 'Claude Sonnet 5.5' },
  { id: 'claude-haiku-5-5', label: 'Claude Haiku 5.5' },
  { id: 'claude-fable-5-1', label: 'Claude Fable 5.1' },
] as const;

// Server-side refusal fallbacks are supported on these models (not Haiku).
const FALLBACK_MODELS = new Set(['claude-opus-5-5', 'claude-sonnet-5-5', 'claude-fable-5-1']);

export class AnthropicProvider implements StoryProvider {
  readonly label: string;
  constructor(private apiKey: string, private model: string) {
    this.label = MODELS.find((m) => m.id === model)?.label ?? model;
  }

  async json<T>(task: Task<T>): Promise<T> {
    const [{ default: Anthropic }, { betaZodOutputFormat }] = await Promise.all([
      import('@anthropic-ai/sdk'),
      import('@anthropic-ai/sdk/helpers/beta/zod'),
    ]);
    // The key is the player's own, typed into this page and sent only to
    // the Anthropic API from their browser.
    const client = new Anthropic({ apiKey: this.apiKey, dangerouslyAllowBrowser: true });
    const fallback = FALLBACK_MODELS.has(this.model);
    try {
      const msg = await client.beta.messages.parse({
        model: this.model,
        max_tokens: 16000,
        system: task.system,
        messages: [{ role: 'user', content: task.prompt }],
        output_config: { format: betaZodOutputFormat(task.schema), effort: 'low' },
        ...(fallback ? { betas: ['server-side-fallback-2026-07-01'], fallbacks: 'default' as const } : {}),
      });
      if (msg.stop_reason === 'refusal') {
        throw new StoryError('The storyteller declined this request.');
      }
      if (msg.stop_reason === 'max_tokens') {
        throw new StoryError('The storyteller ran out of room; try a shorter era.');
      }
      if (msg.parsed_output == null) {
        throw new StoryError('The storyteller answered in an unexpected shape.');
      }
      return msg.parsed_output as T;
    } catch (e) {
      if (e instanceof StoryError) throw e;
      if (e instanceof Anthropic.AuthenticationError) throw new StoryError('The API key was not accepted.');
      if (e instanceof Anthropic.PermissionDeniedError) throw new StoryError('This API key may not use that model.');
      if (e instanceof Anthropic.RateLimitError) throw new StoryError('Rate limited: wait a moment and try again.');
      if (e instanceof Anthropic.BadRequestError) throw new StoryError('The request was rejected: ' + e.message);
      if (e instanceof Anthropic.APIConnectionError) throw new StoryError('Could not reach the Anthropic API.');
      if (e instanceof Anthropic.APIError) throw new StoryError(`API error ${e.status}: ${e.message}`);
      throw e;
    }
  }
}

// Settings: the key stays in this tab (sessionStorage) unless the player
// asks to remember it on this device (localStorage).
export interface StorySettings { apiKey: string; model: string; remember: boolean }

function store(remember: boolean): Storage | null {
  try {
    return remember ? localStorage : sessionStorage;
  } catch {
    return null;
  }
}

export function loadSettings(): StorySettings {
  for (const remember of [true, false]) {
    try {
      const raw = store(remember)?.getItem('isogloss.storyteller');
      if (raw) return { ...JSON.parse(raw), remember };
    } catch {
      /* storage unavailable or corrupt: start empty */
    }
  }
  return { apiKey: '', model: MODELS[0].id, remember: false };
}

export function saveSettings(s: StorySettings) {
  try {
    for (const remember of [true, false]) store(remember)?.removeItem('isogloss.storyteller');
    if (s.apiKey) store(s.remember)?.setItem('isogloss.storyteller', JSON.stringify({ apiKey: s.apiKey, model: s.model }));
  } catch {
    /* storage unavailable: settings last for this page only */
  }
}

export function provider(s: StorySettings): StoryProvider | null {
  return s.apiKey ? new AnthropicProvider(s.apiKey, s.model) : null;
}
