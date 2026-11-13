import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { I18nextProvider } from 'react-i18next';

import { createI18n, detectLanguage } from '../shared/i18n';
import { App } from './App';
import './index.css';

const language = detectLanguage(navigator.languages);
document.documentElement.lang = language;

const root = document.getElementById('root');
if (!root) {
  throw new Error('Root element #root not found in index.html');
}

createRoot(root).render(
  <StrictMode>
    <I18nextProvider i18n={createI18n(language)}>
      <App version={import.meta.env.VITE_APP_VERSION ?? 'dev'} />
    </I18nextProvider>
  </StrictMode>,
);
