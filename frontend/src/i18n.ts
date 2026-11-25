import i18n from 'i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { initReactI18next } from 'react-i18next';
import { de } from './locales/de';
import { en } from './locales/en';

export const languages = ['en', 'de'] as const;
export type Language = (typeof languages)[number];

declare module 'i18next' {
  interface CustomTypeOptions {
    resources: { translation: typeof en };
  }
}

// Language: an explicit choice saved in localStorage wins, otherwise the
// browser language; anything unsupported falls back to English.
void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: { en: { translation: en }, de: { translation: de } },
    supportedLngs: languages,
    nonExplicitSupportedLngs: true,
    load: 'languageOnly',
    fallbackLng: 'en',
    interpolation: { escapeValue: false }, // React escapes already
    detection: {
      order: ['localStorage', 'navigator'],
      lookupLocalStorage: 'issuetrax.language',
      caches: ['localStorage'],
    },
  });

function syncDocumentLanguage(lng: string) {
  document.documentElement.lang = lng;
}

syncDocumentLanguage(i18n.resolvedLanguage ?? 'en');
i18n.on('languageChanged', syncDocumentLanguage);

export default i18n;
