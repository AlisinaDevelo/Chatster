import React, { useEffect, useRef, useState } from 'react';
import './ChatHistory.scss';

// The custom message log is intentionally tabbable so keyboard users can scroll it.
/* eslint-disable jsx-a11y/no-noninteractive-tabindex */

const announcementsPreferenceKey = 'chatster.reduce-announcements';

const readAnnouncementsPreference = () => {
  if (typeof window === 'undefined') {
    return false;
  }

  try {
    return window.localStorage.getItem(announcementsPreferenceKey) === 'true';
  } catch {
    return false;
  }
};

const formatTime = (timestamp) => {
  // If there's a timestamp (from DB), use it, otherwise use current time
  const date = timestamp ? new Date(timestamp) : new Date();
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
};

const ChatHistory = ({ chatHistory, currentUsername }) => {
  const messagesEndRef = useRef(null);
  const [reduceAnnouncements, setReduceAnnouncements] = useState(readAnnouncementsPreference);
  
  const scrollToBottom = () => {
    const el = messagesEndRef.current;
    if (el && typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ behavior: 'smooth' });
    }
  };

  const updateAnnouncementsPreference = (event) => {
    const shouldReduceAnnouncements = event.target.checked;
    setReduceAnnouncements(shouldReduceAnnouncements);

    try {
      window.localStorage.setItem(
        announcementsPreferenceKey,
        String(shouldReduceAnnouncements)
      );
    } catch {
      // A blocked storage API should not prevent chat from working.
    }
  };
  
  useEffect(() => {
    scrollToBottom();
  }, [chatHistory]);
  
  const renderMessages = () => {
    return chatHistory.map((msg, index) => {
      const isNotification = msg.type === 'notification';
      const isOwn = !isNotification && currentUsername && msg.username === currentUsername;
      const key =
        msg.id != null ? `msg-${msg.id}` : `local-${index}-${msg.username}-${msg.content?.slice(0, 16)}`;

      return (
        <div
          key={key}
          className={isNotification ? 'message-notification' : `message-container ${isOwn ? 'is-own' : ''}`}
        >
          {isNotification ? (
            <div className="notification">
              <span>{msg.content}</span>
              {msg.timestamp && <time>{formatTime(msg.timestamp)}</time>}
            </div>
          ) : (
            <div className="message">
              <div className="message-header">
                <span className="username">{isOwn ? 'You' : msg.username}</span>
                <time className="timestamp">{formatTime(msg.timestamp)}</time>
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
    <section className="chat-history" aria-labelledby="chat-heading">
      <div className="chat-header">
        <h2 id="chat-heading">Chat History</h2>
        <div className="chat-header-controls">
          <label className="announcement-toggle" title="Stop announcing newly added messages">
            <input
              type="checkbox"
              checked={reduceAnnouncements}
              onChange={updateAnnouncementsPreference}
            />
            <span>Quiet updates</span>
          </label>
          <span className="message-count">
            {chatHistory.length} messages
          </span>
        </div>
      </div>

      <div
        className="messages"
        role="log"
        tabIndex={0}
        aria-live={reduceAnnouncements ? 'off' : 'polite'}
        aria-relevant="additions"
        aria-label="Chat messages"
      >
        {chatHistory.length > 0 ? renderMessages() : (
          <div className="no-messages">
            No messages yet. Start the conversation!
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>
    </section>
  );
};

export default ChatHistory;
