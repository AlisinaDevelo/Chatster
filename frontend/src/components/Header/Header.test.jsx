import { render, screen } from '@testing-library/react';
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
});
