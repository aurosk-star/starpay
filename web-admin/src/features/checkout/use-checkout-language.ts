import { useEffect } from "react";
import { useTranslation } from "react-i18next";

const SUPPORTED_CHECKOUT_LANGUAGES = new Set(["zh-CN", "en"]);

export function useCheckoutLanguage() {
  const { i18n } = useTranslation();

  useEffect(() => {
    if (typeof window === "undefined") return;
    const params = new URLSearchParams(window.location.search);
    const language = normalizeCheckoutLanguage(
      params.get("lang") ?? params.get("lng") ?? params.get("locale"),
    );
    if (language && i18n.language !== language) {
      void i18n.changeLanguage(language);
    }
  }, [i18n]);
}

function normalizeCheckoutLanguage(value: string | null) {
  const language = value?.trim();
  if (!language) return "";
  if (language.toLowerCase() === "zh" || language.toLowerCase() === "zh-cn") {
    return "zh-CN";
  }
  if (
    language.toLowerCase() === "en" ||
    language.toLowerCase().startsWith("en-")
  ) {
    return "en";
  }
  return SUPPORTED_CHECKOUT_LANGUAGES.has(language) ? language : "";
}
