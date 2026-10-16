import { defineConfig, loadEnv } from '../app/node_modules/vite/dist/node/index.js';
import { fileURLToPath } from 'node:url';
import { productPreviews } from './scripts/product-previews.mjs';

const root = fileURLToPath(new URL('.', import.meta.url));
export default defineConfig(({ mode, integrated = false }) => {
  const env = loadEnv(mode, root, 'STOREPOS_');
  const site = env.STOREPOS_SITE_URL ? new URL(env.STOREPOS_SITE_URL) : null;
  const app = new URL(env.STOREPOS_APP_URL || 'http://localhost:5173/masuk');
  for (const url of [site, app].filter(Boolean)) {
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) throw new Error('URL situs/aplikasi harus HTTP/HTTPS tanpa kredensial.');
  }
  if (site && (site.pathname !== '/' || site.search || site.hash)) throw new Error('STOREPOS_SITE_URL harus domain tanpa subpath, query, atau hash.');
  const siteURL = site?.origin;
  const appURL = integrated ? '/masuk' : app.href;
  return {
    root,
    plugins: [{
      name: 'storepos-static-content',
      configureServer(server) {
        server.middlewares.use(async (req, res, next) => {
          const name = req.url?.match(/^\/previews\/(dashboard|reports|stock|receipt)\.html$/)?.[1];
          if (!name) return next();
          try {
            const previews = await productPreviews();
            res.setHeader('Content-Type', 'text/html; charset=utf-8');
            res.end(previews[name]);
          } catch (error) { next(error); }
        });
      },
      async transformIndexHtml(html) {
        html = html.replaceAll('__APP_URL__', appURL.replaceAll('&', '&amp;').replaceAll('"', '&quot;'));
        return html.replace('<!-- site:metadata -->', siteURL
          ? `<link rel="canonical" href="${siteURL}/"><meta property="og:url" content="${siteURL}/"><meta property="og:image" content="${siteURL}/images/logo_header_POS.png">`
          : '<meta name="robots" content="noindex,follow">');
      },
      async generateBundle() {
        for (const [name, source] of Object.entries(await productPreviews())) this.emitFile({ type: 'asset', fileName: `previews/${name}.html`, source });
        this.emitFile({ type: 'asset', fileName: 'robots.txt', source: siteURL
          ? `User-agent: *\nAllow: /\nSitemap: ${siteURL}/sitemap.xml\n`
          : 'User-agent: *\nDisallow: /\n' });
        if (siteURL) this.emitFile({ type: 'asset', fileName: 'sitemap.xml', source: `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>${siteURL}/</loc></url><url><loc>${siteURL}/perangkat.html</loc></url></urlset>` });
      }
    }],
    server: { port: 5174, proxy: { '/api': { target: env.STOREPOS_API_URL || 'http://127.0.0.1:8090', changeOrigin: true } } },
    build: { outDir: 'dist', rollupOptions: { input: { home: root + 'index.html', devices: root + 'perangkat.html', privacy: root + 'privasi.html', terms: root + 'syarat.html' } } }
  };
});
