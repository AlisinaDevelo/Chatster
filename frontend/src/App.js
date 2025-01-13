import React, { useState, useEffect } from 'react';
// import logo from './logo.svg';
import './App.css';
import { connect, sendMsg } from './api';
import Header from './components/Header/Header';
import ChatHistory from './components/ChatHistory/ChatHistory';
import ChatInput from './components/ChatInput/ChatInput';

function App() {
  const [chatHistory, setChatHistory] = useState([]);
  const [hasUsername, setHasUsername] = useState(false);
  const [connectionStatus, setConnectionStatus] = useState('connecting');

  useEffect(() => {
    connect((msg) => {
      try {
        const parsedMessage = JSON.parse(msg.data);
        setChatHistory(prevChatHistory => [...prevChatHistory, parsedMessage]);
      } catch (e) {
        console.error('Error parsing message:', e);
      }
    }, setConnectionStatus);
  }, []);

  const send = (message) => {
    if (!hasUsername) {
      // If this is the first message, it's setting a username
      sendMsg(JSON.stringify({
        type: 'username',
        content: message
      }));
      setHasUsername(true);
      
      // Add local message to show username set
      setChatHistory(prev => [
        ...prev, 
        {
          type: 'notification',
          username: 'System',
          content: `You joined as ${message}`
        }
      ]);
    } else {
      // Regular chat message
      sendMsg(JSON.stringify({
        type: 'message',
        content: message
      }));
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-b from-gray-50 to-gray-100">
      <div className="max-w-3xl mx-auto px-4 py-6">
        <Header />
        
        <div className="bg-white rounded-xl shadow-lg overflow-hidden border border-gray-100">
          {connectionStatus !== 'connected' && (
            <div className="bg-yellow-50 px-4 py-2 text-sm text-yellow-700 flex items-center justify-center">
              {connectionStatus === 'connecting' ? 'Connecting to server...' : 'Disconnected from server'}
            </div>
          )}
          
          <div className="p-4 sm:p-6">
            <ChatHistory chatHistory={chatHistory} />
            <ChatInput sendMessage={send} hasUsername={hasUsername} />
          </div>
        </div>
        
        <footer className="mt-8 text-center text-sm text-gray-500">
          <p>Chatster © {new Date().getFullYear()} - Real-time chat application</p>
        </footer>
      </div>
    </div>
  );
}

/* function App() {
  return (
    <div className="App">
      <header className="App-header">
        <img src={logo} className="App-logo" alt="logo" />
        <p>
          Edit <code>src/App.js</code> and save to reload.
        </p>
        <a
          className="App-link"
          href="https://reactjs.org"
          target="_blank"
          rel="noopener noreferrer"
        >
          Learn React
        </a>
      </header>
    </div>
  );
} */

export default App;
