import { useCallback } from "react";

export function useApi() {
  const request = useCallback(async <T>(path: string, init?: RequestInit): Promise<T> => {
    const res = await fetch(path, {
      headers: { "Content-Type": "application/json", ...init?.headers },
      ...init,
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(body.error || res.statusText);
    }
    return res.json();
  }, []);

  return { request };
}
