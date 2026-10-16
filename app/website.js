import { readFile, rename, cp, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { build } from 'vite';
import landingConfig from '../landing/vite.config.js';

const landingRoot = fileURLToPath(new URL('../landing/', import.meta.url));
const publicPages = new Map([['/', 'index.html'], ['/index.html', 'index.html'], ['/perangkat.html', 'perangkat.html'], ['/privasi.html', 'privasi.html'], ['/syarat.html', 'syarat.html']]);

export function isApplicationPage(pathname) {
  return /^\/(masuk|dashboard|mulai|receipt|produk|impor-katalog|kasir|shift|supplier|pelanggan|laporan|restoran|rekonsiliasi|ekspor|stok|penerimaan|pengeluaran|pengaturan|keamanan|pajak|audit|anggota|batch|apotek|pesanan|menu-habis|meja|dapur|transaksi)(\/|$)/.test(pathname);
}

export function websitePlugin(mode) {
  const landing = landingConfig({ mode, integrated: true });
  const staticPlugin = landing.plugins[0];
  let output;
  let building = false;
  return {
    name: 'storepos-website',
    configResolved(config) {
      output = path.resolve(config.root, config.build.outDir);
      building = config.command === 'build';
    },
    configureServer(server) {
      staticPlugin.configureServer(server);
      server.watcher.add(landingRoot);
      server.middlewares.use(async (req, res, next) => {
        const pathname = new URL(req.url, 'http://localhost').pathname;
        const page = publicPages.get(pathname);
        if (!page || !['GET', 'HEAD'].includes(req.method)) return next();
        try {
          let html = await readFile(path.join(landingRoot, page), 'utf8');
          html = html.replaceAll('/src/', `/@fs/${landingRoot}src/`);
          html = await staticPlugin.transformIndexHtml(html);
          html = await server.transformIndexHtml(pathname, html);
          res.setHeader('Content-Type', 'text/html; charset=utf-8');
          res.end(req.method === 'HEAD' ? undefined : html);
        } catch (error) { next(error); }
      });
    },
    configurePreviewServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const pathname = new URL(req.url, 'http://localhost').pathname;
        if (!isApplicationPage(pathname) || !['GET', 'HEAD'].includes(req.method)) return next();
        try {
          const html = await readFile(path.join(output, 'admin.html'), 'utf8');
          res.setHeader('Content-Type', 'text/html; charset=utf-8');
          res.end(req.method === 'HEAD' ? undefined : html);
        } catch (error) { next(error); }
      });
    },
    handleHotUpdate({ file, server }) {
      if (file.startsWith(landingRoot) && file.endsWith('.html')) server.ws.send({ type: 'full-reload', path: '*' });
    },
    async closeBundle() {
      if (!building) return;
      await build({ ...landing, configFile: false, mode });
      await rename(path.join(output, 'index.html'), path.join(output, 'admin.html'));
      await cp(path.join(landingRoot, 'dist'), output, { recursive: true });
      await writeFile(path.join(output, '_redirects'), '/* /admin.html 200\n');
    }
  };
}
