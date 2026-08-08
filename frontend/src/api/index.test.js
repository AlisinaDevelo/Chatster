import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import {
  connect,
  disconnect,
  fetchRecentMessages,
  fetchSession,
  loginSession,
  logoutSession,
} from './index';

describe('api room routing', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ messages: [] }),
    }));
  });

  afterEach(() => {
    disconnect();
    vi.unstubAllGlobals();
  });

  test('requests history for the selected room', async () => {
    await fetchRecentMessages(25, 'engineering');

    expect(fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/messages?limit=25&room=engineering',
      { credentials: 'include' }
    );
  });

  test('opens the websocket in the selected room', () => {
    const instances = [];
    class FakeWebSocket {
      static OPEN = 1;

      constructor(url) {
        this.url = url;
        this.readyState = 0;
        instances.push(this);
      }

      close() {}
    }

    vi.stubGlobal('WebSocket', FakeWebSocket);
    connect(vi.fn(), vi.fn(), 'engineering');

    expect(instances[0].url).toBe('ws://localhost:8080/ws?room=engineering');
  });

  test('reads session state with cookies enabled', async () => {
    fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ mode: 'session', authenticated: false }),
    });

    await expect(fetchSession()).resolves.toEqual({ mode: 'session', authenticated: false });
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/session', {
      credentials: 'include',
    });
  });

  test('exchanges a runtime token without putting it in the URL', async () => {
    fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ mode: 'session', authenticated: true, user: { user_id: 'usr_alice' } }),
    });

    await loginSession('secret-token');

    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/session', {
      method: 'POST',
      credentials: 'include',
      headers: { Authorization: 'Bearer secret-token' },
    });
  });

  test('logs out by expiring the server session cookie', async () => {
    fetch.mockResolvedValueOnce({ ok: true });

    await logoutSession();

    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/session', {
      method: 'DELETE',
      credentials: 'include',
    });
  });
});
