import React, { useState } from 'react';
import './ChatInput.scss';

const ChatInput = ({ sendMessage, hasUsername }) => {
  const [message, setMessage] = useState('');
  
  const handleSubmit = (e) => {
    e.preventDefault();
    if (message.trim() !== '') {
      sendMessage(message);
      setMessage('');
    }
  };
  
  return (
    <div className="chat-input">
      <form onSubmit={handleSubmit}>
        <input
          type="text"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder={hasUsername ? "Type a message..." : "Enter username..."}
          disabled={!hasUsername && message !== ''}
        />
        <button type="submit" disabled={message.trim() === ''}>
          {hasUsername ? 'Send' : 'Set Username'}
        </button>
      </form>
    </div>
  );
};

export default ChatInput; 