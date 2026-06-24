import React, { useState } from 'react';
import './ChatInput.scss';

const ChatInput = ({ sendMessage, hasUsername, username, connectionStatus }) => {
  const [message, setMessage] = useState('');
  
  const handleSubmit = (e) => {
    e.preventDefault();
    const trimmed = message.trim();
    if (trimmed !== '') {
      sendMessage(trimmed);
      setMessage('');
    }
  };
  
  const canSend = connectionStatus === 'connected';
  const inputId = 'chat-message-input';
  const hintId = 'chat-username-hint';
  const submitLabel = hasUsername ? 'Send' : 'Join chat';

  return (
    <div className={`chat-input ${hasUsername ? 'is-composer' : 'is-setup'}`}>
      {!hasUsername ? (
        <div className="setup-copy">
          <p className="eyebrow">Identity</p>
          <h3>Choose a display name</h3>
          <p id={hintId}>
            This demo has no accounts. Your name is used only for this WebSocket session.
          </p>
        </div>
      ) : (
        <div className="composer-status" aria-live="polite">
          <span>Joined as</span>
          <strong>{username}</strong>
        </div>
      )}
      <form onSubmit={handleSubmit}>
        <label htmlFor={inputId} className="visually-hidden">
          {hasUsername ? 'Chat message' : 'Choose a display name'}
        </label>
        <input
          id={inputId}
          type="text"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder={hasUsername ? 'Type your message…' : 'Enter your username…'}
          disabled={!canSend}
          autoComplete="off"
          autoFocus
          aria-describedby={hasUsername ? undefined : hintId}
        />
        <button type="submit" disabled={message.trim() === '' || !canSend}>
          {submitLabel}
        </button>
      </form>
    </div>
  );
};

export default ChatInput;
