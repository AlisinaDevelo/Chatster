import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';
import { connect, disconnect, sendMsg } from './api';

jest.mock('./api', () => ({
  connect: jest.fn(),
  disconnect: jest.fn(),
  sendMsg: jest.fn(),
}));

describe('App', () => {
  beforeEach(() => {
    jest.clearAllMocks();
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

  test('disconnects on unmount', () => {
    const { unmount } = render(<App />);
    unmount();
    expect(disconnect).toHaveBeenCalled();
  });

  test('sends username handshake on first message', async () => {
    render(<App />);
    await waitFor(() =>
      expect(screen.getByPlaceholderText(/enter your username/i)).toBeInTheDocument()
    );
    const input = screen.getByPlaceholderText(/enter your username/i);
    await userEvent.type(input, 'alice');
    await userEvent.click(screen.getByRole('button', { name: /set username/i }));
    expect(sendMsg).toHaveBeenCalledWith(
      JSON.stringify({ type: 'username', content: 'alice' })
    );
  });
});
