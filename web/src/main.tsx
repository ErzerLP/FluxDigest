import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import 'antd/dist/reset.css';

import { AppProviders } from './app/providers/AppProviders';
import { AppRouter } from './app/router';
import './styles/index.css';

const rootElement = document.getElementById('root');

if (!rootElement) {
  throw new Error('Missing root element for FluxDigest Console');
}

createRoot(rootElement).render(
  <StrictMode>
    <AppProviders>
      <BrowserRouter>
        <AppRouter />
      </BrowserRouter>
    </AppProviders>
  </StrictMode>,
);
