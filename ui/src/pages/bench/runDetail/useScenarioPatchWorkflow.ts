import { useState } from "react";
import type { ScenarioPatchPreview, ScenarioPatchValidation } from "../../../lib/scenarioPatchPreview.mts";

export function useScenarioPatchWorkflow() {
  const [scenarioPatchPreview, setScenarioPatchPreview] = useState<ScenarioPatchPreview | null>(null);
  const [scenarioPatchPreviewLoading, setScenarioPatchPreviewLoading] = useState(false);
  const [scenarioPatchPreviewError, setScenarioPatchPreviewError] = useState<string | null>(null);
  const [scenarioPatchValidation, setScenarioPatchValidation] = useState<ScenarioPatchValidation | null>(null);
  const [scenarioPatchValidationLoading, setScenarioPatchValidationLoading] = useState(false);
  const [scenarioPatchValidationError, setScenarioPatchValidationError] = useState<string | null>(null);
  const [scenarioPatchValidationLoaded, setScenarioPatchValidationLoaded] = useState(false);

  return {
    scenarioPatchPreview,
    setScenarioPatchPreview,
    scenarioPatchPreviewLoading,
    setScenarioPatchPreviewLoading,
    scenarioPatchPreviewError,
    setScenarioPatchPreviewError,
    scenarioPatchValidation,
    setScenarioPatchValidation,
    scenarioPatchValidationLoading,
    setScenarioPatchValidationLoading,
    scenarioPatchValidationError,
    setScenarioPatchValidationError,
    scenarioPatchValidationLoaded,
    setScenarioPatchValidationLoaded,
  };
}
