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
