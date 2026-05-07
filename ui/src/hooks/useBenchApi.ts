import { useCallback } from "react";

const API_BASE = import.meta.env.VITE_BENCH_API_URL || "";
const API_KEY = import.meta.env.VITE_BENCH_API_KEY || "";

export function useBenchApi() {
  const request = useCallback(
    async <T = unknown>(path: string, options: RequestInit = {}): Promise<T> => {
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        ...(options.headers as Record<string, string>),
      };
      if (API_KEY) {
        headers["Authorization"] = `Bearer ${API_KEY}`;
      }
      const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
      if (!res.ok) {
        let message = res.statusText;
        try {
          const body = await res.json();
          message = body.error || body.message || message;
        } catch {}
        throw new Error(message);
      }
      return res.json() as Promise<T>;
    },
    [],
  );
  return { request };
}
