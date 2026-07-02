import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ChatInput from './ChatInput';

describe('ChatInput', () => {
  test('submits trimmed message when connected', async () => {
    const user = userEvent.setup();
    const sendMessage = jest.fn();
    render(
      <ChatInput sendMessage={sendMessage} hasUsername username="alice" connectionStatus="connected" />
    );
    await user.type(screen.getByPlaceholderText(/type your message/i), '  hi  ');
    await user.click(screen.getByRole('button', { name: /send/i }));
    expect(sendMessage).toHaveBeenCalledWith('hi');
  });

  test('disables input when disconnected', () => {
    render(
      <ChatInput sendMessage={jest.fn()} hasUsername username="alice" connectionStatus="disconnected" />
    );
    expect(screen.getByPlaceholderText(/type your message/i)).toBeDisabled();
  });

  test('shows explicit username setup before joining', async () => {
    const user = userEvent.setup();
    const sendMessage = jest.fn();
    render(
      <ChatInput sendMessage={sendMessage} hasUsername={false} connectionStatus="connected" />
    );
    expect(screen.getByRole('heading', { name: /choose a display name/i })).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText(/enter your username/i), ' alice ');
    await user.click(screen.getByRole('button', { name: /join chat/i }));
    expect(sendMessage).toHaveBeenCalledWith('alice');
  });
});
