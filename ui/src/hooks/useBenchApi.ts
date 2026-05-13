import { useCallback } from "react";

const API_BASE = import.meta.env.VITE_BENCH_API_URL || "";
const READ_METHODS = new Set(["GET", "HEAD"]);

export function useBenchApi() {
  const request = useCallback(
    async <T = unknown>(path: string, options: RequestInit = {}): Promise<T> => {
      const method = (options.method || "GET").toUpperCase();
      if (!READ_METHODS.has(method)) {
        throw new Error(
          "The public Bench UI is read-only. Use bench-cli or a server-side API client for authenticated actions.",
        );
      }
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        ...(options.headers as Record<string, string>),
      };
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
