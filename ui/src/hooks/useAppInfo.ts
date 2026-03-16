import { useState, useEffect } from "react";

interface AppInfo {
  readonly: boolean;
  loading: boolean;
}

export function useAppInfo(): AppInfo {
  const [info, setInfo] = useState<AppInfo>({ readonly: true, loading: true });

  useEffect(() => {
    fetch("/v1/bench/info")
      .then((res) => res.json())
      .then((data) => setInfo({ readonly: !!data.readonly, loading: false }))
      .catch(() => setInfo({ readonly: true, loading: false }));
  }, []);

  return info;
}
