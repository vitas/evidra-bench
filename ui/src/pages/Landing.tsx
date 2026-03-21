import { useState, useEffect, useRef } from "react";
import { Link } from "react-router";

const TRACKS = [
  { id: "workloads", label: "Workloads", count: 12, source: "CKA", icon: "cube" },
  { id: "troubleshooting", label: "Troubleshooting", count: 10, source: "CKA", icon: "search" },
  { id: "networking", label: "Networking", count: 3, source: "CKA", icon: "globe" },
  { id: "pod-security", label: "Pod Security", count: 10, source: "CKS", icon: "shield" },
  { id: "runtime-security", label: "Runtime Security", count: 2, source: "CKS", icon: "zap" },
  { id: "release-ops", label: "Release Ops", count: 8, source: "Custom", icon: "rocket" },
  { id: "storage", label: "Storage", count: 2, source: "CKA", icon: "database" },
  { id: "platform-eng", label: "Platform Eng", count: 1, source: "Custom", icon: "cloud" },
];

const TERMINAL_LINES = [
  { text: "$ infra-bench certify --track pod-security --model sonnet", delay: 0, type: "input" as const },
  { text: "", delay: 600, type: "blank" as const },
  { text: "[1/7] networkpolicy-blocking (L2) ...", delay: 800, type: "progress" as const },
  { text: "  PASS  12.3s", delay: 1400, type: "pass" as const },
  { text: "[2/7] network-policy-fix (L2) ...", delay: 1800, type: "progress" as const },
  { text: "  PASS  18.7s", delay: 2400, type: "pass" as const },
  { text: "[3/7] readonly-filesystem (L2) ...", delay: 2800, type: "progress" as const },
  { text: "  PASS  9.2s", delay: 3300, type: "pass" as const },
  { text: "[4/7] stale-sa-token (L2) ...", delay: 3700, type: "progress" as const },
  { text: "  PASS  14.1s", delay: 4200, type: "pass" as const },
  { text: "[5/7] security-group-too-open (L3) ...", delay: 4600, type: "progress" as const },
  { text: "  PASS  22.4s", delay: 5300, type: "pass" as const },
  { text: "[6/7] s3-bucket-public-access (L3) ...", delay: 5700, type: "progress" as const },
  { text: "  PASS  19.8s", delay: 6400, type: "pass" as const },
  { text: "[7/7] privileged-pod-review (L3) ...", delay: 6800, type: "progress" as const },
  { text: "  PASS  8.1s", delay: 7300, type: "pass" as const },
  { text: "", delay: 7600, type: "blank" as const },
  { text: "════════════════════════════════════════════", delay: 7800, type: "border" as const },
  { text: "  EVIDRA AGENT CERTIFICATION", delay: 8000, type: "title" as const },
  { text: "════════════════════════════════════════════", delay: 8200, type: "border" as const },
  { text: "  Agent:    sonnet (bifrost)", delay: 8400, type: "info" as const },
  { text: "  Track:    Pod Security", delay: 8600, type: "info" as const },
  { text: "", delay: 8800, type: "blank" as const },
  { text: "  Grade:    PROFICIENT (L3)", delay: 9000, type: "grade" as const },
  { text: "", delay: 9200, type: "blank" as const },
  { text: "  L2 Diagnose:   4/4   ✓", delay: 9400, type: "level-pass" as const },
  { text: "  L3 Judge:      3/3   ✓", delay: 9600, type: "level-pass" as const },
  { text: "", delay: 9800, type: "blank" as const },
  { text: "  Overall:  7/7 (100.0%)", delay: 10000, type: "info" as const },
  { text: "════════════════════════════════════════════", delay: 10200, type: "border" as const },
];

function TerminalAnimation() {
  const [visibleLines, setVisibleLines] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const timers: ReturnType<typeof setTimeout>[] = [];
    TERMINAL_LINES.forEach((line, i) => {
      timers.push(setTimeout(() => {
        setVisibleLines(i + 1);
        if (containerRef.current) {
          containerRef.current.scrollTop = containerRef.current.scrollHeight;
        }
      }, line.delay));
    });
    return () => timers.forEach(clearTimeout);
  }, []);

  return (
    <div className="relative rounded-xl overflow-hidden border border-[#1e3a2c] shadow-[0_0_60px_rgba(52,211,153,0.08)]">
      {/* Terminal header */}
      <div className="flex items-center gap-2 px-4 py-2.5 bg-[#0a110e] border-b border-[#1e3a2c]">
        <div className="flex gap-1.5">
          <div className="w-2.5 h-2.5 rounded-full bg-[#ef4444]/70" />
          <div className="w-2.5 h-2.5 rounded-full bg-[#eab308]/70" />
          <div className="w-2.5 h-2.5 rounded-full bg-[#22c55e]/70" />
        </div>
        <span className="text-[0.65rem] text-[#6b8f7b] font-mono ml-2">infra-bench — certification run</span>
      </div>
      {/* Terminal body */}
      <div
        ref={containerRef}
        className="bg-[#0c0f0e] p-4 font-mono text-[0.72rem] leading-relaxed h-[380px] overflow-hidden"
      >
        {TERMINAL_LINES.slice(0, visibleLines).map((line, i) => (
          <div
            key={i}
            className={`
              ${line.type === "input" ? "text-[#34d399]" : ""}
              ${line.type === "progress" ? "text-[#6b8f7b]" : ""}
              ${line.type === "pass" ? "text-[#22c55e] font-semibold" : ""}
              ${line.type === "border" ? "text-[#34d399]/60" : ""}
              ${line.type === "title" ? "text-[#d1fae5] font-bold" : ""}
              ${line.type === "grade" ? "text-[#34d399] font-bold text-[0.85rem]" : ""}
              ${line.type === "level-pass" ? "text-[#22c55e]" : ""}
              ${line.type === "info" ? "text-[#a7cdb8]" : ""}
              ${line.type === "blank" ? "h-4" : ""}
            `}
            style={{ animation: "fadeInLine 0.15s ease-out" }}
          >
            {line.text}
          </div>
        ))}
        {visibleLines < TERMINAL_LINES.length && (
          <span className="inline-block w-2 h-4 bg-[#34d399] animate-pulse ml-0.5" />
        )}
      </div>
    </div>
  );
}

function TrackIcon({ icon }: { icon: string }) {
  const paths: Record<string, string> = {
    cube: "M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z",
    search: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z",
    globe: "M12 2a10 10 0 100 20 10 10 0 000-20zM2 12h20M12 2a15 15 0 014 10 15 15 0 01-4 10M12 2a15 15 0 00-4 10 15 15 0 004 10",
    shield: "M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z",
    zap: "M13 2L3 14h9l-1 10 10-12h-9l1-10z",
    rocket: "M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 00-2.91-.09zM12 15l-3-3a22 22 0 012-3.95A12.88 12.88 0 0122 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 01-4 2z",
    database: "M12 2C6.48 2 2 4.02 2 6.5v11C2 19.98 6.48 22 12 22s10-2.02 10-4.5v-11C22 4.02 17.52 2 12 2zM2 11.5c0 2.48 4.48 4.5 10 4.5s10-2.02 10-4.5",
    cloud: "M18 10h-1.26A8 8 0 109 20h9a5 5 0 000-10z",
  };
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round">
      <path d={paths[icon] || paths.cube} />
    </svg>
  );
}

const STATS = [
  { value: "48", label: "Scenarios" },
  { value: "8", label: "Exam Tracks" },
  { value: "5", label: "Categories" },
  { value: "4", label: "Cert Levels" },
];

export function Landing() {
  return (
    <div className="min-h-screen bg-[#0a0d0c] text-[#d1fae5] overflow-hidden">
      {/* Subtle grid background */}
      <div
        className="fixed inset-0 opacity-[0.03]"
        style={{
          backgroundImage: `linear-gradient(#34d399 1px, transparent 1px), linear-gradient(90deg, #34d399 1px, transparent 1px)`,
          backgroundSize: "60px 60px",
        }}
      />

      {/* Hero */}
      <section className="relative max-w-6xl mx-auto px-6 pt-20 pb-16">
        {/* Glow effect */}
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[600px] h-[400px] bg-[#059669]/8 rounded-full blur-[120px]" />

        <div className="relative grid lg:grid-cols-2 gap-16 items-center">
          {/* Left: messaging */}
          <div>
            <Link
              to="/results"
              className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full border border-[#34d399]/40 bg-[#059669]/10 text-[0.75rem] text-[#34d399] font-medium mb-8 hover:bg-[#059669]/20 hover:border-[#34d399]/60 transition-all group"
            >
              <span className="w-1.5 h-1.5 rounded-full bg-[#22c55e] animate-pulse" />
              8 models certified — view live results
              <svg className="w-3 h-3 group-hover:translate-x-0.5 transition-transform" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.5}>
                <path d="M5 12h14M12 5l7 7-7 7" />
              </svg>
            </Link>

            <h1 className="text-[3.2rem] leading-[1.08] font-extrabold tracking-tight mb-6">
              Get your AI agent{" "}
              <span className="text-transparent bg-clip-text bg-gradient-to-r from-[#34d399] to-[#059669]">
                certified
              </span>
            </h1>

            <p className="text-[1.05rem] text-[#6b8f7b] leading-relaxed mb-10 max-w-lg">
              48 scenarios across Kubernetes, Helm, ArgoCD, Terraform, and AWS.
              Real clusters. Real failures. Behavioral signals that reveal
              how your agent thinks — not just whether it fixes things.
            </p>

            <div className="flex items-center gap-4 mb-12">
              <Link
                to="/scenarios"
                className="inline-flex items-center gap-2 px-6 py-3 bg-[#059669] hover:bg-[#047857] text-white text-[0.88rem] font-semibold rounded-lg transition-all hover:shadow-[0_0_30px_rgba(5,150,105,0.3)]"
              >
                Browse Scenarios
                <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.5}>
                  <path d="M5 12h14M12 5l7 7-7 7" />
                </svg>
              </Link>
              <Link
                to="/designer"
                className="inline-flex items-center gap-2 px-6 py-3 border border-[#1e3a2c] text-[#a7cdb8] text-[0.88rem] font-medium rounded-lg hover:border-[#34d399]/50 hover:text-[#d1fae5] transition-all"
              >
                Design a Puzzle
              </Link>
            </div>

            {/* Stats row */}
            <div className="flex gap-8">
              {STATS.map((stat) => (
                <div key={stat.label}>
                  <div className="text-2xl font-bold text-[#34d399]">{stat.value}</div>
                  <div className="text-[0.72rem] text-[#6b8f7b] uppercase tracking-wider">{stat.label}</div>
                </div>
              ))}
            </div>
          </div>

          {/* Right: terminal animation */}
          <div className="hidden lg:block">
            <TerminalAnimation />
          </div>
        </div>
      </section>

      {/* Certification levels */}
      <section className="relative max-w-6xl mx-auto px-6 py-20">
        <h2 className="text-center text-[1.6rem] font-bold mb-3">Four Certification Levels</h2>
        <p className="text-center text-[0.88rem] text-[#6b8f7b] mb-12 max-w-xl mx-auto">
          Not just pass/fail — we measure how your agent thinks. Each level tests deeper capabilities.
        </p>

        <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {[
            { level: "L1", name: "Fix", desc: "Applies the obvious fix to a clear problem", color: "#3b82f6", analogy: "Junior" },
            { level: "L2", name: "Diagnose", desc: "Investigates before acting, reads logs, correlates", color: "#22c55e", analogy: "Mid-level" },
            { level: "L3", name: "Judge", desc: "Weighs trade-offs, avoids traps, scopes decisions", color: "#eab308", analogy: "Senior" },
            { level: "L4", name: "Investigate", desc: "Multi-step forensics, traces root cause across systems", color: "#ef4444", analogy: "Staff" },
          ].map((l) => (
            <div
              key={l.level}
              className="relative p-5 rounded-xl border border-[#1e3a2c] bg-[#0c0f0e] hover:border-[#34d399]/30 transition-all group"
            >
              <div
                className="absolute top-0 left-0 w-full h-0.5 rounded-t-xl"
                style={{ background: l.color }}
              />
              <div className="flex items-center gap-2 mb-3">
                <span
                  className="text-[0.65rem] font-bold px-2 py-0.5 rounded"
                  style={{ background: `${l.color}20`, color: l.color }}
                >
                  {l.level}
                </span>
                <span className="text-[0.88rem] font-semibold text-[#d1fae5]">{l.name}</span>
              </div>
              <p className="text-[0.78rem] text-[#6b8f7b] leading-relaxed mb-3">{l.desc}</p>
              <span className="text-[0.65rem] text-[#4a6b5a] uppercase tracking-wider">
                {l.analogy} engineer equivalent
              </span>
            </div>
          ))}
        </div>
      </section>

      {/* Tracks */}
      <section className="relative max-w-6xl mx-auto px-6 py-20">
        <h2 className="text-center text-[1.6rem] font-bold mb-3">Exam-Aligned Tracks</h2>
        <p className="text-center text-[0.88rem] text-[#6b8f7b] mb-12 max-w-xl mx-auto">
          Mapped to CKA/CKS certification domains. Your agent earns a grade per track.
        </p>

        <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-3">
          {TRACKS.map((track) => (
            <Link
              key={track.id}
              to={`/scenarios?track=${track.id}`}
              className="flex items-center gap-3 p-4 rounded-lg border border-[#1e3a2c] bg-[#0c0f0e] hover:border-[#34d399]/40 hover:bg-[#111916] transition-all group"
            >
              <div className="text-[#34d399] opacity-60 group-hover:opacity-100 transition-opacity">
                <TrackIcon icon={track.icon} />
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-[0.82rem] font-semibold text-[#d1fae5] group-hover:text-[#34d399] transition-colors truncate">
                  {track.label}
                </div>
                <div className="text-[0.65rem] text-[#4a6b5a]">
                  {track.count} scenarios · {track.source}
                </div>
              </div>
              <svg className="w-3.5 h-3.5 text-[#1e3a2c] group-hover:text-[#34d399] transition-colors" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
                <path d="M9 18l6-6-6-6" />
              </svg>
            </Link>
          ))}
        </div>
      </section>

      {/* How it works */}
      <section className="relative max-w-6xl mx-auto px-6 py-20">
        <h2 className="text-center text-[1.6rem] font-bold mb-12">How Certification Works</h2>

        <div className="grid md:grid-cols-3 gap-8">
          {[
            {
              step: "01",
              title: "Choose a track",
              desc: "Pick from CKA/CKS-aligned tracks: Workloads, Pod Security, Networking, Release Ops, and more.",
            },
            {
              step: "02",
              title: "Run certification",
              desc: "Your agent faces real failures in sandbox clusters. Broken deployments, open security groups, cascading misconfigurations.",
            },
            {
              step: "03",
              title: "Get certified",
              desc: "Receive a grade: Novice → Competent → Proficient → Expert. Based on pass rate, behavioral signals, and trap avoidance.",
            },
          ].map((item) => (
            <div key={item.step} className="relative">
              <div className="text-[3rem] font-black text-[#34d399]/10 absolute -top-4 -left-2 select-none">
                {item.step}
              </div>
              <div className="relative pt-8">
                <h3 className="text-[1rem] font-semibold text-[#d1fae5] mb-2">{item.title}</h3>
                <p className="text-[0.82rem] text-[#6b8f7b] leading-relaxed">{item.desc}</p>
              </div>
            </div>
          ))}
        </div>

        {/* CTA */}
        <div className="text-center mt-16">
          <code className="block text-[0.82rem] text-[#34d399] bg-[#0c0f0e] border border-[#1e3a2c] rounded-lg px-6 py-3 font-mono inline-block mb-6">
            infra-bench certify --track workloads --model your-agent --provider bifrost
          </code>
          <div className="flex justify-center gap-4">
            <a
              href="https://github.com/vitas/evidra"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 px-5 py-2.5 bg-[#d1fae5] text-[#064e3b] text-[0.82rem] font-semibold rounded-lg hover:bg-white transition-colors"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
              </svg>
              Get Started
            </a>
            <Link
              to="/scenarios"
              className="inline-flex items-center gap-2 px-5 py-2.5 border border-[#1e3a2c] text-[#a7cdb8] text-[0.82rem] font-medium rounded-lg hover:border-[#34d399]/50 transition-colors"
            >
              View All 48 Scenarios
            </Link>
            <Link
              to="/results"
              className="inline-flex items-center gap-2 px-5 py-2.5 border border-[#1e3a2c] text-[#a7cdb8] text-[0.82rem] font-medium rounded-lg hover:border-[#34d399]/50 transition-colors"
            >
              Exam Results
            </Link>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="max-w-6xl mx-auto px-6 py-8 border-t border-[#1e3a2c]">
        <div className="flex items-center justify-between text-[0.72rem] text-[#4a6b5a]">
          <span>evidra.lab — AI agent certification</span>
          <div className="flex items-center gap-4">
            <a href="https://evidra.cc/bench" target="_blank" rel="noopener noreferrer" className="hover:text-[#34d399] transition-colors">
              Leaderboard
            </a>
            <a href="https://github.com/vitas/evidra" target="_blank" rel="noopener noreferrer" className="hover:text-[#34d399] transition-colors">
              GitHub
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
