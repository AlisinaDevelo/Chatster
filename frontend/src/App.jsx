import React, { useState, useEffect } from 'react';
// import logo from './logo.svg';
import './App.css';
import {
  connect,
  disconnect,
  fetchRecentMessages,
  fetchSession,
  loginSession,
  logoutSession,
  sendMsg,
} from './api';
import Header from './components/Header/Header';
import ChatHistory from './components/ChatHistory/ChatHistory';
import ChatInput from './components/ChatInput/ChatInput';
import { DEFAULT_ROOM, ROOM_OPTIONS, roomFromPath, roomPath } from './rooms';

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
  const [activeRoom, setActiveRoom] = useState(() => roomFromPath());
  const [session, setSession] = useState({ status: 'loading', mode: null, authenticated: false });
  const [accessToken, setAccessToken] = useState('');
  const [authError, setAuthError] = useState('');
  const [authBusy, setAuthBusy] = useState(false);

  const sessionRequired = session.mode === 'session';
  const user = session.authenticated ? session.user : null;
  const availableRooms = sessionRequired
    ? (user?.rooms || [])
    : (ROOM_OPTIONS.includes(activeRoom) ? ROOM_OPTIONS : [activeRoom, ...ROOM_OPTIONS]);
  const currentRoom = availableRooms.includes(activeRoom)
    ? activeRoom
    : availableRooms[0] || DEFAULT_ROOM;
  const canChat = session.status === 'ready' && (!sessionRequired || Boolean(user));
  const currentUsername = user?.display_name || username;
  const currentUserID = user?.user_id || '';
  const hasUsername = Boolean(user) || username !== '';

  useEffect(() => {
    let cancelled = false;
    fetchSession()
      .then((nextSession) => {
        if (!cancelled) {
          setSession({ ...nextSession, status: 'ready' });
          if (nextSession.mode === 'session' && !nextSession.authenticated) {
            setConnectionStatus('offline');
          }
        }
      })
      .catch(() => {
        if (!cancelled) {
          setSession({ status: 'error', mode: null, authenticated: false });
          setConnectionStatus('offline');
          setAuthError('Session status is unavailable.');
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const nextPath = roomPath(currentRoom);
    if (!canChat || window.location.pathname === nextPath) {
      return;
    }
    window.history.replaceState({}, '', nextPath);
  }, [canChat, currentRoom]);

  useEffect(() => {
    if (!sessionRequired || !user?.expires_at) {
      return undefined;
    }

    const expiresIn = Date.parse(user.expires_at) - Date.now();
    const expireSession = () => {
      disconnect();
      setChatHistory([]);
      setSession({ status: 'ready', mode: 'session', authenticated: false });
      setConnectionStatus('offline');
      setAuthError('Your session expired. Sign in again.');
    };
    const timer = window.setTimeout(expireSession, Math.max(0, expiresIn));

    return () => window.clearTimeout(timer);
  }, [sessionRequired, user?.expires_at]);

  useEffect(() => {
    if (!canChat) {
      disconnect();
      return undefined;
    }
    connect((msg) => {
      try {
        const parsedMessage = JSON.parse(msg.data);
        setChatHistory((prevChatHistory) => appendUniqueMessages(prevChatHistory, [parsedMessage]));
      } catch (e) {
        console.error('Error parsing message:', e);
      }
    }, setConnectionStatus, currentRoom);
    return () => disconnect();
  }, [canChat, currentRoom, session.status]);

  useEffect(() => {
    if (connectionStatus !== 'connected') {
      return undefined;
    }

    let cancelled = false;
    fetchRecentMessages(50, currentRoom)
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
  }, [connectionStatus, currentRoom]);

  useEffect(() => {
    if (!sessionRequired || !user || !['disconnected', 'error'].includes(connectionStatus)) {
      return undefined;
    }

    let cancelled = false;
    fetchSession()
      .then((nextSession) => {
        if (cancelled || nextSession.authenticated) {
          return;
        }
        disconnect();
        setChatHistory([]);
        setSession({ ...nextSession, status: 'ready' });
        setConnectionStatus('offline');
        setAuthError(
          nextSession.reason === 'session_expired'
            ? 'Your session expired. Sign in again.'
            : 'Your session is no longer valid. Sign in again.'
        );
      })
      .catch(() => {
        // Keep the bounded WebSocket reconnect loop active during transient HTTP failures.
      });

    return () => {
      cancelled = true;
    };
  }, [connectionStatus, sessionRequired, user]);

  useEffect(() => {
    const handlePopState = () => {
      const nextRoom = roomFromPath();
      if (nextRoom === activeRoom) {
        return;
      }
      setChatHistory([]);
      setActiveRoom(nextRoom);
    };

    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, [activeRoom]);

  useEffect(() => {
    if (connectionStatus === 'connected' && hasUsername && !sessionRequired) {
      sendMsg(JSON.stringify({
        type: 'username',
        content: currentUsername
      }));
    }
  }, [connectionStatus, currentUsername, hasUsername, sessionRequired]);

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

  const changeRoom = (room) => {
    if (!availableRooms.includes(room) || room === currentRoom) {
      return;
    }

    window.history.pushState({}, '', roomPath(room));
    setChatHistory([]);
    setActiveRoom(room);
  };

  const signIn = async (event) => {
    event.preventDefault();
    const token = accessToken.trim();
    if (!token || authBusy) {
      return;
    }
    setAuthBusy(true);
    setAuthError('');
    try {
      const nextSession = await loginSession(token);
      setAccessToken('');
      setSession({ ...nextSession, status: 'ready' });
    } catch (error) {
      setAuthError(error instanceof Error ? error.message : 'Sign in failed.');
    } finally {
      setAuthBusy(false);
    }
  };

  const signOut = async () => {
    if (authBusy) {
      return;
    }
    setAuthBusy(true);
    setAuthError('');
    try {
      await logoutSession();
      disconnect();
      setChatHistory([]);
      setSession({ status: 'ready', mode: 'session', authenticated: false });
      setConnectionStatus('offline');
    } catch (error) {
      setAuthError(error instanceof Error ? error.message : 'Sign out failed.');
    } finally {
      setAuthBusy(false);
    }
  };

  return (
    <div className="App">
      <a href="#main-content" className="skip-link">
        Skip to chat
      </a>
      <Header
        connectionStatus={connectionStatus}
        onRoomChange={changeRoom}
        room={currentRoom}
        roomOptions={availableRooms}
        identity={user?.display_name}
        onLogout={user ? signOut : undefined}
        logoutDisabled={authBusy}
      />

      <main id="main-content" className="chat-main" tabIndex={-1}>
        <div className="chat-container">
          {session.status === 'loading' && (
            <div className="connection-status" role="status" aria-live="polite">
              Checking session…
            </div>
          )}
          {session.status === 'error' && (
            <div className="connection-status" role="alert">
              {authError}
            </div>
          )}
          {user && authError && (
            <p className="auth-error" role="alert">{authError}</p>
          )}
          {sessionRequired && !user && session.status === 'ready' && (
            <section className="auth-gate" aria-labelledby="auth-heading">
              <div>
                <p className="eyebrow">Private rooms</p>
                <h2 id="auth-heading">Sign in to Chatster</h2>
              </div>
              <form onSubmit={signIn}>
                <label htmlFor="chatster-access-token">Access token</label>
                <div className="auth-gate-fields">
                  <input
                    id="chatster-access-token"
                    type="password"
                    value={accessToken}
                    onChange={(event) => setAccessToken(event.target.value)}
                    autoComplete="off"
                    autoCapitalize="none"
                    maxLength={512}
                    spellCheck={false}
                    disabled={authBusy}
                    autoFocus
                  />
                  <button type="submit" disabled={!accessToken.trim() || authBusy}>
                    {authBusy ? 'Signing in…' : 'Sign in'}
                  </button>
                </div>
                {authError && <p className="auth-error" role="alert">{authError}</p>}
              </form>
            </section>
          )}
          {canChat && (
            <>
              {connectionStatus !== 'connected' && (
                <div className="connection-status" role="status" aria-live="polite">
                  {connectionStatus === 'connecting' && 'Connecting…'}
                  {connectionStatus === 'disconnected' && 'Reconnecting and catching up…'}
                  {connectionStatus === 'error' && 'Connection error — retrying…'}
                </div>
              )}

              <ChatHistory
                chatHistory={chatHistory}
                currentUserID={currentUserID}
                currentUsername={currentUsername}
              />
              <ChatInput
                sendMessage={send}
                hasUsername={hasUsername}
                username={currentUsername}
                connectionStatus={connectionStatus}
              />
            </>
          )}
        </div>
      </main>

      <footer className="footer">
        <p>Chatster © {new Date().getFullYear()} — Real-time chat application</p>
      </footer>
    </div>
  );
}

export default App;
