import { BrowserRouter, Routes, Route } from "react-router";
import { Layout } from "./Layout";
import { Dashboard } from "./pages/Dashboard";
import { Runs } from "./pages/Runs";
import { RunDetail } from "./pages/RunDetail";
import { Scenarios } from "./pages/Scenarios";
import { Compare } from "./pages/Compare";
import { Benchmarks } from "./pages/Benchmarks";
import { Leaderboard } from "./pages/Leaderboard";
import { ScenarioDetail } from "./pages/ScenarioDetail";
import { SkillImpact } from "./pages/SkillImpact";
import { Regressions } from "./pages/Regressions";
import { Insights } from "./pages/Insights";

export function App() {
  return (
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/leaderboard" element={<Leaderboard />} />
          <Route path="/skill-impact" element={<SkillImpact />} />
          <Route path="/regressions" element={<Regressions />} />
          <Route path="/insights" element={<Insights />} />
          <Route path="/runs" element={<Runs />} />
          <Route path="/runs/:id" element={<RunDetail />} />
          <Route path="/scenarios" element={<Scenarios />} />
          <Route path="/scenarios/:id" element={<ScenarioDetail />} />
          <Route path="/compare" element={<Compare />} />
          <Route path="/benchmarks" element={<Benchmarks />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  );
}
