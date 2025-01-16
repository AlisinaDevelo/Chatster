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
    <div className="App">
      <Header />
      
      <div className="chat-container">
        {connectionStatus !== 'connected' && (
          <div className="connection-status">
            {connectionStatus === 'connecting' ? 'Connecting to server...' : 'Disconnected from server'}
          </div>
        )}
        
        <ChatHistory chatHistory={chatHistory} />
        <ChatInput sendMessage={send} hasUsername={hasUsername} />
      </div>
      
      <footer className="footer">
        <p>Chatster © {new Date().getFullYear()} - Real-time chat application</p>
      </footer>
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
