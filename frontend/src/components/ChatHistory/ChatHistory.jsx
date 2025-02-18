import React, { useEffect, useRef } from 'react';
import './ChatHistory.scss';

const formatTime = (timestamp) => {
  // If there's a timestamp (from DB), use it, otherwise use current time
  const date = timestamp ? new Date(timestamp) : new Date();
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
};

const ChatHistory = ({ chatHistory }) => {
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
      const key =
        msg.id != null ? `msg-${msg.id}` : `local-${index}-${msg.username}-${msg.content?.slice(0, 16)}`;

      return (
        <div key={key} className={isNotification ? 'message-notification' : 'message-container'}>
          {isNotification ? (
            <div className="notification">
              {msg.content}
            </div>
          ) : (
            <div className="message">
              <div className="message-header">
                <span className="username">{msg.username}</span>
                <span className="timestamp">{formatTime(msg.timestamp)}</span>
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
    <div className="chat-history">
      <div className="chat-header">
        <h2>Chat History</h2>
        <span className="message-count">{chatHistory.length} messages</span>
      </div>
      
      <div className="messages">
        {chatHistory.length > 0 ? renderMessages() : (
          <div className="no-messages">
            No messages yet. Start the conversation!
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>
    </div>
  );
};

export default ChatHistory;
