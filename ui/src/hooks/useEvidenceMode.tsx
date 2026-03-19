import { createContext, useContext, useState, type ReactNode } from "react";

export type EvidenceMode = "proxy" | "smart";

interface EvidenceModeCtx {
  mode: EvidenceMode;
  setMode: (m: EvidenceMode) => void;
}

const EvidenceModeContext = createContext<EvidenceModeCtx>({
  mode: "proxy",
  setMode: () => {},
});

export function EvidenceModeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<EvidenceMode>(() => {
    const saved = localStorage.getItem("evidra-bench-evidence-mode");
    return saved === "smart" ? "smart" : "proxy";
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
