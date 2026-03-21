import { BrowserRouter, Routes, Route } from "react-router";
import { EvidenceModeProvider } from "./hooks/useEvidenceMode";
import { Layout } from "./components/Layout";
import { Landing } from "./pages/Landing";
import { Scenarios } from "./pages/Scenarios";
import { Designer } from "./pages/Designer";
import { Run } from "./pages/Run";

export function App() {
  return (
    <EvidenceModeProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<Landing />} />
          <Route path="/*" element={
            <Layout>
              <Routes>
                <Route path="/scenarios" element={<Scenarios />} />
                <Route path="/designer" element={<Designer />} />
                <Route path="/run" element={<Run />} />
              </Routes>
            </Layout>
          } />
        </Routes>
      </BrowserRouter>
    </EvidenceModeProvider>
  );
}
