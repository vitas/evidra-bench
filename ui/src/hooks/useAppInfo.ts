import { useState, useEffect } from "react";

interface AppInfo {
  readonly: boolean;
  version: string;
  loading: boolean;
}

export function useAppInfo(): AppInfo {
  const [info, setInfo] = useState<AppInfo>({ readonly: true, version: "", loading: true });

  useEffect(() => {
    fetch("/v1/bench/info")
      .then((res) => res.json())
      .then((data) =>
        setInfo({
          readonly: !!data.readonly,
          version: data.version || "",
          loading: false,
        }),
      )
      .catch(() => setInfo({ readonly: true, version: "", loading: false }));
  }, []);

  return info;
}
