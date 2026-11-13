import { renderToStaticMarkup } from 'react-dom/server';
import { I18nextProvider } from 'react-i18next';
import { describe, expect, it } from 'vitest';

import { createI18n, type Language } from '../shared/i18n';
import { App } from './App';

function render(language: Language): string {
  return renderToStaticMarkup(
    <I18nextProvider i18n={createI18n(language)}>
      <App version="1.2.3" />
    </I18nextProvider>,
  );
}

describe('App', () => {
  it('shows the name and the version in Portuguese', () => {
    const html = render('pt');
    expect(html).toContain('<h1>Intársia</h1>');
    expect(html).toContain('Versão 1.2.3');
  });

  it('shows the name and the version in English', () => {
    const html = render('en');
    expect(html).toContain('<h1>Intársia</h1>');
    expect(html).toContain('Version 1.2.3');
  });
});
