import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi } from 'vitest';
import Header from './Header';

describe('Header', () => {
  test('shows live when connected', () => {
    render(<Header connectionStatus="connected" />);
    expect(screen.getByText('Live')).toBeInTheDocument();
  });

  test('shows connecting state', () => {
    render(<Header connectionStatus="connecting" />);
    expect(screen.getByText('Connecting')).toBeInTheDocument();
  });

  test('allows the active room to be changed', async () => {
    const user = userEvent.setup();
    const onRoomChange = vi.fn();

    render(
      <Header
        connectionStatus="connected"
        room="general"
        roomOptions={['general', 'engineering']}
        onRoomChange={onRoomChange}
      />
    );

    const roomPicker = screen.getByRole('combobox', { name: /chat room/i });
    expect(roomPicker).toHaveValue('general');

    await user.selectOptions(roomPicker, 'engineering');

    expect(onRoomChange).toHaveBeenCalledWith('engineering');
  });

  test('shows authenticated identity and exposes logout', async () => {
    const user = userEvent.setup();
    const onLogout = vi.fn();

    render(
      <Header
        connectionStatus="connected"
        identity="Alice"
        onLogout={onLogout}
      />
    );

    expect(screen.getByText('Alice')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /sign out/i }));
    expect(onLogout).toHaveBeenCalledOnce();
  });
});
