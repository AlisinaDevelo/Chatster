import React, { useEffect, useRef } from 'react';
import './ChatHistory.scss';

const formatTime = (timestamp) => {
  // If there's a timestamp (from DB), use it, otherwise use current time
  const date = timestamp ? new Date(timestamp) : new Date();
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
};

const ChatHistory = ({ chatHistory, currentUsername }) => {
  const messagesEndRef = useRef(null);
  
  const scrollToBottom = () => {
    const el = messagesEndRef.current;
    if (el && typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ behavior: 'smooth' });
    }
  };
  
  useEffect(() => {
    scrollToBottom();
  }, [chatHistory]);
  
  const renderMessages = () => {
    return chatHistory.map((msg, index) => {
      const isNotification = msg.type === 'notification';
      const isOwn = !isNotification && currentUsername && msg.username === currentUsername;
      const key =
        msg.id != null ? `msg-${msg.id}` : `local-${index}-${msg.username}-${msg.content?.slice(0, 16)}`;

      return (
        <div
          key={key}
          className={isNotification ? 'message-notification' : `message-container ${isOwn ? 'is-own' : ''}`}
        >
          {isNotification ? (
            <div className="notification">
              <span>{msg.content}</span>
              {msg.timestamp && <time>{formatTime(msg.timestamp)}</time>}
            </div>
          ) : (
            <div className="message">
              <div className="message-header">
                <span className="username">{isOwn ? 'You' : msg.username}</span>
                <time className="timestamp">{formatTime(msg.timestamp)}</time>
              </div>
              <div className="message-content">
                {msg.content}
              </div>
            </div>
          )}
        </div>
      );
    });
  };
  
  return (
    <section className="chat-history" aria-labelledby="chat-heading">
      <div className="chat-header">
        <h2 id="chat-heading">Chat History</h2>
        <span className="message-count" aria-live="polite">
          {chatHistory.length} messages
        </span>
      </div>

      <div
        className="messages"
        role="log"
        aria-live="polite"
        aria-relevant="additions"
        aria-label="Chat messages"
      >
        {chatHistory.length > 0 ? renderMessages() : (
          <div className="no-messages">
            No messages yet. Start the conversation!
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>
    </section>
  );
};

export default ChatHistory;
