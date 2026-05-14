import { useEffect } from "react";
import { BrowserRouter, Navigate, Routes, Route, useLocation, useParams } from "react-router";
import { Layout } from "./components/Layout";
import { Landing } from "./pages/Landing";
import { Designer } from "./pages/Designer";
import {
  BENCH_ARTICLE_PASS_FAIL_PATH,
  BENCH_LEADERBOARD_PATH,
  BENCH_MCP_READINESS_PATH,
  BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH,
  BENCH_RUNS_PATH,
  BENCH_SAMPLE_REPORT_PATH,
  BENCH_SCENARIOS_PATH,
  BENCH_TOOL_SERVER_REPORT_PATH,
  benchRunPath,
  benchScenarioPath,
} from "./lib/routes.mts";

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
import { ToolServerReport } from "./pages/bench/ToolServerReport";
import { SampleReport } from "./pages/bench/SampleReport";
import { LiveToolServerReport } from "./pages/bench/LiveToolServerReport";
import { PublicKubernetesMCPReport } from "./pages/bench/PublicKubernetesMCPReport";
import { PassFailArticle } from "./pages/bench/PassFailArticle";

export function App() {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL.replace(/\/$/, "") || "/"}>
      <ScrollToTop />
      <Routes>
        <Route path="/" element={<Landing />} />

        {/* Bench routes (rich dashboard with leaderboard, runs, compare) */}
        <Route path="/bench" element={<BenchLayout><Leaderboard /></BenchLayout>} />
        <Route path={BENCH_LEADERBOARD_PATH} element={<BenchLayout><Leaderboard /></BenchLayout>} />
        <Route path="/bench/dashboard" element={<BenchLayout><Dashboard /></BenchLayout>} />
        <Route path="/bench/runs" element={<BenchLayout><Runs /></BenchLayout>} />
        <Route path="/bench/runs/:id" element={<BenchLayout><RunDetail /></BenchLayout>} />
        <Route path="/bench/scenarios" element={<BenchLayout><BenchScenarios /></BenchLayout>} />
        <Route path="/bench/scenarios/:id" element={<BenchLayout><ScenarioDetail /></BenchLayout>} />
        <Route path="/bench/compare" element={<BenchLayout><Compare /></BenchLayout>} />
        <Route path={BENCH_MCP_READINESS_PATH} element={<BenchLayout><ToolServerReport /></BenchLayout>} />
        <Route path={BENCH_SAMPLE_REPORT_PATH} element={<BenchLayout><SampleReport /></BenchLayout>} />
        <Route path={BENCH_ARTICLE_PASS_FAIL_PATH} element={<BenchLayout><PassFailArticle /></BenchLayout>} />
        <Route path={BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH} element={<BenchLayout><PublicKubernetesMCPReport /></BenchLayout>} />
        <Route path={BENCH_TOOL_SERVER_REPORT_PATH} element={<BenchLayout><LiveToolServerReport /></BenchLayout>} />
        <Route path="/bench/reports/:reportId" element={<BenchLayout><PublicKubernetesMCPReport /></BenchLayout>} />
        <Route path="/bench/skill-impact" element={<BenchLayout><SkillImpact /></BenchLayout>} />
        <Route path="/bench/regressions" element={<BenchLayout><Regressions /></BenchLayout>} />
        <Route path="/bench/insights" element={<BenchLayout><Insights /></BenchLayout>} />
        <Route path="/bench/benchmarks" element={<BenchLayout><Benchmarks /></BenchLayout>} />

        {/* Lab routes (scenario catalog, designer, run configurator) */}
        <Route path="/scenarios" element={<RedirectWithSearch to={BENCH_SCENARIOS_PATH} />} />
        <Route path="/scenarios/:id" element={<LegacyScenarioRedirect />} />
        <Route path="/runs" element={<RedirectWithSearch to={BENCH_RUNS_PATH} />} />
        <Route path="/runs/:id" element={<LegacyRunRedirect />} />
        <Route path="/designer" element={<Layout><Designer /></Layout>} />
        <Route path="/run" element={<RedirectWithSearch to={BENCH_SCENARIOS_PATH} />} />
        <Route path="/results" element={<RedirectWithSearch to={BENCH_RUNS_PATH} />} />
      </Routes>
    </BrowserRouter>
  );
}

function ScrollToTop() {
  const { pathname, search } = useLocation();

  useEffect(() => {
    window.scrollTo({ top: 0, left: 0, behavior: "auto" });
  }, [pathname, search]);

  return null;
}

function RedirectWithSearch({ to }: { to: string }) {
  const { search } = useLocation();
  return <Navigate to={`${to}${search}`} replace />;
}

function LegacyRunRedirect() {
  const { id } = useParams();
  return <RedirectWithSearch to={id ? benchRunPath(id) : BENCH_RUNS_PATH} />;
}

function LegacyScenarioRedirect() {
  const { id } = useParams();
  return <RedirectWithSearch to={id ? benchScenarioPath(id) : BENCH_SCENARIOS_PATH} />;
}
