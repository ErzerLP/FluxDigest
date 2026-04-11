import '@testing-library/jest-dom/vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

import { AppProviders } from '../providers/AppProviders';
import { AppRouter } from './index';

test('renders dashboard navigation item', async () => {
  render(
    <AppProviders>
      <MemoryRouter initialEntries={["/dashboard"]}>
        <AppRouter />
      </MemoryRouter>
    </AppProviders>,
  );

  expect(await screen.findByText('Dashboard')).toBeInTheDocument();
  expect(screen.getByText('FluxDigest')).toBeInTheDocument();
});
