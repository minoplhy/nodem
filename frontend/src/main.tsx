import React from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import './style.css';
import { initParticles } from './components/particles';

import { BrowserRouter } from 'react-router-dom';

initParticles();

const rawBase = (window as any).__BASE_PATH__ || import.meta.env.BASE_URL || '/';
const basename = rawBase.replace(/\/$/, '');

const container = document.getElementById('app');
if (container) {
  const root = createRoot(container);
  root.render(
    <React.StrictMode>
      <BrowserRouter basename={basename}>
        <App />
      </BrowserRouter>
    </React.StrictMode>
  );
}
