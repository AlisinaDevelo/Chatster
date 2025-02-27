import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ChatInput from './ChatInput';

describe('ChatInput', () => {
  test('submits trimmed message when connected', async () => {
    const sendMessage = jest.fn();
    render(
      <ChatInput sendMessage={sendMessage} hasUsername connectionStatus="connected" />
    );
    await userEvent.type(screen.getByPlaceholderText(/type your message/i), '  hi  ');
    await userEvent.click(screen.getByRole('button', { name: /send/i }));
    expect(sendMessage).toHaveBeenCalledWith('hi');
  });

  test('disables input when disconnected', () => {
    render(
      <ChatInput sendMessage={jest.fn()} hasUsername connectionStatus="disconnected" />
    );
    expect(screen.getByPlaceholderText(/type your message/i)).toBeDisabled();
  });
});
