import { cleanup, render, screen } from '@testing-library/react';
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
});
