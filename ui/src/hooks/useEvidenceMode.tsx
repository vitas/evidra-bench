import { createContext, useContext, useState, type ReactNode } from "react";

export type EvidenceMode = "all" | "none" | "mcp";

interface EvidenceModeCtx {
  mode: EvidenceMode;
  setMode: (m: EvidenceMode) => void;
}

const VALID_MODES: EvidenceMode[] = ["all", "none", "mcp"];

function normalizeEvidenceMode(mode: string | null): EvidenceMode {
  if (mode && VALID_MODES.includes(mode as EvidenceMode)) {
    return mode as EvidenceMode;
  }
  return "all";
}

const EvidenceModeContext = createContext<EvidenceModeCtx>({
  mode: "all",
  setMode: () => {},
});

export function EvidenceModeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<EvidenceMode>(() => {
    const saved = localStorage.getItem("bench-evidence-mode");
    return normalizeEvidenceMode(saved);
  });

  const setMode = (m: EvidenceMode) => {
    setModeState(m);
    localStorage.setItem("bench-evidence-mode", m);
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
    case "mcp":
      return "MCP";
    default:
      return mode || "Unknown";
  }
}
