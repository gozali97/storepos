import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer as createHTTPServer, IncomingMessage, ServerResponse } from 'node:http';
import { Socket } from 'node:net';
import { readFile, mkdtemp, writeFile, rm } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { createServer } from 'vite';
import { isApplicationPage, websitePlugin } from '../website.js';

const appRoot = fileURLToPath(new URL('../', import.meta.url));

function request(middleware, url, method = 'GET') {
  const req = new IncomingMessage(new Socket());
  req.url = url;
  req.method = method;
  req.headers = { host: 'localhost:5173', accept: 'text/html' };
  const res = new ServerResponse(req);
  return new Promise((resolve, reject) => {
    const chunks = [];
    res.write = chunk => { chunks.push(Buffer.from(chunk)); return true; };
    res.end = chunk => {
      if (chunk) chunks.push(Buffer.from(chunk));
      resolve({ status: res.statusCode, body: Buffer.concat(chunks).toString() });
    };
    middleware(req, res, error => error ? reject(error) : resolve(null));
  });
}

test('root publik melayani landing, sedangkan login/dashboard/operasional memuat aplikasi', async () => {
  // HTTP/HMR ditangani langsung agar tes routing tidak membutuhkan port jaringan.
  const transport = createHTTPServer();
  const server = await createServer({ configFile: appRoot + 'vite.config.js', root: appRoot, server: { middlewareMode: true, hmr: { server: transport }, watch: null } });
  try {
    for (const url of ['/', '/?utm_source=local', '/index.html']) {
      const result = await request(server.middlewares, url);
      assert.equal(result.status, 200);
      assert.match(result.body, /Usaha berjalan\./);
      assert.match(result.body, /href="\/masuk"/);
      assert.doesNotMatch(result.body, /id="root"|__APP_URL__/);
      const entry = result.body.match(/src="([^"]+\/src\/main\.js)"/)[1];
      const script = await request(server.middlewares, entry);
      assert.equal(script.status, 200);
      assert.match(script.body, /IntersectionObserver/);
    }
    for (const url of ['/masuk', '/dashboard', '/kasir', '/produk', '/pesanan/order-demo', '/receipt']) {
      const result = await request(server.middlewares, url);
      assert.equal(result.status, 200);
      assert.match(result.body, /id="root"/);
      assert.doesNotMatch(result.body, /Usaha berjalan\./);
    }
    for (const url of ['/perangkat.html', '/privasi.html', '/syarat.html', '/previews/dashboard.html']) assert.equal((await request(server.middlewares, url)).status, 200);
  } finally { await server.close(); transport.close(); }
});

test('preview produksi memilih entry admin hanya untuk halaman aplikasi', async t => {
  const output = await mkdtemp(path.join(tmpdir(), 'storepos-routing-'));
  t.after(() => rm(output, { recursive: true, force: true }));
  await writeFile(path.join(output, 'admin.html'), await readFile(appRoot + 'index.html', 'utf8'));
  const plugin = websitePlugin('test');
  plugin.configResolved({ command: 'serve', root: output, build: { outDir: '.' } });
  let handler;
  plugin.configurePreviewServer({ middlewares: { use(middleware) { handler = middleware; } } });
  for (const url of ['/masuk', '/dashboard', '/kasir', '/pesanan/demo', '/receipt']) {
    assert.equal(isApplicationPage(url), true);
    const result = await request(handler, url);
    assert.match(result.body, /id="root"/);
  }
  for (const url of ['/', '/privasi.html', '/api/v1/leads', '/images/icon_web_POS.png', '/dashboard-other']) {
    assert.equal(isApplicationPage(url), false);
    assert.equal(await request(handler, url), null);
  }
});
