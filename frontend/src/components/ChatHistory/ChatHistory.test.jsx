import { render, screen } from '@testing-library/react';
import ChatHistory from './ChatHistory';

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
});
