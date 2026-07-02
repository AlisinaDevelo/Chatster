import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';
import { connect, disconnect, fetchRecentMessages, sendMsg } from './api';

jest.mock('./api', () => ({
  connect: jest.fn(),
  disconnect: jest.fn(),
  fetchRecentMessages: jest.fn(),
  sendMsg: jest.fn(),
}));

describe('App', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    fetchRecentMessages.mockResolvedValue([]);
    connect.mockImplementation((_onMessage, setStatus) => {
      if (setStatus) setStatus('connected');
    });
  });

  test('connects and shows live status', async () => {
    render(<App />);
    expect(connect).toHaveBeenCalled();
    await waitFor(() => {
      expect(screen.getByText('Live')).toBeInTheDocument();
    });
    expect(screen.getByRole('heading', { name: /chatster/i })).toBeInTheDocument();
  });

  test('loads recent messages when connected', async () => {
    fetchRecentMessages.mockResolvedValue([
      {
        id: 7,
        type: 'message',
        username: 'bob',
        content: 'already here',
        timestamp: '2026-06-24T09:00:00Z',
      },
    ]);

    render(<App />);

    await waitFor(() => {
      expect(fetchRecentMessages).toHaveBeenCalledWith(50);
    });
    expect(await screen.findByText('already here')).toBeInTheDocument();
  });

  test('disconnects on unmount', () => {
    const { unmount } = render(<App />);
    unmount();
    expect(disconnect).toHaveBeenCalled();
  });

  test('sends username handshake after joining', async () => {
    const user = userEvent.setup();
    render(<App />);
    const input = await screen.findByPlaceholderText(/enter your username/i);
    await user.type(input, 'alice');
    await user.click(screen.getByRole('button', { name: /join chat/i }));
    await waitFor(() => {
      expect(sendMsg).toHaveBeenCalledWith(
        JSON.stringify({ type: 'username', content: 'alice' })
      );
    });
    expect(screen.getAllByText(/joined as/i).length).toBeGreaterThan(0);
    expect(screen.getByText('alice')).toBeInTheDocument();
  });
});
