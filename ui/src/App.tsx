import { lazy, Suspense, useEffect } from "react";
import { BrowserRouter, Navigate, Routes, Route, useLocation, useParams } from "react-router";
import { Layout } from "./components/Layout";
import { Landing } from "./pages/Landing";
import {
  BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH,
  BENCH_ARTICLE_PASS_FAIL_PATH,
  BENCH_LEADERBOARD_PATH,
  BENCH_MCP_READINESS_PATH,
  BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH,
  BENCH_REVIEWS_PATH,
  BENCH_RUNS_PATH,
  BENCH_SAMPLE_REPORT_PATH,
  BENCH_SCENARIOS_PATH,
  BENCH_SESSION_PATH,
  BENCH_TOOL_SERVER_REPORT_PATH,
  benchRunPath,
  benchScenarioPath,
} from "./lib/routes.mts";

import { Layout as BenchLayout } from "./pages/bench/Layout";

const Leaderboard = lazy(() => import("./pages/bench/Leaderboard").then((m) => ({ default: m.Leaderboard })));
const Dashboard = lazy(() => import("./pages/bench/Dashboard").then((m) => ({ default: m.Dashboard })));
const Runs = lazy(() => import("./pages/bench/Runs").then((m) => ({ default: m.Runs })));
const Reviews = lazy(() => import("./pages/bench/Reviews").then((m) => ({ default: m.Reviews })));
const Session = lazy(() => import("./pages/bench/Session").then((m) => ({ default: m.Session })));
const RunDetail = lazy(() => import("./pages/bench/RunDetail").then((m) => ({ default: m.RunDetail })));
const BenchScenarios = lazy(() => import("./pages/bench/Scenarios").then((m) => ({ default: m.Scenarios })));
const ScenarioDetail = lazy(() => import("./pages/bench/ScenarioDetail").then((m) => ({ default: m.ScenarioDetail })));
const Compare = lazy(() => import("./pages/bench/Compare").then((m) => ({ default: m.Compare })));
const SkillImpact = lazy(() => import("./pages/bench/SkillImpact").then((m) => ({ default: m.SkillImpact })));
const Benchmarks = lazy(() => import("./pages/bench/Benchmarks").then((m) => ({ default: m.Benchmarks })));
const Regressions = lazy(() => import("./pages/bench/Regressions").then((m) => ({ default: m.Regressions })));
const Insights = lazy(() => import("./pages/bench/Insights").then((m) => ({ default: m.Insights })));
const ToolServerReport = lazy(() => import("./pages/bench/ToolServerReport").then((m) => ({ default: m.ToolServerReport })));
const SampleReport = lazy(() => import("./pages/bench/SampleReport").then((m) => ({ default: m.SampleReport })));
const LiveToolServerReport = lazy(() => import("./pages/bench/LiveToolServerReport").then((m) => ({ default: m.LiveToolServerReport })));
const PublicKubernetesMCPReport = lazy(() => import("./pages/bench/PublicKubernetesMCPReport").then((m) => ({ default: m.PublicKubernetesMCPReport })));
const PassFailArticle = lazy(() => import("./pages/bench/PassFailArticle").then((m) => ({ default: m.PassFailArticle })));
const AiSreBenchmarkArticle = lazy(() => import("./pages/bench/AiSreBenchmarkArticle").then((m) => ({ default: m.AiSreBenchmarkArticle })));
const Designer = lazy(() => import("./pages/Designer").then((m) => ({ default: m.Designer })));

export function App() {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL.replace(/\/$/, "") || "/"}>
      <ScrollToTop />
      <Suspense fallback={<div className="p-6 text-sm text-fg-muted">Loading...</div>}>
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
          <Route path={BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH} element={<BenchLayout><AiSreBenchmarkArticle /></BenchLayout>} />
          <Route path={BENCH_ARTICLE_PASS_FAIL_PATH} element={<BenchLayout><PassFailArticle /></BenchLayout>} />
          <Route path={BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH} element={<BenchLayout><PublicKubernetesMCPReport /></BenchLayout>} />
          <Route path={BENCH_TOOL_SERVER_REPORT_PATH} element={<BenchLayout><LiveToolServerReport /></BenchLayout>} />
          <Route path="/bench/reports/:reportId" element={<BenchLayout><PublicKubernetesMCPReport /></BenchLayout>} />
          <Route path="/bench/skill-impact" element={<BenchLayout><SkillImpact /></BenchLayout>} />
          <Route path="/bench/regressions" element={<BenchLayout><Regressions /></BenchLayout>} />
          <Route path="/bench/insights" element={<BenchLayout><Insights /></BenchLayout>} />
          <Route path="/bench/benchmarks" element={<BenchLayout><Benchmarks /></BenchLayout>} />
          <Route path={BENCH_REVIEWS_PATH} element={<BenchLayout><Reviews /></BenchLayout>} />
          <Route path={BENCH_SESSION_PATH} element={<BenchLayout><Session /></BenchLayout>} />

          {/* Lab routes (scenario catalog, designer, run configurator) */}
          <Route path="/scenarios" element={<RedirectWithSearch to={BENCH_SCENARIOS_PATH} />} />
          <Route path="/scenarios/:id" element={<LegacyScenarioRedirect />} />
          <Route path="/runs" element={<RedirectWithSearch to={BENCH_RUNS_PATH} />} />
          <Route path="/runs/:id" element={<LegacyRunRedirect />} />
          <Route path="/designer" element={<Layout><Designer /></Layout>} />
          <Route path="/run" element={<RedirectWithSearch to={BENCH_SCENARIOS_PATH} />} />
          <Route path="/results" element={<RedirectWithSearch to={BENCH_RUNS_PATH} />} />
        </Routes>
      </Suspense>
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
