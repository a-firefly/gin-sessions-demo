import http from 'node:http';
import { readFile } from 'node:fs/promises';

const port = Number(process.env.PORT || 3000);
const backend = new URL(process.env.BACKEND_URL || 'http://localhost:8080');
const page = await readFile(new URL('./index.html', import.meta.url));

const server = http.createServer((req, res) => {
  const pathname = new URL(req.url, 'http://localhost').pathname;

  if (['/hello', '/login', '/profile'].includes(pathname)) {
    const upstream = http.request(new URL(req.url, backend), {
      method: req.method,
      headers: { ...req.headers, host: backend.host },
    }, (response) => {
      res.writeHead(response.statusCode, response.headers);
      response.pipe(res);
      response.on('error', () => res.destroy());
    });
    upstream.setTimeout(10000, () => upstream.destroy(new Error('后端请求超时')));
    upstream.on('error', () => {
      if (res.headersSent) return res.destroy();
      res.writeHead(502, { 'Content-Type': 'application/json; charset=utf-8' });
      res.end(JSON.stringify({ error: '无法连接后端，请确认后端已在 ' + backend.origin + ' 启动' }));
    });
    req.pipe(upstream);
    return;
  }

  if (pathname === '/' || pathname === '/index.html') {
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    res.end(page);
    return;
  }

  res.writeHead(404);
  res.end('Not Found');
});

server.on('error', (error) => {
  console.error('启动失败：', error.message);
  process.exit(1);
});
server.listen(port, '127.0.0.1', () => {
  console.log(`前端：http://localhost:${port}`);
  console.log(`接口代理：/hello、/login、/profile -> ${backend.origin}`);
});
