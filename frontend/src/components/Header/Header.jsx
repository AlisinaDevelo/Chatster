import React from 'react';
import "./Header.scss";

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

const Header = ({ connectionStatus }) => {
  const live = connectionStatus === 'connected';

  return (
    <header className="header" role="banner">
      <div className="header-content">
        <div className="header-brand">
          <h1 className="header-title">Chatster</h1>
          <p className="header-sub">WebSocket · SQLite</p>
        </div>
        <div className={`header-status ${live ? 'is-live' : 'is-muted'}`}>
          <span className="header-status-dot" aria-hidden />
          <span className="header-status-text">{statusLabel(connectionStatus)}</span>
        </div>
      </div>
    </header>
  );
};

export default Header;

