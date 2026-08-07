import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach } from 'vitest';
import ChatHistory from './ChatHistory';

beforeEach(() => {
  window.localStorage.clear();
});

afterEach(() => {
  cleanup();
  window.localStorage.clear();
});

describe('ChatHistory', () => {
  const createMessages = (count) => Array.from({ length: count }, (_, index) => ({
    id: index + 1,
    type: 'message',
    username: index % 2 === 0 ? 'alice' : 'bob',
    content: `message ${index + 1}`,
    timestamp: '2026-06-24T09:00:00Z',
  }));

  test('labels current user messages as yours', () => {
    render(
      <ChatHistory
        currentUsername="alice"
        chatHistory={[
          {
            id: 1,
            type: 'message',
            username: 'alice',
            content: 'hello',
            timestamp: '2026-06-24T09:00:00Z',
          },
        ]}
      />
    );

    expect(screen.getByText('You')).toBeInTheDocument();
    expect(screen.getByText('hello')).toBeInTheDocument();
  });

  test('keeps the scrollable message history keyboard reachable', async () => {
    const user = userEvent.setup();

    render(<ChatHistory currentUsername="alice" chatHistory={[]} />);

    const log = screen.getByRole('log', { name: 'Chat messages' });
    expect(log).toHaveAttribute('tabindex', '0');

    await user.tab();
    await user.tab();

    expect(log).toHaveFocus();
  });

  test('lets users quiet live announcements and persists the preference', async () => {
    const user = userEvent.setup();

    render(<ChatHistory currentUsername="alice" chatHistory={[]} />);

    const log = screen.getByRole('log', { name: 'Chat messages' });
    const toggle = screen.getByRole('checkbox', { name: /quiet updates/i });
    expect(log).toHaveAttribute('aria-live', 'polite');

    await user.click(toggle);

    expect(log).toHaveAttribute('aria-live', 'off');
    expect(window.localStorage.getItem('chatster.reduce-announcements')).toBe('true');
  });

  test('virtualizes long histories while keeping rendered content accessible', async () => {
    const messages = createMessages(1200);
    const originalGetBoundingClientRect = HTMLElement.prototype.getBoundingClientRect;
    const originalOffsetHeight = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetHeight');
    const originalOffsetWidth = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetWidth');
    const originalScrollTo = HTMLElement.prototype.scrollTo;
    HTMLElement.prototype.getBoundingClientRect = function getBoundingClientRect() {
      if (this.classList.contains('messages')) {
        return { bottom: 374, height: 374, left: 0, right: 600, top: 0, width: 600 };
      }
      if (this.dataset.index) {
        return { bottom: 82, height: 82, left: 0, right: 600, top: 0, width: 600 };
      }
      return originalGetBoundingClientRect.call(this);
    };
    Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
      configurable: true,
      get() {
        if (this.classList.contains('messages')) return 374;
        if (this.dataset.index) return 82;
        return 0;
      },
    });
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', {
      configurable: true,
      get() {
        return this.classList.contains('messages') || this.dataset.index ? 600 : 0;
      },
    });
    HTMLElement.prototype.scrollTo = function scrollTo({ top = 0 } = {}) {
      this.scrollTop = top;
      this.dispatchEvent(new Event('scroll'));
    };

    try {
      render(<ChatHistory currentUsername="alice" chatHistory={messages} />);

      const log = screen.getByRole('log', { name: 'Chat messages' });
      await waitFor(() => {
        expect(log.querySelectorAll('.message-container').length).toBeLessThan(messages.length);
      });

      expect(log).toHaveAttribute('role', 'log');
      expect(log.querySelectorAll('.message-container').length).toBeGreaterThan(0);
      expect(screen.getByText('message 1')).toBeInTheDocument();
    } finally {
      HTMLElement.prototype.getBoundingClientRect = originalGetBoundingClientRect;
      Object.defineProperty(HTMLElement.prototype, 'offsetHeight', originalOffsetHeight);
      Object.defineProperty(HTMLElement.prototype, 'offsetWidth', originalOffsetWidth);
      HTMLElement.prototype.scrollTo = originalScrollTo;
    }
  });
});
