import React, { forwardRef, useCallback, useEffect, useRef, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import './ChatHistory.scss';

// The custom message log is intentionally tabbable so keyboard users can scroll it.
/* eslint-disable jsx-a11y/no-noninteractive-tabindex */

const announcementsPreferenceKey = 'chatster.reduce-announcements';
const reducedMotionQuery = '(prefers-reduced-motion: reduce)';
const virtualizationThreshold = 1000;

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

const messageKey = (msg, index) => (
  msg.id != null ? `msg-${msg.id}` : `local-${index}-${msg.username}-${msg.content?.slice(0, 16)}`
);

const readReducedMotionPreference = () => (
  typeof window !== 'undefined' &&
  typeof window.matchMedia === 'function' &&
  window.matchMedia(reducedMotionQuery).matches
);

const MessageRow = forwardRef(({ message, currentUsername, index, style }, ref) => {
  const isNotification = message.type === 'notification';
  const isOwn = !isNotification && currentUsername && message.username === currentUsername;

  return (
    <div
      ref={ref}
      data-index={index}
      style={style}
      className={isNotification ? 'message-notification' : `message-container ${isOwn ? 'is-own' : ''}`}
    >
      {isNotification ? (
        <div className="notification">
          <span>{message.content}</span>
          {message.timestamp && <time dateTime={message.timestamp}>{formatTime(message.timestamp)}</time>}
        </div>
      ) : (
        <div className="message">
          <div className="message-header">
            <span className="username">{isOwn ? 'You' : message.username}</span>
            <time className="timestamp" dateTime={message.timestamp}>{formatTime(message.timestamp)}</time>
          </div>
          <div className="message-content">
            {message.content}
          </div>
        </div>
      )}
    </div>
  );
});

MessageRow.displayName = 'MessageRow';

const ChatHistory = ({ chatHistory, currentUsername }) => {
  const messagesRef = useRef(null);
  const messagesEndRef = useRef(null);
  const shouldStickToBottomRef = useRef(true);
  const [reduceAnnouncements, setReduceAnnouncements] = useState(readAnnouncementsPreference);
  const [prefersReducedMotion, setPrefersReducedMotion] = useState(readReducedMotionPreference);
  const isVirtualized = chatHistory.length >= virtualizationThreshold;
  // TanStack Virtual exposes an imperative instance; keep its methods live for scroll updates.
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: chatHistory.length,
    getScrollElement: () => messagesRef.current,
    estimateSize: () => 88,
    getItemKey: useCallback(
      (index) => messageKey(chatHistory[index], index),
      [chatHistory]
    ),
    overscan: 8,
    enabled: isVirtualized,
    useFlushSync: false,
  });

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return undefined;
    }

    const mediaQuery = window.matchMedia(reducedMotionQuery);
    const updatePreference = () => setPrefersReducedMotion(mediaQuery.matches);
    updatePreference();
    mediaQuery.addEventListener?.('change', updatePreference);

    return () => mediaQuery.removeEventListener?.('change', updatePreference);
  }, []);

  const scrollToBottom = useCallback(() => {
    shouldStickToBottomRef.current = true;
    if (chatHistory.length === 0) {
      return;
    }

    const behavior = prefersReducedMotion || isVirtualized ? 'auto' : 'smooth';
    if (isVirtualized) {
      virtualizer.scrollToIndex(chatHistory.length - 1, { align: 'end', behavior });
      return;
    }

    const el = messagesEndRef.current;
    if (el && typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ behavior });
    }
  }, [chatHistory.length, isVirtualized, prefersReducedMotion, virtualizer]);

  const handleScroll = () => {
    const element = messagesRef.current;
    if (!element) {
      return;
    }

    const distanceFromBottom = element.scrollHeight - element.scrollTop - element.clientHeight;
    shouldStickToBottomRef.current = distanceFromBottom <= 48;
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
    if (chatHistory.length === 0) {
      shouldStickToBottomRef.current = true;
      return;
    }

    if (shouldStickToBottomRef.current) {
      scrollToBottom();
    }
  }, [chatHistory, scrollToBottom]);

  const renderMessage = (message, index, virtualItem) => {
    const style = virtualItem
      ? {
          left: 0,
          position: 'absolute',
          top: 0,
          transform: `translateY(${virtualItem.start}px)`,
          width: '100%',
        }
      : undefined;

    return (
      <MessageRow
        key={messageKey(message, index)}
        ref={virtualItem ? virtualizer.measureElement : undefined}
        index={index}
        message={message}
        currentUsername={currentUsername}
        style={style}
      />
    );
  };

  const renderedHistory = isVirtualized ? (
    <div
      className="messages-virtual-content"
      style={{ height: `${virtualizer.getTotalSize()}px` }}
    >
      {virtualizer.getVirtualItems().map((virtualItem) => (
        renderMessage(chatHistory[virtualItem.index], virtualItem.index, virtualItem)
      ))}
    </div>
  ) : (
    chatHistory.map((message, index) => renderMessage(message, index))
  );
  
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
        ref={messagesRef}
        className="messages"
        role="log"
        tabIndex={0}
        aria-live={reduceAnnouncements ? 'off' : 'polite'}
        aria-relevant="additions"
        aria-label="Chat messages"
        onScroll={handleScroll}
      >
        {chatHistory.length > 0 ? renderedHistory : (
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
