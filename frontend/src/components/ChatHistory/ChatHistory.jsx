import React, { useEffect, useRef } from 'react';
import { FaBell, FaUser } from 'react-icons/fa';

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
        <div key={index} className={`mb-4 ${isNotification ? 'flex justify-center' : ''}`}>
          {isNotification ? (
            <div className="bg-blue-50 px-4 py-2 rounded-full text-sm text-blue-700 flex items-center shadow-sm">
              <FaBell className="mr-2 text-blue-500" />
              <span>{msg.content}</span>
            </div>
          ) : (
            <div className="flex items-start mb-4 last:mb-0">
              <div className="bg-primary bg-opacity-10 rounded-full p-2 mr-3">
                <FaUser className="text-primary" />
              </div>
              <div className="flex-1">
                <div className="flex items-center mb-1">
                  <span className="font-semibold text-gray-800">{msg.username}</span>
                  <span className="text-xs text-gray-500 ml-2">
                    {new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                  </span>
                </div>
                <div className="bg-white rounded-lg p-3 shadow-message text-gray-700">
                  {msg.content}
                </div>
              </div>
            </div>
          )}
        </div>
      );
    });
  };
  
  return (
    <div className="bg-gray-50 rounded-lg shadow-soft p-4">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold text-gray-800">Chat History</h2>
        <span className="text-xs text-gray-500 bg-gray-200 px-2 py-1 rounded-full">
          {chatHistory.length} messages
        </span>
      </div>
      
      <div className="h-80 overflow-y-auto pr-2 space-y-2 scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-gray-100">
        {chatHistory.length > 0 ? renderMessages() : (
          <div className="text-center text-gray-500 py-8">
            No messages yet. Start the conversation!
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>
    </div>
  );
};

export default ChatHistory;
