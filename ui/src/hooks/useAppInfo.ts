import { useState, useEffect } from "react";
import { requestBenchApi } from "../lib/benchApi.mts";

const API_BASE = import.meta.env.VITE_BENCH_API_URL || "";

interface AppInfo {
  readonly: boolean;
  version: string;
  loading: boolean;
}

export function useAppInfo(): AppInfo {
  const [info, setInfo] = useState<AppInfo>({ readonly: false, version: "", loading: true });

  useEffect(() => {
    requestBenchApi<Omit<AppInfo, "loading">>("/v1/bench/info", {}, { apiBase: API_BASE })
      .then((data) =>
        setInfo({
          readonly: !!data.readonly,
          version: data.version || "",
          loading: false,
        }),
      )
      .catch(() => setInfo({ readonly: false, version: "", loading: false }));
  }, []);

  return info;
}
