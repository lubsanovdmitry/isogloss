// Browser smoke test for the built app (web/dist): map labels, looking back
// and forking from the timeline, and the storyteller end to end against a
// mocked Anthropic API (the real SDK in the page, the network faked).
//
//   make web && node web/e2e/smoke.mjs
//
// Chromium: CHROMIUM=/path/to/chrome, else Playwright's cached build.
// Everything runs in one Node process (it serves dist/ itself).
import { createServer } from 'node:http';
import { readFile, readdir } from 'node:fs/promises';
import { homedir } from 'node:os';
import { extname, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright-core';

const dist = fileURLToPath(new URL('../dist/', import.meta.url));
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.wasm': 'application/wasm', '.json': 'application/json', '.svg': 'image/svg+xml' };

const server = createServer(async (req, res) => {
  const path = normalize(decodeURIComponent(new URL(req.url, 'http://x').pathname)).replace(/^\/+/, '') || 'index.html';
  try {
    const body = await readFile(join(dist, path));
    res.writeHead(200, { 'content-type': types[extname(path)] ?? 'application/octet-stream' });
    res.end(body);
  } catch {
    res.writeHead(404).end();
  }
});
await new Promise((r) => server.listen(0, '127.0.0.1', r));
const base = `http://127.0.0.1:${server.address().port}/`;

async function chromePath() {
  if (process.env.CHROMIUM) return process.env.CHROMIUM;
  const root = join(homedir(), '.cache', 'ms-playwright');
  const dirs = (await readdir(root)).filter((d) => d.startsWith('chromium-')).sort().reverse();
  if (!dirs.length) throw new Error('no cached Chromium; set CHROMIUM');
  return join(root, dirs[0], 'chrome-linux64', 'chrome');
}

const failures = [];
const check = (ok, what) => {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${what}`);
  if (!ok) failures.push(what);
};

const browser = await chromium.launch({ executablePath: await chromePath(), args: ['--no-sandbox'] });
const page = await browser.newPage({ viewport: { width: 1400, height: 1000 } });
const errors = [];
page.on('pageerror', (e) => errors.push(String(e)));
page.on('console', (m) => m.type() === 'error' && errors.push(m.text()));

// ---- The mocked Anthropic API: answers by job, records what was sent.
const KEY = 'sk-ant-test-key';
const requests = [];
await page.route('https://api.anthropic.com/**', async (route) => {
  const req = route.request();
  const cors = { 'access-control-allow-origin': '*', 'access-control-allow-headers': '*', 'access-control-allow-methods': 'POST, OPTIONS' };
  if (req.method() === 'OPTIONS') return route.fulfill({ status: 204, headers: cors });
  const body = JSON.parse(req.postData() ?? '{}');
  requests.push({ url: req.url(), headers: await req.allHeaders(), body });
  const sys = typeof body.system === 'string' ? body.system : JSON.stringify(body.system);
  const prompt = body.messages?.[0]?.content ?? '';
  let out;
  if (sys.includes('name its peoples') || sys.includes('name its realms')) {
    const label = sys.includes('realms') ? 'Polity' : 'Language';
    const ids = [...String(prompt).matchAll(new RegExp(`${label} (\\d+):`, 'g'))].map((m) => +m[1]);
    out = { names: ids.map((id) => ({ id, pick: 1, exonym: `Mock ${label} ${id}`, note: 'a mocked note' })) };
  } else if (sys.includes('chronicles')) {
    out = { title: 'The mocked era', text: 'In those years the rivers rose and the people spoke as they always had.' };
  } else {
    out = {
      title: 'The mocked myth',
      clauses: [
        { subj: { pron: 'he' }, verb: 'go', tense: 'past', english: 'He went.' },
        { subj: { pron: 'i' }, verb: 'give', tense: 'future', obj: { noun: 'fire' }, dat: { noun: 'woman', def: true }, english: 'I will give fire to the woman.' },
      ],
    };
  }
  return route.fulfill({
    status: 200,
    headers: { ...cors, 'content-type': 'application/json', 'request-id': 'req_mock' },
    body: JSON.stringify({
      id: 'msg_mock', type: 'message', role: 'assistant', model: body.model,
      content: [{ type: 'text', text: JSON.stringify(out) }],
      stop_reason: 'end_turn', stop_sequence: null, usage: { input_tokens: 10, output_tokens: 20 },
    }),
  });
});

// ---- Load and run a few centuries.
await page.goto(base);
await page.waitForSelector('svg[role="application"] polygon', { timeout: 60000 });
await page.waitForFunction(() => !document.querySelector('.status')?.textContent?.includes('Generating'), null, { timeout: 60000 });
const step = page.getByRole('button', { name: '+100 years' });
for (let i = 0; i < 4; i++) {
  await step.click();
  await page.waitForFunction(() => !document.querySelector('button.primary')?.closest('.controls')?.querySelector('button[disabled]'), null, { timeout: 60000 });
}
await page.waitForFunction(() => document.querySelector('.where')?.textContent?.includes('year 400'), null, { timeout: 60000 });
check(true, 'ran to year 400');

const labels = await page.locator('svg text.place').count();
check(labels >= 5, `place names on the map (${labels})`);
const long = await page.locator('svg text.place').evaluateAll((ts) => ts.filter((t) => [...(t.lastChild?.textContent ?? '').trim()].length > 10).length);
check(long === 0, 'map labels are at most ten letters');

// ---- Sound-change suggestions for a settled hex (a capital's).
const cap = await page.locator('svg circle.capital').first().boundingBox();
if (cap) {
  // Rivers, borders and labels must not swallow clicks on the hex below.
  const hit = await page.evaluate(([x, y]) => document.elementFromPoint(x, y)?.tagName, [cap.x + cap.width / 2, cap.y + cap.height / 2]);
  check(hit === 'polygon', `a click on a capital reaches its hex (${hit})`);
  await page.mouse.click(cap.x + cap.width / 2, cap.y + cap.height / 2);
  await page.getByRole('tab', { name: 'Change rules' }).click();
  await page.getByRole('button', { name: 'Suggest changes here' }).click();
  await page.waitForSelector('ul.sugg li', { timeout: 30000 });
  const n = await page.locator('ul.sugg li').count();
  check(n >= 3, `sound changes suggested (${n})`);
  await page.locator('ul.sugg button.use').first().click();
  await page.waitForSelector('ul.pv li', { timeout: 30000 });
  const src = await page.locator('textarea').inputValue();
  check(src.startsWith('rule ') && (await page.locator('ul.pv li').count()) > 0, `a suggestion loads and previews: ${src.split('\n')[0]}`);
  // Structural interventions: set the word order, shorten codas.
  await page.locator('select[aria-label="Word-order parameter"]').selectOption('clause order');
  const now = (await page.locator('.row .now').nth(1).textContent()) ?? '';
  await page.getByRole('button', { name: 'Set', exact: true }).click();
  await page.waitForFunction(() => /clause order becomes/.test(document.querySelector('.status')?.textContent ?? ''), null, { timeout: 30000 });
  check(true, `word order scheduled (${now.trim()}): ${(await page.locator('.status').textContent())?.trim()}`);
  const shorter = page.getByRole('button', { name: 'Shorter codas' });
  if (await shorter.isEnabled()) {
    await shorter.click();
    await page.waitForFunction(() => /Codas shrink/.test(document.querySelector('.status')?.textContent ?? ''), null, { timeout: 30000 });
    check(true, 'shorter codas scheduled');
  }
  await page.getByRole('button', { name: 'Next generation' }).click();
  await page.getByRole('tab', { name: 'Chronicle' }).click();
  await page.waitForFunction(() => /forced: clause order becomes/.test(document.body.textContent ?? ''), null, { timeout: 60000 });
  check(true, 'the forced word order is in the chronicle');
} else {
  console.log('skip no capital yet');
}

// ---- Dialects: a word map, and a language's local forms in Word lists.
await page.getByRole('button', { name: 'A word', exact: true }).click();
await page.waitForSelector('ul.wlegend li', { timeout: 30000 });
const forms = await page.locator('ul.wlegend li').count();
check(forms >= 1, `word map legend (${forms} entries: ${(await page.locator('.wordmap .maplegend').textContent())?.trim()})`);
await page.getByRole('button', { name: 'by root' }).click();
await page.waitForSelector('ul.wlegend li');
await page.getByRole('button', { name: 'Languages', exact: true }).click();
await page.getByRole('tab', { name: 'Word lists' }).click();
await page.getByRole('button', { name: 'Dialects', exact: true }).click();
await page.waitForSelector('table.dial tbody tr', { timeout: 30000 });
const varied = await page.locator('table.dial td.vars').evaluateAll((tds) => tds.filter((td) => td.querySelectorAll('.var').length > 1).length);
check(varied > 0, `dialect table: ${varied} words with more than one local form`);
await page.getByRole('button', { name: 'Cognates', exact: true }).click();

// ---- Look back: drag to an early frame, release, panels show that moment.
const slider = page.locator('input[aria-label="Show the language map at an earlier time"]');
await slider.evaluate((el) => {
  el.value = '2';
  el.dispatchEvent(new Event('input', { bubbles: true }));
  el.dispatchEvent(new Event('change', { bubbles: true }));
});
await page.waitForFunction(() => /as of year/.test(document.querySelector('.where')?.textContent ?? ''), null, { timeout: 30000 });
await page.waitForFunction(() => !(document.querySelector('.where')?.textContent ?? '').includes('map only'), null, { timeout: 60000 });
const asOf = await page.locator('.where').textContent();
check(/as of year (\d+)/.test(asOf ?? ''), `looking back shows full state (${asOf?.trim()})`);
await page.getByRole('tab', { name: 'Word lists' }).click();
await page.waitForSelector('table', { timeout: 30000 });
check(true, 'word lists render for the past moment');

const fork = page.getByRole('button', { name: /Continue from year/ });
await fork.click();
await page.waitForFunction(() => document.querySelector('.status')?.textContent?.startsWith('Forked at year'), null, { timeout: 60000 });
check(true, `forked: ${(await page.locator('.status').textContent())?.trim()}`);
await step.click();
await page.waitForFunction(() => !document.querySelector('.where')?.textContent?.includes('as of'), null, { timeout: 60000 });
check(true, 'the fork runs on');

// ---- Storyteller, end to end against the mock.
await page.getByRole('tab', { name: 'Stories' }).click();
await page.locator('input[type="password"]').fill(KEY);
await page.getByRole('button', { name: 'Save', exact: true }).click();
await page.getByRole('button', { name: 'Name the peoples' }).click();
await page.waitForFunction(() => !document.body.textContent?.includes('Naming the peoples…'), null, { timeout: 60000 });
const named = await page.locator('table.names').first().locator('td', { hasText: /^Mock Language/ }).count();
check(named > 0, `peoples named from the mock (${named})`);
const warn = await page.locator('.warn').count();
check(warn === 0, `no storyteller error shown${warn ? ': ' + (await page.locator('.warn').textContent()) : ''}`);

const realms = page.getByRole('button', { name: 'Name the realms' });
if (await realms.count()) {
  await realms.click();
  await page.waitForFunction(() => !document.body.textContent?.includes('Naming the realms…'), null, { timeout: 60000 });
  const n = await page.locator('td', { hasText: /^Mock Polity/ }).count();
  check(n > 0, `realms named from the mock (${n})`);
} else {
  console.log('skip no polities yet');
}

await page.getByRole('button', { name: "Write this era's chronicle" }).first().click();
await page.waitForSelector('text=The mocked era', { timeout: 60000 });
check(true, 'a chronicle written');

await page.getByRole('button', { name: 'Tell a myth' }).click();
await page.waitForSelector('text=The mocked myth', { timeout: 60000 });
const lines = await page.locator('.clause .ilg').count();
check(lines > 0, `the myth rendered in every language (${lines} lines)`);

// What the page sent.
const r0 = requests[0];
check(requests.length >= 3, `requests sent (${requests.length})`);
if (r0) {
  check(r0.url.startsWith('https://api.anthropic.com/v1/messages'), `endpoint ${r0.url}`);
  // SDK 1.x sends the key as a bearer token; older ones used x-api-key.
  check(r0.headers['authorization'] === `Bearer ${KEY}` || r0.headers['x-api-key'] === KEY, "the player's key is sent");
  check((r0.headers['anthropic-beta'] ?? '').includes('server-side-fallback-2026-07-01'), `beta header: ${r0.headers['anthropic-beta']}`);
  check(r0.body.model === 'claude-opus-5-5', `model ${r0.body.model}`);
  check(r0.body.fallbacks === 'default', `fallbacks ${JSON.stringify(r0.body.fallbacks)}`);
  check(r0.body.output_config?.effort === 'low', 'effort low');
  check(r0.body.output_config?.format?.type === 'json_schema' && !!r0.body.output_config?.format?.schema, 'structured output schema');
  check(r0.body.messages?.[0]?.role === 'user' && typeof r0.body.system === 'string', 'system and user message');
}

// Replays read the annotations back: no new request after a save round trip.
const before = requests.length;
await page.getByRole('tab', { name: 'Family tree' }).click();
await page.getByRole('tab', { name: 'Stories' }).click();
await page.waitForSelector('text=The mocked era');
check(requests.length === before, 'stored stories are not asked for again');

// ---- Settings: a smaller map and faster sound change, then back to defaults.
await page.getByRole('button', { name: /^Settings/ }).click();
await page.locator('.settings select').first().selectOption('small');
await page.locator('.settings label.sl', { hasText: 'Sound changes' }).locator('input').evaluate((el) => {
  el.value = '5';
  el.dispatchEvent(new Event('input', { bubbles: true }));
});
const label = (await page.getByRole('button', { name: /^Settings/ }).textContent())?.trim();
await page.getByRole('button', { name: 'New world' }).click();
await page.waitForFunction(() => document.querySelectorAll('svg[role="application"] polygon.land, svg[role="application"] polygon:not(.land)').length > 0 && !document.querySelector('.status')?.textContent?.includes('Generating'), null, { timeout: 60000 });
await page.waitForTimeout(500);
const hexes = await page.locator('svg[role="application"] > polygon:not(.sel):not(.wave)').count();
check(label === 'Settings (4 changed)' && hexes === 20 * 15, `custom settings make a small world (${label}; ${hexes} hexes)`);
await page.getByRole('button', { name: 'Reset to defaults' }).click();
await page.getByRole('button', { name: 'New world' }).click();
await page.waitForFunction(() => document.querySelectorAll('svg[role="application"] > polygon:not(.sel):not(.wave)').length === 26 * 20, null, { timeout: 60000 });
check(true, 'defaults restored');

// ---- Timing.
await page.getByRole('button', { name: 'Time 1000 years' }).click();
await page.waitForFunction(() => /simulated in \d+ ms/.test(document.querySelector('.status')?.textContent ?? ''), null, { timeout: 120000 });
console.log('     ' + (await page.locator('.status').textContent())?.trim());

check(errors.length === 0, `no page errors${errors.length ? ': ' + errors.join(' | ') : ''}`);
await browser.close();
server.close();
if (failures.length) {
  console.log(`${failures.length} failed`);
  process.exit(1);
}
