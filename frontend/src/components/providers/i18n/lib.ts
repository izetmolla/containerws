import i18n from "i18next"
import { initReactI18next } from "react-i18next"
import HttpBackend from "i18next-http-backend"
import LanguageDetector from "i18next-browser-languagedetector"

i18n
  .use(HttpBackend) // load translations from external source
  .use(LanguageDetector) // detect user language
  .use(initReactI18next) // bind react-i18next to i18next
  .init({
    compatibilityJSON: "v4",
    fallbackLng: false,
    supportedLngs: ["sq_AL", "en_US"], // list your supported languages
    load: "currentOnly", // Only load the current language, no fallbacks
    debug: false, // log only in dev mode
    // debug: import.meta.env.DEV , // log only in dev mode
    defaultNS: "translation",
    ns: ["translation"],
    partialBundledLanguages: true,
    // HTTP backend configuration (for loading translations)
    backend: {
      loadPath: "/api/languages/{{lng}}/{{ns}}.json",
      // Add cache control headers
      requestOptions: {
        cache: "default",
      },
    },

    interpolation: {
      escapeValue: false, // react already escapes
    },

    // Language detection configuration
    detection: {
      order: ["localStorage", "htmlTag", "navigator"],
      caches: ["localStorage"],
      // Convert detected language codes to supported format
      // convertDetectedLanguage: (lng: string) => {
      //     // Convert hyphen-separated codes to underscore-separated codes
      //     const converted = lng.replace('-', '_');

      //     // Map common language codes to your supported ones
      //     const languageMap: Record<string, string> = {
      //         'en': 'en_US',
      //         'de': 'de_DE',
      //         'fr': 'fr_FR',
      //         'it': 'it_IT',
      //         'sq': 'sq_AL'
      //     };

      //     // Return mapped language or converted language
      //     return languageMap[converted] || converted;
      // }
    },
  })

export default i18n
