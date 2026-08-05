export const DEFAULT_ROOM = 'general';

export const ROOM_OPTIONS = [
  DEFAULT_ROOM,
  'engineering',
  'off-topic',
];

export function roomFromPath(pathname = window.location.pathname) {
  const match = pathname.match(/^\/rooms\/([^/]+)\/?$/);
  if (!match) {
    return DEFAULT_ROOM;
  }

  try {
    const room = decodeURIComponent(match[1]);
    return ROOM_OPTIONS.includes(room) ? room : DEFAULT_ROOM;
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
