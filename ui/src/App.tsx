import { BrowserRouter, Routes, Route } from "react-router";
import { Layout } from "./Layout";
import { Dashboard } from "./pages/Dashboard";
import { Runs } from "./pages/Runs";
import { RunDetail } from "./pages/RunDetail";
import { Scenarios } from "./pages/Scenarios";
import { Compare } from "./pages/Compare";
import { Benchmarks } from "./pages/Benchmarks";
import { Leaderboard } from "./pages/Leaderboard";

export function App() {
  return (
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/leaderboard" element={<Leaderboard />} />
          <Route path="/runs" element={<Runs />} />
          <Route path="/runs/:id" element={<RunDetail />} />
          <Route path="/scenarios" element={<Scenarios />} />
          <Route path="/compare" element={<Compare />} />
          <Route path="/benchmarks" element={<Benchmarks />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  );
}
