import React, { useState, useEffect } from 'react';
// import logo from './logo.svg';
import './App.css';
import { connect, disconnect, sendMsg } from './api';
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
        setChatHistory((prevChatHistory) => [...prevChatHistory, parsedMessage]);
      } catch (e) {
        console.error('Error parsing message:', e);
      }
    }, setConnectionStatus);
    return () => disconnect();
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
    <div className="App">
      <a href="#main-content" className="skip-link">
        Skip to chat
      </a>
      <Header connectionStatus={connectionStatus} />

      <main id="main-content" className="chat-main" tabIndex={-1}>
        <div className="chat-container">
          {connectionStatus !== 'connected' && (
            <div className="connection-status" role="status" aria-live="polite">
              {connectionStatus === 'connecting' && 'Connecting…'}
              {connectionStatus === 'disconnected' && 'Reconnecting…'}
              {connectionStatus === 'error' && 'Connection error — retrying…'}
            </div>
          )}

          <ChatHistory chatHistory={chatHistory} />
          <ChatInput
            sendMessage={send}
            hasUsername={hasUsername}
            connectionStatus={connectionStatus}
          />
        </div>
      </main>

      <footer className="footer">
        <p>Chatster © {new Date().getFullYear()} — Real-time chat application</p>
      </footer>
    </div>
  );
}

export default App;
