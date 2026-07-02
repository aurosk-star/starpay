import { useEffect, useState } from "react";

import { getPublicSiteConfig } from "./api";

const FALLBACK_SITE_NAME = "starpay-支付网关";

let cachedSiteName = FALLBACK_SITE_NAME;
const listeners = new Set<(siteName: string) => void>();

export function useSiteName() {
  const [siteName, setSiteName] = useState(cachedSiteName);

  useEffect(() => {
    listeners.add(setSiteName);
    if (cachedSiteName !== FALLBACK_SITE_NAME) {
      return () => {
        listeners.delete(setSiteName);
      };
    }
    void getPublicSiteConfig()
      .then((result) => setCachedSiteName(result.site_config.site_name))
      .catch(() => setCachedSiteName(FALLBACK_SITE_NAME));
    return () => {
      listeners.delete(setSiteName);
    };
  }, []);

  return siteName;
}

export function setCachedSiteName(siteName: string) {
  const normalized = siteName.trim() || FALLBACK_SITE_NAME;
  cachedSiteName = normalized;
  for (const listener of listeners) {
    listener(normalized);
  }
}

export function formatDocumentTitle(pageTitle: string | undefined, siteName: string) {
  const normalizedPageTitle = pageTitle?.trim();
  const normalizedSiteName = siteName.trim() || FALLBACK_SITE_NAME;
  return normalizedPageTitle
    ? `${normalizedPageTitle} - ${normalizedSiteName}`
    : normalizedSiteName;
}
