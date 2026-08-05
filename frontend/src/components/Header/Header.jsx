import React from 'react';
import "./Header.scss";
import { ROOM_OPTIONS, roomLabel } from '../../rooms';

const statusLabel = (s) => {
  switch (s) {
    case 'connected':
      return 'Live';
    case 'connecting':
      return 'Connecting';
    case 'disconnected':
      return 'Reconnecting';
    case 'error':
      return 'Error';
    default:
      return 'Offline';
  }
};

const Header = ({
  connectionStatus,
  onRoomChange = () => {},
  room = 'general',
  roomOptions = ROOM_OPTIONS,
}) => {
  const live = connectionStatus === 'connected';

  return (
    <header className="header" role="banner">
      <div className="header-content">
        <div className="header-brand">
          <h1 className="header-title">Chatster</h1>
          <p className="header-sub">WebSocket · SQLite</p>
        </div>
        <div className="header-controls">
          <label className="room-picker">
            <span className="visually-hidden">Chat room</span>
            <select
              aria-label="Chat room"
              value={room}
              onChange={(event) => onRoomChange(event.target.value)}
            >
              {roomOptions.map((option) => (
                <option key={option} value={option}>
                  {roomLabel(option)}
                </option>
              ))}
            </select>
          </label>
          <div className={`header-status ${live ? 'is-live' : 'is-muted'}`}>
            <span className="header-status-dot" aria-hidden />
            <span className="header-status-text">{statusLabel(connectionStatus)}</span>
          </div>
        </div>
      </div>
    </header>
  );
};

export default Header;
