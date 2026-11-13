import { createInstance, type i18n } from 'i18next';
import { initReactI18next } from 'react-i18next';

import en from './locales/en.json';
import pt from './locales/pt.json';

export const resources = {
  en: { translation: en },
  pt: { translation: pt },
} as const;

export type Language = keyof typeof resources;

const fallbackLanguage: Language = 'en';

function isLanguage(code: string): code is Language {
  return code in resources;
}

/**
 * Picks the first supported language from a list of BCP 47 tags, such as
 * `navigator.languages`. Region subtags are ignored: "pt-BR" matches "pt".
 */
export function detectLanguage(preferred: readonly string[]): Language {
  for (const tag of preferred) {
    const code = tag.toLowerCase().split('-')[0] ?? '';
    if (isLanguage(code)) {
      return code;
    }
  }
  return fallbackLanguage;
}

/** Creates an initialized i18next instance. Resources are bundled, so init is synchronous. */
export function createI18n(language: Language): i18n {
  const instance = createInstance();
  void instance.use(initReactI18next).init({
    lng: language,
    fallbackLng: fallbackLanguage,
    resources,
    interpolation: { escapeValue: false }, // React already escapes.
  });
  return instance;
}
