export const DEFAULT_ROOM = 'general';

export const ROOM_OPTIONS = [
  DEFAULT_ROOM,
  'engineering',
  'off-topic',
];

const roomPattern = /^[a-z0-9][a-z0-9_-]{0,31}$/;

export function roomFromPath(pathname = window.location.pathname) {
  const match = pathname.match(/^\/rooms\/([^/]+)\/?$/);
  if (!match) {
    return DEFAULT_ROOM;
  }

  try {
    const room = decodeURIComponent(match[1]).trim().toLowerCase();
    return roomPattern.test(room) ? room : DEFAULT_ROOM;
  } catch {
    return DEFAULT_ROOM;
  }
}

export function roomPath(room) {
  return `/rooms/${encodeURIComponent(room)}`;
}

export function roomLabel(room) {
  return `#${room}`;
}
