import React, { useState, useEffect } from 'react';
// import logo from './logo.svg';
import './App.css';
import { connect, disconnect, fetchRecentMessages, sendMsg } from './api';
import Header from './components/Header/Header';
import ChatHistory from './components/ChatHistory/ChatHistory';
import ChatInput from './components/ChatInput/ChatInput';

function appendUniqueMessages(existing, incoming) {
  const seenIds = new Set(existing.filter((msg) => msg.id != null).map((msg) => msg.id));
  const nextMessages = [...existing];

  incoming.forEach((msg) => {
    if (msg.id != null) {
      if (seenIds.has(msg.id)) {
        return;
      }
      seenIds.add(msg.id);
    }
    nextMessages.push(msg);
  });

  return nextMessages;
}

function App() {
  const [chatHistory, setChatHistory] = useState([]);
  const [username, setUsername] = useState('');
  const [connectionStatus, setConnectionStatus] = useState('connecting');

  const hasUsername = username !== '';

  useEffect(() => {
    connect((msg) => {
      try {
        const parsedMessage = JSON.parse(msg.data);
        setChatHistory((prevChatHistory) => appendUniqueMessages(prevChatHistory, [parsedMessage]));
      } catch (e) {
        console.error('Error parsing message:', e);
      }
    }, setConnectionStatus);
    return () => disconnect();
  }, []);

  useEffect(() => {
    if (connectionStatus !== 'connected') {
      return undefined;
    }

    let cancelled = false;
    fetchRecentMessages(50)
      .then((messages) => {
        if (!cancelled) {
          setChatHistory((prevChatHistory) => appendUniqueMessages(prevChatHistory, messages));
        }
      })
      .catch((e) => {
        console.warn('Error fetching message history:', e);
      });

    return () => {
      cancelled = true;
    };
  }, [connectionStatus]);

  useEffect(() => {
    if (connectionStatus === 'connected' && hasUsername) {
      sendMsg(JSON.stringify({
        type: 'username',
        content: username
      }));
    }
  }, [connectionStatus, hasUsername, username]);

  const send = (message) => {
    if (!hasUsername) {
      setUsername(message);
      setChatHistory(prev => [
        ...prev,
        {
          type: 'notification',
          username: 'System',
          content: `You joined as ${message}`,
          timestamp: new Date().toISOString()
        }
      ]);
    } else {
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
              {connectionStatus === 'disconnected' && 'Reconnecting and catching up…'}
              {connectionStatus === 'error' && 'Connection error — retrying…'}
            </div>
          )}

          <ChatHistory chatHistory={chatHistory} currentUsername={username} />
          <ChatInput
            sendMessage={send}
            hasUsername={hasUsername}
            username={username}
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
