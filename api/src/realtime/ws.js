const WebSocket = require('ws');

// userId -> Set<ws>
const clients = new Map();

function attachWSServer(server) {
  const wss = new WebSocket.Server({ server, path: '/ws' });

  wss.on('connection', (ws, req) => {
    // simple auth via query: /ws?userId=...
    const url = new URL(req.url, 'http://localhost');
    const userId = url.searchParams.get('userId');

    if (!userId) {
      ws.close();
      return;
    }

    if (!clients.has(userId)) clients.set(userId, new Set());
    clients.get(userId).add(ws);

    ws.on('close', () => {
      const set = clients.get(userId);
      if (set) {
        set.delete(ws);
        if (set.size === 0) clients.delete(userId);
      }
    });
  });

  return wss;
}

function sendToUser(userId, payload) {
  const set = clients.get(userId);
  if (!set) return;
  const data = JSON.stringify(payload);
  for (const ws of set) {
    if (ws.readyState === ws.OPEN) {
      ws.send(data);
    }
  }
}

module.exports = { attachWSServer, sendToUser };
