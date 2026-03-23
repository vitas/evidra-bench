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
import { Regressions } from "./pages/bench/Regressions";
import { Insights } from "./pages/bench/Insights";

export function App() {
  return (
    <EvidenceModeProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<Landing />} />

          {/* Bench routes (rich dashboard with leaderboard, runs, compare) */}
          <Route path="/bench" element={<BenchLayout><Leaderboard /></BenchLayout>} />
          <Route path="/bench/dashboard" element={<BenchLayout><Dashboard /></BenchLayout>} />
          <Route path="/bench/runs" element={<BenchLayout><Runs /></BenchLayout>} />
          <Route path="/bench/runs/:id" element={<BenchLayout><RunDetail /></BenchLayout>} />
          <Route path="/bench/scenarios" element={<BenchLayout><BenchScenarios /></BenchLayout>} />
          <Route path="/bench/scenarios/:id" element={<BenchLayout><ScenarioDetail /></BenchLayout>} />
          <Route path="/bench/compare" element={<BenchLayout><Compare /></BenchLayout>} />
          <Route path="/bench/skill-impact" element={<BenchLayout><SkillImpact /></BenchLayout>} />
          <Route path="/bench/regressions" element={<BenchLayout><Regressions /></BenchLayout>} />
          <Route path="/bench/insights" element={<BenchLayout><Insights /></BenchLayout>} />
          <Route path="/bench/benchmarks" element={<BenchLayout><Benchmarks /></BenchLayout>} />

          {/* Lab routes (scenario catalog, designer, run configurator) */}
          <Route path="/scenarios" element={<Layout><Scenarios /></Layout>} />
          <Route path="/designer" element={<Layout><Designer /></Layout>} />
          <Route path="/run" element={<Layout><Run /></Layout>} />
          <Route path="/results" element={<Layout><Results /></Layout>} />
        </Routes>
      </BrowserRouter>
    </EvidenceModeProvider>
  );
}
