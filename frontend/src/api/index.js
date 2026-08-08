import { DEFAULT_ROOM } from '../rooms';

const buildEnv = import.meta.env;

function envValue(primaryName, legacyName) {
  return buildEnv[primaryName] || buildEnv[legacyName];
}

function withRoom(baseUrl, room) {
  const separator = baseUrl.includes('?') ? '&' : '?';
  return `${baseUrl}${separator}room=${encodeURIComponent(room)}`;
}

function defaultWsUrl(room = DEFAULT_ROOM) {
  const configuredUrl = envValue('VITE_WS_URL', 'REACT_APP_WS_URL');
  if (configuredUrl) {
    return withRoom(configuredUrl, room);
  }

  if (buildEnv.DEV) {
    const port = envValue('VITE_WS_PORT', 'REACT_APP_WS_PORT') || '8080';
    const host = window.location.hostname || '127.0.0.1';
    return withRoom(`ws://${host}:${port}/ws`, room);
  }

  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return withRoom(`${proto}//${window.location.host}/ws`, room);
}

function defaultApiUrl() {
  const configuredUrl = envValue('VITE_API_URL', 'REACT_APP_API_URL');
  if (configuredUrl) {
    return configuredUrl.replace(/\/$/, '');
  }

  if (buildEnv.DEV) {
    const port = envValue('VITE_API_PORT', 'REACT_APP_API_PORT') || '8080';
    const host = window.location.hostname || '127.0.0.1';
    return `http://${host}:${port}`;
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

export function connect(onMessage, setConnectionStatus, room = DEFAULT_ROOM) {
  disconnect();

  if (setConnectionStatus) {
    setConnectionStatus('connecting');
  }

  socket = new WebSocket(defaultWsUrl(room));

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
      connect(onMessage, setConnectionStatus, room);
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

export async function fetchRecentMessages(limit = 50, room = DEFAULT_ROOM) {
  const params = new URLSearchParams({
    limit: String(limit),
    room,
  });
  const response = await fetch(`${defaultApiUrl()}/api/messages?${params.toString()}`, {
    credentials: 'include',
  });
  if (!response.ok) {
    throw new Error(`message history request failed: ${response.status}`);
  }

  const payload = await response.json();
  return Array.isArray(payload) ? payload : payload.messages || [];
}

export async function fetchSession() {
  const response = await fetch(`${defaultApiUrl()}/api/session`, {
    credentials: 'include',
  });
  if (!response.ok) {
    throw new Error(`session request failed: ${response.status}`);
  }
  return response.json();
}

export async function loginSession(token) {
  const response = await fetch(`${defaultApiUrl()}/api/session`, {
    method: 'POST',
    credentials: 'include',
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!response.ok) {
    throw new Error(response.status === 401 ? 'Access token not recognized.' : 'Sign in failed.');
  }
  return response.json();
}

export async function logoutSession() {
  const response = await fetch(`${defaultApiUrl()}/api/session`, {
    method: 'DELETE',
    credentials: 'include',
  });
  if (!response.ok) {
    throw new Error('Sign out failed.');
  }
}
