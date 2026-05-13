import { useEffect, useState } from "react";

import { useBenchApi } from "./useBenchApi";
import { fallbackModels, selectAvailableModels } from "../lib/modelData.mts";
import type { EnabledModel, ModelsResponse } from "../types/models";

export function useModels() {
  const { request } = useBenchApi();
  const [models, setModels] = useState<EnabledModel[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    request<ModelsResponse>("/v1/bench/models")
      .then((res) => {
        if (!cancelled) {
          setModels(selectAvailableModels(res.models));
        }
      })
      .catch(() => {
        if (!cancelled) {
          setModels(fallbackModels());
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [request]);

  return { models, loading };
}
