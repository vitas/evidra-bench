import { createContext, useContext, useState, type ReactNode } from "react";

export type EvidenceMode = "all" | "none" | "evidra";

interface EvidenceModeCtx {
  mode: EvidenceMode;
  setMode: (m: EvidenceMode) => void;
}

const VALID_MODES: EvidenceMode[] = ["all", "none", "evidra"];
const LEGACY_EVIDRA_MODES = new Set(["proxy", "smart", "direct", "mcp"]);

function normalizeEvidenceMode(mode: string | null): EvidenceMode {
  if (mode && VALID_MODES.includes(mode as EvidenceMode)) {
    return mode as EvidenceMode;
  }
  if (mode && LEGACY_EVIDRA_MODES.has(mode)) {
    return "evidra";
  }
  return "all";
}

const EvidenceModeContext = createContext<EvidenceModeCtx>({
  mode: "all",
  setMode: () => {},
});

export function EvidenceModeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<EvidenceMode>(() => {
    const saved = localStorage.getItem("evidra-bench-evidence-mode");
    return normalizeEvidenceMode(saved);
  });

  const setMode = (m: EvidenceMode) => {
    setModeState(m);
    localStorage.setItem("evidra-bench-evidence-mode", m);
  };

  return (
    <EvidenceModeContext.Provider value={{ mode, setMode }}>
      {children}
    </EvidenceModeContext.Provider>
  );
}

export function useEvidenceMode() {
  return useContext(EvidenceModeContext);
}

export function formatEvidenceModeLabel(mode?: string): string {
  switch (mode) {
    case "none":
      return "Baseline";
    case "proxy":
      return "Evidra Proxy";
    case "smart":
      return "Evidra Smart";
    case "mcp":
      return "Evidra Full";
    case "direct":
      return "Evidra Full";
    case "evidra":
      return "Evidra";
    default:
      return mode || "Unknown";
  }
}
