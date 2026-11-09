import http from 'node:http';
import {readFile} from 'node:fs/promises';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const root = fileURLToPath(new URL('../../frontend/', import.meta.url));
const types = {'.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.png': 'image/png', '.woff2': 'font/woff2'};
const server = http.createServer(async (request, response) => {
    try {
        const pathname = decodeURIComponent(new URL(request.url, 'http://localhost').pathname);
        if (pathname === '/favicon.ico') { response.writeHead(204).end(); return; }
        if (pathname === '/test-icon.svg') {
            response.writeHead(200, {'Content-Type': 'image/svg+xml'});
            response.end('<svg xmlns="http://www.w3.org/2000/svg" width="60" height="60"><rect width="60" height="60" fill="#456789"/></svg>');
            return;
        }
        const filename = path.resolve(root, `.${pathname === '/' ? '/index.html' : pathname}`);
        if (!filename.startsWith(root) || request.method !== 'GET') {
            response.writeHead(403).end();
            return;
        }
        const contents = await readFile(filename);
        response.writeHead(200, {'Content-Type': types[path.extname(filename)] || 'application/octet-stream', 'Cache-Control': 'no-store'});
        response.end(contents);
    } catch (error) {
        if (error.code !== 'ENOENT') console.error(`Test server: ${error.message}`);
        response.writeHead(error.code === 'ENOENT' ? 404 : 500).end();
    }
});
server.on('error', error => { console.error(error); process.exitCode = 1; });
server.listen(4173, '127.0.0.1');
for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, () => server.close());
