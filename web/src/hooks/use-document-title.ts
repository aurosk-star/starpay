import { useEffect } from "react";

import { formatDocumentTitle, useSiteName } from "@/features/config/site-config";

export function useDocumentTitle(pageTitle?: string) {
  const siteName = useSiteName();

  useEffect(() => {
    document.title = formatDocumentTitle(pageTitle, siteName);
  }, [pageTitle, siteName]);
}
