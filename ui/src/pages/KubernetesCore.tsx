import { Link } from "react-router";
import { ArrowIcon, CtaLink, FOCUS_RING, PublicFooter, PublicHeader } from "../components/PublicChrome";
import { SCENARIOS } from "../data/catalog";
import {
  ADMISSION_CONTRACT,
  CONTROL_MATRIX,
  CORE_CASES,
  CORE_PACK_FACTS,
  CORE_PACK_HERO,
  CORE_PACK_LIMIT,
  DROPPED_CASES,
  WHY_FEWER,
} from "../lib/kubernetesCore.mts";
import { BENCH_KUBERNETES_CORE_REPORT_PATH, BENCH_ONLINE_PATH, benchScenarioPath } from "../lib/routes.mts";

const TITLES = new Map(SCENARIOS.map((item) => [item.id, item.title]));

function ControlMatrix() {
  return (
    <div className="overflow-hidden rounded-2xl border border-border bg-bg-elevated shadow-[var(--shadow-card)]">
      <div className="border-b border-border bg-bg-alt px-5 py-4">
        <p className="text-sm font-semibold uppercase tracking-[0.08em] text-accent">The grader is tested too</p>
      </div>

      <table className="w-full border-collapse text-left text-sm">
        <thead>
          <tr className="border-b border-border-subtle text-fg-muted">
            <th scope="col" className="px-5 py-3 font-semibold">
              {CONTROL_MATRIX.columns.control}
            </th>
            <th scope="col" className="px-3 py-3 font-semibold">
              {CONTROL_MATRIX.columns.repair}
            </th>
            <th scope="col" className="px-3 py-3 font-semibold">
              {CONTROL_MATRIX.columns.falseAlarm}
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border-subtle">
          {CONTROL_MATRIX.rows.map((row) => (
            <tr key={row.control}>
              <td className="px-5 py-4 align-top text-fg-body">{row.control}</td>
              <td className="px-3 py-4 align-top font-mono font-semibold text-fg">{row.repair}</td>
              <td className="px-3 py-4 align-top font-mono font-semibold text-fg">{row.falseAlarm}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <p className="border-t border-border-subtle px-5 py-4 text-sm leading-relaxed text-fg-muted">{CONTROL_MATRIX.note}</p>
    </div>
  );
}

export function KubernetesCore() {
  return (
    <div className="min-h-screen bg-bg text-fg">
      <PublicHeader />

      <main>
        <section className="mx-auto grid max-w-7xl gap-12 px-5 pb-14 pt-12 sm:px-6 sm:pb-20 sm:pt-16 lg:grid-cols-[1.02fr_0.98fr] lg:items-center lg:gap-16 lg:py-24">
          <div className="min-w-0">
            <p className="text-sm font-semibold uppercase tracking-[0.08em] text-accent">{CORE_PACK_HERO.eyebrow}</p>
            <h1 className="mt-5 max-w-[16ch] text-[clamp(2.65rem,6vw,4.5rem)] font-semibold leading-[1.02] tracking-[-0.035em] text-fg">
              {CORE_PACK_HERO.title}
            </h1>
            <p className="mt-6 max-w-[58ch] text-base leading-relaxed text-fg-muted sm:text-lg">{CORE_PACK_HERO.body}</p>
            <div className="mt-8 flex flex-col items-stretch gap-3 sm:flex-row sm:items-center">
              {CORE_PACK_HERO.ctas.map((cta) => (
                <CtaLink key={cta.label} cta={cta} />
              ))}
            </div>
          </div>

          <ControlMatrix />
        </section>

        <section aria-label="Pack facts" className="border-y border-border bg-bg-elevated">
          <ul className="mx-auto grid max-w-7xl divide-y divide-border px-5 sm:px-6 md:grid-cols-4 md:divide-x md:divide-y-0">
            {CORE_PACK_FACTS.map((fact) => (
              <li key={fact.label} className="py-6 md:px-7 md:first:pl-0 md:last:pr-0">
                <span className="block font-mono text-2xl font-semibold text-accent">{fact.value}</span>
                <span className="mt-2 block text-sm font-medium leading-relaxed text-fg-body">{fact.label}</span>
              </li>
            ))}
          </ul>
        </section>

        <section className="mx-auto max-w-7xl px-5 py-16 sm:px-6 sm:py-24">
          <p className="text-sm font-semibold uppercase tracking-[0.08em] text-accent">Why fewer is harder</p>
          <h2 className="mt-3 max-w-3xl text-[clamp(2rem,4vw,3rem)] font-semibold leading-tight tracking-tight text-fg">
            A case that cannot fail an unsafe agent is not a test.
          </h2>

          <div className="mt-10 grid gap-6 md:grid-cols-3">
            {WHY_FEWER.map((item) => (
              <article key={item.title} className="rounded-2xl border border-border bg-bg-elevated p-6 shadow-[var(--shadow-card)] sm:p-7">
                <h3 className="text-lg font-semibold leading-snug text-fg">{item.title}</h3>
                <p className="mt-3 text-base leading-relaxed text-fg-muted">{item.body}</p>
              </article>
            ))}
          </div>
        </section>

        <section className="border-y border-border bg-bg-elevated">
          <div className="mx-auto grid max-w-7xl gap-10 px-5 py-16 sm:px-6 sm:py-24 lg:grid-cols-[0.8fr_1.2fr] lg:gap-16">
            <div>
              <p className="text-sm font-semibold uppercase tracking-[0.08em] text-accent">Admission contract</p>
              <h2 className="mt-3 text-[clamp(2rem,4vw,3rem)] font-semibold leading-tight tracking-tight text-fg">
                What a case must prove before it counts.
              </h2>
              <p className="mt-5 text-base leading-relaxed text-fg-muted sm:text-lg">
                A candidate that fails any line is replaced, never waived.
              </p>
            </div>

            <ul className="grid gap-x-8 gap-y-3 sm:grid-cols-2">
              {ADMISSION_CONTRACT.map((item) => (
                <li key={item} className="flex gap-3 text-base leading-relaxed text-fg-body">
                  <span aria-hidden="true" className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-accent" />
                  {item}
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section className="mx-auto max-w-7xl px-5 py-16 sm:px-6 sm:py-24">
          <p className="text-sm font-semibold uppercase tracking-[0.08em] text-accent">What we dropped</p>
          <h2 className="mt-3 max-w-3xl text-[clamp(2rem,4vw,3rem)] font-semibold leading-tight tracking-tight text-fg">
            Three of the scenarios that lost their place.
          </h2>

          <ul className="mt-10 divide-y divide-border-subtle overflow-hidden rounded-2xl border border-border bg-bg-elevated">
            {DROPPED_CASES.map((item) => (
              <li key={item.id} className="grid gap-2 px-5 py-5 md:grid-cols-[0.8fr_1.2fr] md:items-baseline md:gap-8 md:px-7">
                <span className="text-base font-semibold text-fg">{item.title}</span>
                <span className="text-base leading-relaxed text-fg-muted">{item.reason}</span>
              </li>
            ))}
          </ul>
        </section>

        <section className="border-y border-border bg-bg-elevated">
          <div className="mx-auto max-w-7xl px-5 py-16 sm:px-6 sm:py-24">
            <p className="text-sm font-semibold uppercase tracking-[0.08em] text-accent">The pack</p>
            <h2 className="mt-3 max-w-3xl text-[clamp(2rem,4vw,3rem)] font-semibold leading-tight tracking-tight text-fg">
              Twelve cases, twelve ways to be wrong.
            </h2>

            <ol className="mt-10 grid gap-x-8 gap-y-4 md:grid-cols-2">
              {CORE_CASES.map((item) => (
                <li key={item.id} className="flex gap-4 border-b border-border-subtle pb-4">
                  <span className="mt-0.5 shrink-0 rounded-md bg-accent-subtle px-2 py-1 font-mono text-xs font-semibold text-accent">
                    {item.level}
                  </span>
                  <div className="min-w-0">
                    <Link to={benchScenarioPath(item.id)} className={`font-semibold text-fg hover:text-accent ${FOCUS_RING}`}>
                      {TITLES.get(item.id) ?? item.id}
                    </Link>
                    <p className="mt-1 text-sm leading-relaxed text-fg-muted">{item.catches}</p>
                  </div>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section className="mx-auto max-w-7xl px-5 py-16 sm:px-6 sm:py-24">
          <div className="rounded-2xl border border-border bg-bg-elevated p-7 shadow-[var(--shadow-card)] sm:p-10">
            <h2 className="text-[clamp(1.9rem,3.6vw,2.8rem)] font-semibold leading-tight tracking-tight text-fg">
              Run your agent against the pack.
            </h2>
            <p className="mt-4 max-w-2xl text-base leading-relaxed text-fg-muted sm:text-lg">
              Bring any agent that can drive kubectl. The runner builds a disposable cluster and writes inspectable artifacts
              for every verdict.
            </p>
            <pre className="mt-6 overflow-x-auto rounded-xl border border-border bg-code-bg px-5 py-4 font-mono text-xs leading-relaxed text-fg-body sm:text-sm">
{`bench-cli test \\
  --agent ./my-agent.sh \\
  --suite kubernetes-core@1`}
            </pre>
            <div className="mt-7 flex flex-col gap-3 sm:flex-row sm:items-center">
              <Link
                to={BENCH_ONLINE_PATH}
                className={`inline-flex min-h-11 items-center justify-center gap-2 rounded-lg bg-accent px-5 py-3 text-sm font-semibold text-white transition-colors hover:bg-accent-bright hover:text-white ${FOCUS_RING}`}
              >
                Open public Bench
                <ArrowIcon />
              </Link>
              <Link
                to={BENCH_KUBERNETES_CORE_REPORT_PATH}
                className={`inline-flex min-h-11 items-center px-2 text-sm font-semibold text-fg-muted hover:text-accent ${FOCUS_RING}`}
              >
                Read the first public run
              </Link>
            </div>
            <p className="mt-6 text-sm leading-relaxed text-fg-muted">{CORE_PACK_LIMIT}</p>
          </div>
        </section>
      </main>

      <PublicFooter />
    </div>
  );
}
