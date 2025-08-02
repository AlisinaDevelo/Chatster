import React, { useState } from 'react';
import './ChatInput.scss';

const ChatInput = ({ sendMessage, hasUsername, connectionStatus }) => {
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

  return (
    <div className="chat-input">
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
          {hasUsername ? 'Send' : 'Set Username'}
        </button>
      </form>
      {!hasUsername && (
        <p id={hintId} className="hint">
          Please enter your username to start chatting
        </p>
      )}
    </div>
  );
};

export default ChatInput; 