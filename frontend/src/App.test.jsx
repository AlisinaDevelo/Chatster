import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi } from 'vitest';
import axe from 'axe-core';
import App from './App';
import { connect, disconnect, fetchRecentMessages, sendMsg } from './api';

vi.mock('./api', () => ({
  connect: vi.fn(),
  disconnect: vi.fn(),
  fetchRecentMessages: vi.fn(),
  sendMsg: vi.fn(),
}));

describe('App', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.history.replaceState({}, '', '/');
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
      expect(fetchRecentMessages).toHaveBeenCalledWith(50, 'general');
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

  test('switches rooms and reconnects with room-scoped history', async () => {
    const user = userEvent.setup();
    render(<App />);

    await waitFor(() => {
      expect(screen.getByRole('combobox', { name: /chat room/i })).toHaveValue('general');
    });

    await user.selectOptions(
      screen.getByRole('combobox', { name: /chat room/i }),
      'engineering'
    );

    await waitFor(() => {
      expect(connect).toHaveBeenLastCalledWith(expect.any(Function), expect.any(Function), 'engineering');
      expect(fetchRecentMessages).toHaveBeenLastCalledWith(50, 'engineering');
    });
    expect(window.location.pathname).toBe('/rooms/engineering');
  });

  test('has no automated accessibility violations', async () => {
    render(<App />);

    const canvasContext = {
      canvas: document.createElement('canvas'),
      clearRect: vi.fn(),
      fillText: vi.fn(),
      font: '',
      getImageData: (_x, _y, width, height) => ({
        data: new Uint8ClampedArray(Math.ceil(width) * Math.ceil(height) * 4),
      }),
      measureText: (text) => ({ width: text.length * 8 }),
      textAlign: 'left',
      textBaseline: 'top',
    };
    const getContext = vi
      .spyOn(HTMLCanvasElement.prototype, 'getContext')
      .mockReturnValue(canvasContext);
    const originalGetComputedStyle = window.getComputedStyle;
    const getComputedStyle = vi
      .spyOn(window, 'getComputedStyle')
      .mockImplementation((element) => originalGetComputedStyle.call(window, element));

    let results;
    try {
      results = await axe.run(document.body);
    } finally {
      getContext.mockRestore();
      getComputedStyle.mockRestore();
    }

    expect(results.violations).toEqual([]);
  });
});
