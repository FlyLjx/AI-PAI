const apiBase = process.env.API_BASE || 'http://api:3001';
const model = process.env.MODEL || 'e2e-load-image';
const keys = (process.env.LOAD_KEYS || '').split(',').map((value) => value.trim()).filter(Boolean);
const requestsPerKey = Number(process.env.REQUESTS_PER_KEY || 50);
const editEvery = Number(process.env.EDIT_EVERY || 5);
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lYV0ZQAAAABJRU5ErkJggg==', 'base64');

if (keys.length === 0) throw new Error('LOAD_KEYS is required');

async function submit(key, keyIndex, requestIndex) {
  const edit = editEvery > 0 && requestIndex % editEvery === 0;
  const started = performance.now();
  const headers = { authorization: `Bearer ${key}` };
  let body;
  let endpoint;
  if (edit) {
    endpoint = '/v1/images/edits';
    body = new FormData();
    body.set('model', model);
    body.set('prompt', `fixture edit user ${keyIndex} request ${requestIndex}`);
    body.set('size', '1024x1024');
    body.set('n', '1');
    body.set('image', new Blob([png], { type: 'image/png' }), 'fixture.png');
  } else {
    endpoint = '/v1/images/generations';
    headers['content-type'] = 'application/json';
    body = JSON.stringify({ model, prompt: `fixture generation user ${keyIndex} request ${requestIndex}`, size: '1024x1024', n: 1 });
  }
  const response = await fetch(apiBase + endpoint, { method: 'POST', headers, body });
  const text = await response.text();
  return { keyIndex, edit, status: response.status, durationMs: performance.now() - started, body: text.slice(0, 300) };
}

const started = performance.now();
const pending = [];
for (let keyIndex = 0; keyIndex < keys.length; keyIndex += 1) {
  for (let requestIndex = 0; requestIndex < requestsPerKey; requestIndex += 1) {
    pending.push(submit(keys[keyIndex], keyIndex, requestIndex));
  }
}
const results = await Promise.all(pending);
const durations = results.map((item) => item.durationMs).sort((a, b) => a - b);
const failures = results.filter((item) => item.status < 200 || item.status >= 300);
const percentile = (value) => Math.round(durations[Math.min(durations.length - 1, Math.floor(durations.length * value))] || 0);
const perUser = keys.map((_, keyIndex) => {
  const items = results.filter((item) => item.keyIndex === keyIndex);
  return { keyIndex, success: items.filter((item) => item.status >= 200 && item.status < 300).length, failed: items.filter((item) => item.status < 200 || item.status >= 300).length };
});
console.log(JSON.stringify({
  users: keys.length,
  requests: results.length,
  generations: results.filter((item) => !item.edit).length,
  edits: results.filter((item) => item.edit).length,
  success: results.length - failures.length,
  failed: failures.length,
  elapsedMs: Math.round(performance.now() - started),
  latencyMs: { p50: percentile(0.5), p95: percentile(0.95), p99: percentile(0.99), max: Math.round(durations.at(-1) || 0) },
  perUser,
  failureSamples: failures.slice(0, 10),
}, null, 2));
if (failures.length > 0) process.exitCode = 1;
