import { useCallback } from "react";
import { requestBenchApi } from "../lib/benchApi.mts";

const API_BASE = import.meta.env.VITE_BENCH_API_URL || "";

export interface UseBenchApiOptions {
  authToken?: string;
}

export function useBenchApi(options: UseBenchApiOptions = {}) {
  const authToken = options.authToken;

  const request = useCallback(
    async <T = unknown>(path: string, requestOptions: RequestInit = {}): Promise<T> => {
      return requestBenchApi<T>(path, requestOptions, { apiBase: API_BASE, authToken });
    },
    [authToken],
  );

  return { request };
}
