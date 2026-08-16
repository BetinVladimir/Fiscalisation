import net from 'node:net';
import http from 'node:http';

const messages = [];
const smtpPort = Number(process.env.SMTP_PORT || 2525);
const httpPort = Number(process.env.HTTP_PORT || 8080);
const reply = (socket, line) => socket.write(`${line}\r\n`);

net.createServer((socket) => {
  let buffer = '', dataMode = false, data = [], envelope = { from: '', to: [] };
  reply(socket, '220 smtp-capture ESMTP');
  socket.on('data', chunk => {
    buffer += chunk.toString('utf8');
    while (buffer.includes('\n')) {
      const split = buffer.indexOf('\n');
      const line = buffer.slice(0, split).replace(/\r$/, '');
      buffer = buffer.slice(split + 1);
      if (dataMode) {
        if (line === '.') {
          const raw = data.join('\n');
          const [headerBlock = '', ...bodyParts] = raw.split(/\n\n/);
          const headers = Object.fromEntries(headerBlock.split('\n').map(x => {
            const i = x.indexOf(':'); return i < 0 ? ['', ''] : [x.slice(0, i).toLowerCase(), x.slice(i + 1).trim()];
          }).filter(([k]) => k));
          messages.push({ id: messages.length + 1, at: new Date().toISOString(), envelope, subject: headers.subject || '', headers, body: bodyParts.join('\n\n'), raw });
          dataMode = false; data = []; envelope = { from: '', to: [] }; reply(socket, '250 queued');
        } else data.push(line.startsWith('..') ? line.slice(1) : line);
        continue;
      }
      const upper = line.toUpperCase();
      if (upper.startsWith('EHLO') || upper.startsWith('HELO')) reply(socket, '250 smtp-capture');
      else if (upper.startsWith('MAIL FROM:')) { envelope.from = line.slice(10).trim().replace(/[<>]/g, ''); reply(socket, '250 ok'); }
      else if (upper.startsWith('RCPT TO:')) { envelope.to.push(line.slice(8).trim().replace(/[<>]/g, '')); reply(socket, '250 ok'); }
      else if (upper === 'DATA') { dataMode = true; reply(socket, '354 end with <CRLF>.<CRLF>'); }
      else if (upper === 'RSET') { dataMode = false; data = []; envelope = { from: '', to: [] }; reply(socket, '250 reset'); }
      else if (upper === 'QUIT') { reply(socket, '221 bye'); socket.end(); }
      else reply(socket, '250 ok');
    }
  });
}).listen(smtpPort, '0.0.0.0');

http.createServer((req, res) => {
  const url = new URL(req.url, 'http://localhost');
  if (url.pathname === '/healthz') return json(res, 200, { status: 'ok' });
  if (url.pathname === '/reset' && req.method === 'POST') { messages.length = 0; return json(res, 204); }
  if (url.pathname === '/messages' && req.method === 'GET') {
    const to = url.searchParams.get('to'), subject = url.searchParams.get('subject');
    const items = messages.filter(x => (!to || x.envelope.to.includes(to)) && (!subject || x.subject.toLowerCase().includes(subject.toLowerCase())));
    return json(res, 200, { items });
  }
  json(res, 404, { error: 'not_found' });
}).listen(httpPort, '0.0.0.0');

function json(res, status, value) {
  res.writeHead(status, { 'content-type': 'application/json' });
  if (status !== 204) res.end(JSON.stringify(value)); else res.end();
}
