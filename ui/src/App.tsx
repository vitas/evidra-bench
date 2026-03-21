import { BrowserRouter, Routes, Route } from "react-router";
import { EvidenceModeProvider } from "./hooks/useEvidenceMode";
import { Layout } from "./components/Layout";
import { Scenarios } from "./pages/Scenarios";
import { Designer } from "./pages/Designer";
import { Run } from "./pages/Run";

export function App() {
  return (
    <EvidenceModeProvider>
      <BrowserRouter>
        <Layout>
          <Routes>
            <Route path="/" element={<Scenarios />} />
            <Route path="/designer" element={<Designer />} />
            <Route path="/run" element={<Run />} />
          </Routes>
        </Layout>
      </BrowserRouter>
    </EvidenceModeProvider>
  );
}
