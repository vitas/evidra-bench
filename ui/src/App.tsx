import { BrowserRouter, Routes, Route } from "react-router";
import { EvidenceModeProvider } from "./hooks/useEvidenceMode";
import { Layout } from "./components/Layout";
import { Landing } from "./pages/Landing";
import { Scenarios } from "./pages/Scenarios";
import { Designer } from "./pages/Designer";
import { Run } from "./pages/Run";
import { Results } from "./pages/Results";

// Bench dashboard pages (recovered from git history)
import { Layout as BenchLayout } from "./pages/bench/Layout";
import { Leaderboard } from "./pages/bench/Leaderboard";
import { Dashboard } from "./pages/bench/Dashboard";
import { Runs } from "./pages/bench/Runs";
import { RunDetail } from "./pages/bench/RunDetail";
import { Scenarios as BenchScenarios } from "./pages/bench/Scenarios";
import { ScenarioDetail } from "./pages/bench/ScenarioDetail";
import { Compare } from "./pages/bench/Compare";
import { SkillImpact } from "./pages/bench/SkillImpact";
import { Benchmarks } from "./pages/bench/Benchmarks";

export function App() {
  return (
    <EvidenceModeProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<Landing />} />

          {/* Lab routes (scenario catalog, designer, run configurator) */}
          <Route path="/*" element={
            <Layout>
              <Routes>
                <Route path="/scenarios" element={<Scenarios />} />
                <Route path="/designer" element={<Designer />} />
                <Route path="/run" element={<Run />} />
                <Route path="/results" element={<Results />} />
              </Routes>
            </Layout>
          } />

          {/* Bench routes (rich dashboard with leaderboard, runs, compare) */}
          <Route path="/bench/*" element={
            <BenchLayout>
              <Routes>
                <Route path="/" element={<Leaderboard />} />
                <Route path="/dashboard" element={<Dashboard />} />
                <Route path="/runs" element={<Runs />} />
                <Route path="/runs/:id" element={<RunDetail />} />
                <Route path="/scenarios" element={<BenchScenarios />} />
                <Route path="/scenarios/:id" element={<ScenarioDetail />} />
                <Route path="/compare" element={<Compare />} />
                <Route path="/skill-impact" element={<SkillImpact />} />
                <Route path="/benchmarks" element={<Benchmarks />} />
              </Routes>
            </BenchLayout>
          } />
        </Routes>
      </BrowserRouter>
    </EvidenceModeProvider>
  );
}
