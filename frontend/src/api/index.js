function defaultWsUrl() {
  if (process.env.REACT_APP_WS_URL) {
    return process.env.REACT_APP_WS_URL;
  }
  if (process.env.NODE_ENV === 'development') {
    const port = process.env.REACT_APP_WS_PORT || '8080';
    return `ws://127.0.0.1:${port}/ws`;
  }
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${window.location.host}/ws`;
}

function defaultApiUrl() {
  if (process.env.REACT_APP_API_URL) {
    return process.env.REACT_APP_API_URL.replace(/\/$/, '');
  }
  if (process.env.NODE_ENV === 'development') {
    const port = process.env.REACT_APP_API_PORT || '8080';
    return `http://127.0.0.1:${port}`;
  }
  return window.location.origin;
}

let socket = null;
let reconnectTimer = null;

function clearReconnect() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
}

export function disconnect() {
  clearReconnect();
  if (socket) {
    socket.onopen = null;
    socket.onmessage = null;
    socket.onclose = null;
    socket.onerror = null;
    try {
      socket.close();
    } catch {
      /* ignore */
    }
    socket = null;
  }
}

export function connect(onMessage, setConnectionStatus) {
  disconnect();

  if (setConnectionStatus) {
    setConnectionStatus('connecting');
  }

  socket = new WebSocket(defaultWsUrl());

  socket.onopen = () => {
    if (setConnectionStatus) {
      setConnectionStatus('connected');
    }
  };

  socket.onmessage = (msg) => {
    onMessage(msg);
  };

  socket.onclose = () => {
    if (setConnectionStatus) {
      setConnectionStatus('disconnected');
    }
    clearReconnect();
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null;
      connect(onMessage, setConnectionStatus);
    }, 2000);
  };

  socket.onerror = () => {
    if (setConnectionStatus) {
      setConnectionStatus('error');
    }
  };
}

export function sendMsg(msg) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(msg);
  }
}

export async function fetchRecentMessages(limit = 50) {
  const params = new URLSearchParams({ limit: String(limit) });
  const response = await fetch(`${defaultApiUrl()}/api/messages?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`message history request failed: ${response.status}`);
  }

  const payload = await response.json();
  return Array.isArray(payload) ? payload : payload.messages || [];
}
