import { useEffect } from "react";

const BASE = "Bench Lab";
const SITE_ORIGIN = "https://bench.evidra.cc";

interface PageTitleOptions {
  canonicalPath?: string;
}

export function usePageTitle(page?: string, options: PageTitleOptions = {}) {
  useEffect(() => {
    document.title = page ? `${page} — ${BASE}` : BASE;
    return () => {
      document.title = BASE;
    };
  }, [page]);

  useEffect(() => {
    if (!options.canonicalPath) {
      return;
    }
    const canonical = document.querySelector<HTMLLinkElement>('link[rel="canonical"]');
    if (!canonical) {
      return;
    }
    const previousHref = canonical.getAttribute("href");
    canonical.setAttribute("href", new URL(options.canonicalPath, SITE_ORIGIN).toString());
    return () => {
      if (previousHref) {
        canonical.setAttribute("href", previousHref);
      }
    };
  }, [options.canonicalPath]);
}
