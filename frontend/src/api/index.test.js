import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { connect, disconnect, fetchRecentMessages } from './index';

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
      'http://127.0.0.1:8080/api/messages?limit=25&room=engineering'
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

    expect(instances[0].url).toBe('ws://127.0.0.1:8080/ws?room=engineering');
  });
});
