import http from 'node:http';

const port = Number(process.env.PORT || 8080);
const delayMs = Number(process.env.DELAY_MS || 150);
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lYV0ZQAAAABJRU5ErkJggg==', 'base64');
let active = 0;
let maxActive = 0;
let total = 0;
let generations = 0;
let edits = 0;
let sequence = 0;

function sendJSON(response, status, body) {
  response.writeHead(status, { 'content-type': 'application/json' });
  response.end(JSON.stringify(body));
}

function requestedQuantity(request, body) {
  const contentType = request.headers['content-type'] || '';
  if (contentType.includes('application/json')) {
    try {
      return Math.max(1, Number(JSON.parse(body.toString()).n || 1));
    } catch {
      return 1;
    }
  }
  const match = body.toString('latin1').match(/name="n"\r\n\r\n(\d+)/);
  return Math.max(1, Number(match?.[1] || 1));
}

const server = http.createServer((request, response) => {
  if (request.method === 'GET' && request.url === '/metrics') {
    sendJSON(response, 200, { active, maxActive, total, generations, edits });
    return;
  }
  if (request.method === 'GET' && request.url?.startsWith('/images/')) {
    response.writeHead(200, { 'content-type': 'image/png', 'content-length': png.length });
    response.end(png);
    return;
  }
  if (request.method !== 'POST' || !request.url?.match(/^\/v1\/images\/(generations|edits)$/)) {
    sendJSON(response, 404, { error: { message: 'fixture route not found' } });
    return;
  }

  const chunks = [];
  request.on('data', (chunk) => chunks.push(chunk));
  request.on('end', async () => {
    active += 1;
    maxActive = Math.max(maxActive, active);
    total += 1;
    if (request.url.endsWith('/edits')) edits += 1;
    else generations += 1;
    try {
      const quantity = requestedQuantity(request, Buffer.concat(chunks));
      await new Promise((resolve) => setTimeout(resolve, delayMs));
      const data = Array.from({ length: quantity }, () => {
        sequence += 1;
        return { url: `http://aipai-load-upstream:${port}/images/${sequence}.png` };
      });
      sendJSON(response, 200, { created: Math.floor(Date.now() / 1000), data });
    } finally {
      active -= 1;
    }
  });
});

server.listen(port, '0.0.0.0', () => {
  console.log(`fake image upstream listening on ${port}`);
});
