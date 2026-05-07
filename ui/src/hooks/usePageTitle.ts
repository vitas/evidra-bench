import { useEffect } from "react";

const BASE = "Bench Lab";

export function usePageTitle(page?: string) {
  useEffect(() => {
    document.title = page ? `${page} — ${BASE}` : BASE;
    return () => {
      document.title = BASE;
    };
  }, [page]);
}
