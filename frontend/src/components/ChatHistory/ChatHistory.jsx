import React, { useEffect, useRef } from 'react';
import './ChatHistory.scss';

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
      const messageClass = msg.type === 'notification' 
        ? 'chat-message notification' 
        : 'chat-message';
      
      return (
        <div key={index} className={messageClass}>
          <span className="username">{msg.username}</span>
          <span className="message">{msg.content}</span>
        </div>
      );
    });
  };
  
  return (
    <div className="chat-history">
      <h2>Chat History</h2>
      <div className="message-container">
        {renderMessages()}
        <div ref={messagesEndRef} />
      </div>
    </div>
  );
};

export default ChatHistory;
