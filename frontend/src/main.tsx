import React from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import './style.css';
import { initParticles } from './components/particles';

import { BrowserRouter } from 'react-router-dom';
import { BASE_PATH } from './core';

initParticles();

const container = document.getElementById('app');
if (container) {
  const root = createRoot(container);
  root.render(
    <React.StrictMode>
      <BrowserRouter basename={BASE_PATH || '/'}>
        <App />
      </BrowserRouter>
    </React.StrictMode>
  );
}
