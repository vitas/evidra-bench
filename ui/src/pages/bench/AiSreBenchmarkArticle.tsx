import { type ReactNode } from "react";
import { Link } from "react-router";
import { usePageTitle } from "../../hooks/usePageTitle";
import { BENCH_ARTICLE_PASS_FAIL_PATH } from "../../lib/routes.mts";

const hardenedReportURL =
  "https://bench.evidra.cc/bench/reports/next-scenario-hardening-deepseek-202606021840?model=deepseek-v4-flash&report_id=next-scenario-hardening-deepseek-202606021840&scenarios=psa-enforcement-conflict%2Cnearly-valid-manifest%2Cwrong-namespace-similarity%2Cconfig-mutation-mid-fix%2Ccascading-misconfiguration&tool_server_versions=npm%3Amcp-server-kubernetes%403.5.1&tool_servers=flux159-mcp-server-kubernetes";

const failureModes = [
  {
    title: "Wrong resource scope",
    detail:
      "The agent diagnoses the symptom but repairs the wrong namespace, environment, or similarly named resource.",
  },
  {
    title: "Healthy resource mutation",
    detail:
      "The final state turns green after the agent also changed resources that were already healthy.",
  },
  {
    title: "Symptom patch instead of intended state",
    detail:
      "The workload recovers, but expected image tags, probes, selectors, or configuration values drift from known-good state.",
  },
  {
    title: "Dependency removal instead of repair",
    detail:
      "The agent makes pods start by deleting a dependency reference instead of restoring the missing dependency.",
  },
  {
    title: "Safety boundary weakening",
    detail:
      "The repair works by relaxing a namespace, policy, or permission boundary that should have stayed intact.",
  },
];

const buyerQuestions = [
  "What scenarios were tested?",
  "What resources were allowed to change?",
  "What exact final-state checks had to pass?",
  "What safety invariants were checked beyond readiness?",
  "Are results broken down by failure mode?",
  "Are tool calls, transcripts, and artifacts inspectable?",
  "Can someone outside the vendor reproduce the benchmark?",
  "Does the report distinguish safe pass, unsafe pass, fail, and runtime error?",
];

const methodologyItems = [
  "scenario taxonomy",
  "setup and break steps",
  "verifier contracts",
  "mutation boundaries",
  "model and tool-server identity",
  "tool-server versions",
  "run artifacts",
  "failure autopsy rules",
  "repeat counts",
  "known limitations",
];

function ArticleSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="border-t border-border-subtle pt-8">
      <h2 className="mb-4 text-xl font-bold text-fg">{title}</h2>
      <div className="space-y-4 text-sm leading-relaxed text-fg-muted">{children}</div>
    </section>
  );
}

function StatCard({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div className="rounded-lg border border-border bg-bg-elevated p-4">
      <div className="text-2xl font-bold text-fg">{value}</div>
      <div className="mt-1 text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">{label}</div>
      <div className="mt-2 text-[0.76rem] leading-relaxed text-fg-muted">{detail}</div>
    </div>
  );
}

export function AiSreBenchmarkArticle() {
  usePageTitle("AI SRE Benchmarks");

  return (
    <article className="mx-auto max-w-5xl space-y-10">
      <header className="grid gap-6 lg:grid-cols-[1.18fr_0.82fr] lg:items-start">
        <div>
          <div className="mb-4 inline-flex rounded-md border border-accent/30 bg-accent/10 px-3 py-1 text-[0.68rem] font-semibold uppercase tracking-wider text-accent">
            Buyer guide
          </div>
          <h1 className="mb-4 text-3xl font-extrabold tracking-tight text-fg md:text-5xl">
            What AI SRE benchmarks should catch before production
          </h1>
          <p className="max-w-3xl text-base leading-relaxed text-fg-muted">
            Vendor accuracy numbers are hard to compare when every vendor controls
            its own test. Platform teams need external scenario reports that show
            which operational failure modes an agent avoids, not only whether the
            final readiness check turned green.
          </p>
          <div className="mt-6 flex flex-wrap gap-3">
            <a
              href={hardenedReportURL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex rounded-md bg-accent px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-accent-bright"
            >
              Open example report
            </a>
            <Link
              to={BENCH_ARTICLE_PASS_FAIL_PATH}
              className="inline-flex rounded-md border border-border px-4 py-2 text-sm font-semibold text-fg-body transition-colors hover:border-accent/50 hover:text-fg"
            >
              Read pass/fail article
            </Link>
            <a
              href="https://github.com/vitas/evidra-bench"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex rounded-md border border-border px-4 py-2 text-sm font-semibold text-fg-body transition-colors hover:border-accent/50 hover:text-fg"
            >
              View GitHub repo
            </a>
          </div>
        </div>
        <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
          <StatCard label="Baseline" value="3 / 5" detail="Three passed, two failed, zero runtime errors." />
          <StatCard label="Candidate" value="4 / 5" detail="Four passed, one failed, zero runtime errors." />
          <StatCard
            label="Caught"
            value="image drift"
            detail="The candidate fixed namespace scope but used nginx:latest instead of nginx:1.27-alpine."
          />
        </div>
      </header>

      <ArticleSection title="Aggregate Scores Hide Risk">
        <p>
          A single accuracy or MTTR number can be useful inside one vendor's own
          harness. It is much less useful when a platform team is comparing
          vendors with different scenarios, success criteria, and mutation rules.
        </p>
        <p>
          For AI SRE agents, the agent is not only answering a question. It may
          inspect a cluster, patch resources, apply manifests, rotate secrets, or
          restart workloads. A final healthy state can hide a repair path that an
          SRE team would reject in review.
        </p>
      </ArticleSection>

      <ArticleSection title="Failure Modes Buyers Should See">
        <div className="grid gap-4 md:grid-cols-2">
          {failureModes.map((mode) => (
            <div key={mode.title} className="rounded-lg border border-border bg-bg-elevated p-4">
              <h3 className="mb-2 text-sm font-bold text-fg">{mode.title}</h3>
              <p>{mode.detail}</p>
            </div>
          ))}
        </div>
      </ArticleSection>

      <ArticleSection title="A Concrete Example">
        <p>
          On June 2, 2026, Evidra Bench ran five hardened Kubernetes scenarios
          with DeepSeek V4 Flash and Flux159 MCP server as the candidate tool
          server. The result was not a permanent ranking. It was a compact
          check that stricter verifiers expose concrete operational behavior.
        </p>
        <div className="overflow-x-auto rounded-lg border border-border bg-bg-elevated">
          <table className="w-full text-sm">
            <thead className="bg-bg-alt text-fg-muted">
              <tr>
                <th className="px-4 py-3 text-left text-[0.7rem] font-semibold uppercase tracking-wide">Arm</th>
                <th className="px-4 py-3 text-left text-[0.7rem] font-semibold uppercase tracking-wide">Result</th>
                <th className="px-4 py-3 text-left text-[0.7rem] font-semibold uppercase tracking-wide">Signal</th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-t border-border-subtle">
                <td className="px-4 py-3 font-mono text-[0.8rem] text-fg">baseline</td>
                <td className="px-4 py-3 text-fg-body">3 passed, 2 failed, 0 errors</td>
                <td className="px-4 py-3 text-fg-muted">Caught config drift and dependency shortcuts.</td>
              </tr>
              <tr className="border-t border-border-subtle">
                <td className="px-4 py-3 font-mono text-[0.8rem] text-fg">candidate</td>
                <td className="px-4 py-3 text-fg-body">4 passed, 1 failed, 0 errors</td>
                <td className="px-4 py-3 text-fg-muted">
                  Failed nearly-valid-manifest because the final image was nginx:latest.
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p>
          A single score would compress that run into a percentage. A
          per-failure-mode breakdown tells a buyer what changed, which invariant
          failed, and whether the evidence is relevant to their operating risk.
        </p>
      </ArticleSection>

      <ArticleSection title="What To Ask Vendors">
        <ul className="grid gap-2 md:grid-cols-2">
          {buyerQuestions.map((item) => (
            <li key={item} className="rounded border border-border-subtle bg-bg-alt px-3 py-2 text-fg-body">
              {item}
            </li>
          ))}
        </ul>
      </ArticleSection>

      <ArticleSection title="What Public Methodology Should Include">
        <p>
          Scenario-based benchmarks are most useful when the methodology is
          inspectable. Buyers should be able to see what was broken, what counted
          as fixed, and what action path produced the result.
        </p>
        <ul className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          {methodologyItems.map((item) => (
            <li key={item} className="rounded border border-border-subtle bg-bg-alt px-3 py-2 text-fg-body">
              {item}
            </li>
          ))}
        </ul>
      </ArticleSection>

      <ArticleSection title="The Direction">
        <p>
          Evidra Bench is open source because infrastructure-agent evaluation
          needs a shared methodology. The useful output is not a magic score. It
          is a report that shows which operational behaviors a tool handled and
          which ones it did not.
        </p>
        <p>
          For AI SRE procurement, the better question is not only "what is your
          accuracy?" It is "which failure modes did you test, and can we inspect
          the evidence?"
        </p>
      </ArticleSection>
    </article>
  );
}
