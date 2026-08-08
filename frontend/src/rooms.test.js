import { describe, expect, test } from 'vitest';
import { DEFAULT_ROOM, roomFromPath } from './rooms';

describe('roomFromPath', () => {
  test('preserves valid server-supported room names', () => {
    expect(roomFromPath('/rooms/Incident-Response')).toBe('incident-response');
  });

  test('falls back for malformed room names', () => {
    expect(roomFromPath('/rooms/not%2Fa%2Froom')).toBe(DEFAULT_ROOM);
    expect(roomFromPath(`/rooms/${'a'.repeat(33)}`)).toBe(DEFAULT_ROOM);
  });
});
