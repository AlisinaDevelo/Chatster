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

  return (
    <div className="chat-input">
      <form onSubmit={handleSubmit}>
        <input
          type="text"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder={hasUsername ? "Type your message..." : "Enter your username..."}
          disabled={!canSend}
          autoFocus
        />
        <button type="submit" disabled={message.trim() === '' || !canSend}>
          {hasUsername ? 'Send' : 'Set Username'}
        </button>
      </form>
      {!hasUsername && (
        <p className="hint">Please enter your username to start chatting</p>
      )}
    </div>
  );
};

export default ChatInput; 