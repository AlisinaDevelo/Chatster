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
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };
  
  useEffect(() => {
    scrollToBottom();
  }, [chatHistory]);
  
  const renderMessages = () => {
    return chatHistory.map((msg, index) => {
      // Determine if the message is a notification or a regular message
      const isNotification = msg.type === 'notification';
      
      return (
        <div key={index} className={isNotification ? 'message-notification' : 'message-container'}>
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
