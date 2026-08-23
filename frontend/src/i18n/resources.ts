import { en } from "./en";
import { de } from "./de";
import { fr } from "./fr";
import { ru } from "./ru";

export const resources = {
  en: { translation: en },
  de: { translation: de },
  fr: { translation: fr },
  ru: { translation: ru },
};

export type AppLanguage = keyof typeof resources;
