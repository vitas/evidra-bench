import { Link } from "react-router";
import { ArrowIcon, FOCUS_RING, PublicFooter, PublicHeader } from "../components/PublicChrome";
import { SCENARIOS } from "../data/catalog";
import { CORE_REPORT, type CoreReportCase } from "../data/coreReport";
import { BENCH_KUBERNETES_CORE_PATH, BENCH_SCENARIOS_PATH, benchScenarioPath } from "../lib/routes.mts";

const TITLES = new Map(SCENARIOS.map((item) => [item.id, item.title]));

const VERDICT_STYLES: Record<string, string> = {
  PASS: "bg-accent-subtle text-accent",
  FAIL: "bg-danger-tint text-danger",
  UNSAFE: "bg-warning-tint text-warning",
  INCOMPLETE: "bg-bg-alt text-fg-muted",
};

function verdictStyle(verdict: string) {
  return VERDICT_STYLES[verdict] ?? "bg-bg-alt text-fg-muted";
}

function caseTitle(id: string) {
  return TITLES.get(id) ?? id;
}

function formatSeconds(value: number) {
  if (!value) return "—";
  if (value < 60) return `${Math.round(value)}s`;
  const minutes = Math.floor(value / 60);
  const seconds = Math.round(value % 60);
  return `${minutes}m ${seconds}s`;
}

function formatNumber(value: number) {
  return new Intl.NumberFormat("en-US").format(value);
}

function formatCost(value: number) {
  if (!value) return "$0.00";
  if (value < 0.01) return `$${value.toFixed(4)}`;
  return `$${value.toFixed(2)}`;
}

function summarize(cases: CoreReportCase[]) {
  const verdicts = new Map<string, number>();
  let turns = 0;
  let mutations = 0;
  let cost = 0;
  let duration = 0;
  let checksPassed = 0;
  let checksTotal = 0;

  for (const item of cases) {
    verdicts.set(item.verdict, (verdicts.get(item.verdict) ?? 0) + 1);
    turns += item.behavior.turns;
    mutations += item.behavior.mutationCount;
    cost += item.behavior.estimatedCostUsd;
    duration += item.durationSeconds;
    checksPassed += item.checks.passed;
    checksTotal += item.checks.total;
  }

  return { verdicts, turns, mutations, cost, duration, checksPassed, checksTotal };
}

function CaseCard({ item, index }: { item: CoreReportCase; index: number }) {
  const safetyChecks = item.checks.items.filter((check) => check.type === "assert-v2");
  const otherChecks = item.checks.items.filter((check) => check.type !== "assert-v2");

  return (
    <article className="overflow-hidden rounded-2xl border border-border bg-bg-elevated">
      <div className="flex flex-wrap items-start justify-between gap-4 border-b border-border-subtle px-5 py-5 sm:px-6">
        <div className="min-w-0">
          <p className="font-mono text-xs text-fg-muted">
            {String(index + 1).padStart(2, "0")} · {item.id}
          </p>
          <h3 className="mt-2 text-lg font-semibold leading-snug text-fg">{caseTitle(item.id)}</h3>
        </div>
        <span className={`rounded-md px-3 py-1 font-mono text-xs font-semibold ${verdictStyle(item.verdict)}`}>
          {item.verdict}
        </span>
      </div>

      <dl className="grid grid-cols-2 gap-x-4 gap-y-4 px-5 py-5 sm:grid-cols-4 sm:px-6">
        <div>
          <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Checks</dt>
          <dd className="mt-1 font-mono text-sm text-fg-body">
            {item.checks.passed}/{item.checks.total}
          </dd>
        </div>
        <div>
          <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Mutations</dt>
          <dd className="mt-1 font-mono text-sm text-fg-body">{item.behavior.mutationCount}</dd>
        </div>
        <div>
          <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Turns</dt>
          <dd className="mt-1 font-mono text-sm text-fg-body">{item.behavior.turns}</dd>
        </div>
        <div>
          <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Duration</dt>
          <dd className="mt-1 font-mono text-sm text-fg-body">{formatSeconds(item.durationSeconds)}</dd>
        </div>
        <div>
          <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Diagnosis depth</dt>
          <dd className="mt-1 font-mono text-sm text-fg-body">{item.behavior.diagnosisDepth}</dd>
        </div>
        <div>
          <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Tokens</dt>
          <dd className="mt-1 font-mono text-sm text-fg-body">
            {formatNumber(item.behavior.promptTokens + item.behavior.completionTokens)}
          </dd>
        </div>
        <div>
          <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Cost</dt>
          <dd className="mt-1 font-mono text-sm text-fg-body">{formatCost(item.behavior.estimatedCostUsd)}</dd>
        </div>
        <div>
          <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Audit events</dt>
          <dd className="mt-1 font-mono text-sm text-fg-body">{formatNumber(item.evidence.auditEvents)}</dd>
        </div>
      </dl>

      <div className="border-t border-border-subtle px-5 py-4 sm:px-6">
        <p className="text-sm leading-relaxed text-fg-muted">{item.autopsy.summary}</p>
        <p className="mt-2 font-mono text-xs text-fg-muted">
          evidence: audit {item.evidence.auditCoverage || "n/a"} · snapshots {item.evidence.snapshotCoverage || "n/a"} ·
          allowed changes {item.evidence.allowedChanges}
        </p>
      </div>

      <details className="border-t border-border-subtle px-5 py-4 sm:px-6">
        <summary className={`cursor-pointer text-sm font-semibold text-accent ${FOCUS_RING}`}>
          Inspect {safetyChecks.length + otherChecks.length} checks and the evidence digests
        </summary>

        <div className="mt-4 space-y-3">
          {[...otherChecks, ...safetyChecks].map((check) => (
            <div key={`${check.type}-${check.name}`} className="rounded-lg border border-border bg-bg p-4">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="font-mono text-xs text-fg-body">
                  {check.type} · {check.name}
                </span>
                <span className={`font-mono text-xs font-semibold ${check.verdict === "pass" ? "text-accent" : "text-danger"}`}>
                  {check.verdict}
                </span>
              </div>
              {check.message ? (
                <pre className="mt-3 max-h-72 overflow-auto whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-fg-muted">
                  {check.message}
                </pre>
              ) : null}
            </div>
          ))}
        </div>

        <dl className="mt-4 space-y-2 font-mono text-xs text-fg-muted">
          <div className="flex flex-wrap gap-2">
            <dt className="font-semibold">audit</dt>
            <dd className="break-all">{item.evidence.auditDigest || "n/a"}</dd>
          </div>
          <div className="flex flex-wrap gap-2">
            <dt className="font-semibold">baseline</dt>
            <dd className="break-all">{item.evidence.baselineDigest || "n/a"}</dd>
          </div>
          <div className="flex flex-wrap gap-2">
            <dt className="font-semibold">post-agent</dt>
            <dd className="break-all">{item.evidence.postAgentDigest || "n/a"}</dd>
          </div>
          <div className="flex flex-wrap gap-2">
            <dt className="font-semibold">stability</dt>
            <dd className="break-all">{item.evidence.stabilityDigest || "n/a"}</dd>
          </div>
          <div className="flex flex-wrap gap-2">
            <dt className="font-semibold">artifacts</dt>
            <dd className="break-all">{item.artifactDir}</dd>
          </div>
        </dl>
      </details>
    </article>
  );
}

export function KubernetesCoreReport() {
  const totals = summarize(CORE_REPORT.cases);
  const verdictSummary = [...totals.verdicts.entries()].sort((a, b) => a[0].localeCompare(b[0]));

  return (
    <div className="min-h-screen bg-bg text-fg">
      <PublicHeader />

      <main>
        <section className="mx-auto max-w-7xl px-5 pb-12 pt-12 sm:px-6 sm:pb-16 sm:pt-16">
          <p className="text-sm font-semibold uppercase tracking-[0.08em] text-accent">Public run report · {CORE_REPORT.suite}</p>
          <h1 className="mt-5 max-w-[20ch] text-[clamp(2.4rem,5.4vw,4rem)] font-semibold leading-[1.05] tracking-[-0.03em] text-fg">
            {CORE_REPORT.label} on the 12-case core pack.
          </h1>
          <p className="mt-6 max-w-3xl text-base leading-relaxed text-fg-muted sm:text-lg">
            One run, {CORE_REPORT.cases.length} cases, no curation. Every verdict below is accompanied by the audit window and
            the state digests the run produced, so a reader can check the claim instead of trusting it.
          </p>

          <dl className="mt-8 flex flex-wrap gap-3">
            <div className="rounded-xl border border-border bg-bg-elevated px-5 py-3">
              <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Model</dt>
              <dd className="mt-1 font-mono text-sm text-fg-body">{CORE_REPORT.model}</dd>
            </div>
            <div className="rounded-xl border border-border bg-bg-elevated px-5 py-3">
              <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Provider</dt>
              <dd className="mt-1 font-mono text-sm text-fg-body">{CORE_REPORT.provider || "n/a"}</dd>
            </div>
            <div className="rounded-xl border border-border bg-bg-elevated px-5 py-3">
              <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Environment</dt>
              <dd className="mt-1 font-mono text-sm text-fg-body">{CORE_REPORT.environment}</dd>
            </div>
            <div className="rounded-xl border border-border bg-bg-elevated px-5 py-3">
              <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Checks passed</dt>
              <dd className="mt-1 font-mono text-sm text-fg-body">
                {totals.checksPassed}/{totals.checksTotal}
              </dd>
            </div>
            <div className="rounded-xl border border-border bg-bg-elevated px-5 py-3">
              <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Mutations observed</dt>
              <dd className="mt-1 font-mono text-sm text-fg-body">{totals.mutations}</dd>
            </div>
            <div className="rounded-xl border border-border bg-bg-elevated px-5 py-3">
              <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Model cost</dt>
              <dd className="mt-1 font-mono text-sm text-fg-body">
                {formatCost(totals.cost)} · {formatNumber(totals.turns)} turns
              </dd>
            </div>
            <div className="rounded-xl border border-border bg-bg-elevated px-5 py-3">
              <dt className="text-xs font-semibold uppercase tracking-[0.08em] text-fg-muted">Wall clock</dt>
              <dd className="mt-1 font-mono text-sm text-fg-body">{formatSeconds(totals.duration)}</dd>
            </div>
          </dl>

          <ul className="mt-6 flex flex-wrap gap-3">
            {verdictSummary.map(([verdict, count]) => (
              <li
                key={verdict}
                className={`rounded-md px-4 py-2 font-mono text-sm font-semibold ${verdictStyle(verdict)}`}
              >
                {verdict} {count}
              </li>
            ))}
          </ul>

          <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:items-center">
            <Link
              to={BENCH_KUBERNETES_CORE_PATH}
              className={`inline-flex min-h-11 items-center justify-center gap-2 rounded-lg border border-border bg-bg-elevated px-5 py-3 text-sm font-semibold text-fg transition-colors hover:border-accent hover:text-accent ${FOCUS_RING}`}
            >
              How these 12 cases are admitted
              <ArrowIcon />
            </Link>
            <Link to={BENCH_SCENARIOS_PATH} className={`inline-flex min-h-11 items-center px-2 text-sm font-semibold text-fg-muted hover:text-accent ${FOCUS_RING}`}>
              Browse the scenario catalog
            </Link>
          </div>
        </section>

        <section className="border-y border-border bg-bg-elevated">
          <div className="mx-auto max-w-7xl px-5 py-10 sm:px-6 sm:py-12">
            <h2 className="text-lg font-semibold text-fg">How to read this report</h2>
            <ul className="mt-4 grid gap-4 md:grid-cols-2">
              <li className="text-sm leading-relaxed text-fg-muted">
                A verdict is the suite's own canonical result: <span className="font-mono text-fg-body">PASS</span>,{" "}
                <span className="font-mono text-fg-body">FAIL</span>,{" "}
                <span className="font-mono text-fg-body">UNSAFE</span> or{" "}
                <span className="font-mono text-fg-body">INCOMPLETE</span>. Nothing here is re-scored for publication.
              </li>
              <li className="text-sm leading-relaxed text-fg-muted">
                Mutations are the changes the agent actually made to the cluster. Restraint cases pass with zero mutations,
                which is why the number is shown next to every verdict.
              </li>
              <li className="text-sm leading-relaxed text-fg-muted">
                Audit events and the four digests are the run's evidence: the API-audit window, the baseline, the post-agent
                state, and the stability snapshot taken after the agent stopped.
              </li>
              <li className="text-sm leading-relaxed text-fg-muted">
                This is a single run of a free-tier model on one cluster provider. It measures that configuration, not the
                model family, and it is not a production certification.
              </li>
            </ul>
            {CORE_REPORT.notes ? (
              <p className="mt-6 rounded-xl border border-border bg-bg p-4 text-sm leading-relaxed text-fg-muted">
                {CORE_REPORT.notes}
              </p>
            ) : null}
          </div>
        </section>

        <section className="mx-auto max-w-7xl px-5 py-14 sm:px-6 sm:py-20">
          <h2 className="text-[clamp(1.8rem,3.4vw,2.6rem)] font-semibold leading-tight tracking-tight text-fg">
            All {CORE_REPORT.cases.length} cases
          </h2>
          <p className="mt-4 max-w-2xl text-base leading-relaxed text-fg-muted">
            Ordered by scenario id. Open any case to see the checks it was graded on and the digests behind its evidence.
          </p>

          <div className="mt-8 space-y-5">
            {CORE_REPORT.cases.map((item, index) => (
              <CaseCard key={`${item.id}-${item.runId}`} item={item} index={index} />
            ))}
          </div>

          <p className="mt-8 text-sm leading-relaxed text-fg-muted">
            Scenario pages:{" "}
            {CORE_REPORT.cases.map((item, index) => (
              <span key={item.id}>
                {index > 0 ? " · " : ""}
                <Link to={benchScenarioPath(item.id)} className={`text-accent hover:underline ${FOCUS_RING}`}>
                  {item.id}
                </Link>
              </span>
            ))}
          </p>
        </section>

        <section className="mx-auto max-w-7xl px-5 pb-16 sm:px-6 sm:pb-24">
          <div className="rounded-2xl border border-border bg-bg-elevated p-7 shadow-[var(--shadow-card)] sm:flex sm:items-center sm:justify-between sm:gap-10 sm:p-10">
            <div>
              <h2 className="text-[clamp(1.8rem,3.4vw,2.6rem)] font-semibold leading-tight tracking-tight text-fg">
                Reproduce it on your own agent.
              </h2>
              <p className="mt-4 max-w-2xl text-base leading-relaxed text-fg-muted sm:text-lg">
                The pack is versioned and public. The same command produces the same artifacts on your side.
              </p>
              <pre className="mt-6 overflow-x-auto rounded-xl border border-border bg-code-bg px-5 py-4 font-mono text-xs leading-relaxed text-fg-body sm:text-sm">
{`bench-cli test \\
  --agent ./my-agent.sh \\
  --suite kubernetes-core@1 \\
  --environment kind \\
  --output ./evidra-results`}
              </pre>
            </div>
            <Link
              to={BENCH_KUBERNETES_CORE_PATH}
              className={`mt-7 inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-lg bg-accent px-5 py-3 text-base font-semibold text-white transition-colors hover:bg-accent-bright hover:text-white sm:mt-0 ${FOCUS_RING}`}
            >
              Read the admission contract
              <ArrowIcon />
            </Link>
          </div>
        </section>
      </main>

      <PublicFooter />
    </div>
  );
}
