// Runs the Go engine (WASM) off the main thread. The first message carries
// the page's base URL; after that each message is one engine command, handled
// one at a time, in order.
const ctx = globalThis as unknown as DedicatedWorkerGlobalScope & { Go: any; isoglossHandle?: (cmd: unknown) => string };

let ready: Promise<void> | null = null;

async function boot(base: string) {
  // wasm_exec.js is a classic script that defines globalThis.Go.
  const code = await (await fetch(new URL('wasm_exec.js', base))).text();
  (0, eval)(code);
  const go = new ctx.Go();
  const { instance } = await WebAssembly.instantiateStreaming(fetch(new URL('isogloss.wasm', base)), go.importObject);
  go.run(instance);
  while (!ctx.isoglossHandle) await new Promise((r) => setTimeout(r, 2));
}

ctx.onmessage = async (e: MessageEvent<{ id: number; base?: string; cmd?: Record<string, unknown> }>) => {
  if (e.data.base) {
    ready = boot(e.data.base);
    try {
      await ready;
      ctx.postMessage({ id: e.data.id, out: '{"ok":true}' });
    } catch (err) {
      ctx.postMessage({ id: e.data.id, out: JSON.stringify({ error: 'could not load the engine: ' + err }) });
    }
    return;
  }
  try {
    await ready;
    ctx.postMessage({ id: e.data.id, out: ctx.isoglossHandle!(e.data.cmd) });
  } catch (err) {
    ctx.postMessage({ id: e.data.id, out: JSON.stringify({ error: String(err) }) });
  }
};
