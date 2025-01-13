import React, { useState } from 'react';
import { FaPaperPlane, FaUser } from 'react-icons/fa';

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
    <div className="mt-4">
      <form onSubmit={handleSubmit} className="flex items-center">
        {!hasUsername && (
          <div className="mr-2 text-primary">
            <FaUser className="w-5 h-5" />
          </div>
        )}
        <div className="relative flex-1">
          <input
            type="text"
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            placeholder={hasUsername ? "Type your message..." : "Enter your username..."}
            className="w-full bg-white rounded-full py-3 pl-4 pr-10 border border-gray-200 focus:outline-none focus:border-primary focus:ring-2 focus:ring-primary/20 shadow-sm transition"
            disabled={!hasUsername && message !== ''}
            autoFocus
          />
          {message.trim() !== '' && (
            <button type="submit" className="absolute right-1 top-1/2 -translate-y-1/2 bg-primary text-white p-2 rounded-full hover:bg-primary/90 transition">
              <FaPaperPlane className="w-4 h-4" />
            </button>
          )}
        </div>
        {message.trim() === '' && (
          <button 
            type="submit" 
            disabled={message.trim() === ''} 
            className="ml-2 bg-primary text-white px-4 py-3 rounded-full hover:bg-primary/90 transition disabled:opacity-50 disabled:cursor-not-allowed shadow-sm hidden sm:block"
          >
            {hasUsername ? 'Send' : 'Set Username'}
          </button>
        )}
      </form>
      {!hasUsername && (
        <p className="mt-2 text-sm text-gray-500 text-center">
          Please enter your username to start chatting
        </p>
      )}
    </div>
  );
};

export default ChatInput; 