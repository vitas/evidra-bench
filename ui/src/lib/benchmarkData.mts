import type {
  ToolServerMatrixArm,
  ToolServerMatrixScenario,
} from "./toolServerMatrixReport.mts";

export function resolveRunsLimit(totalRuns: number): number {
  return Math.max(totalRuns, 1);
}

export interface BenchmarkSafetySummary {
  candidateCells: number;
  safePass: number;
  unsafePass: number;
  fail: number;
  missingEvidence: number;
}

export function scenarioSafetySummary(scenario: ToolServerMatrixScenario): BenchmarkSafetySummary {
  return summarizeScenarioArms(scenario.arms);
}

export function benchmarkSafetySummary(scenarios: ToolServerMatrixScenario[]): BenchmarkSafetySummary {
  return scenarios.reduce<BenchmarkSafetySummary>(
    (summary, scenario) => mergeSummary(summary, scenarioSafetySummary(scenario)),
    emptySummary(),
  );
}

export function armSafetySummary(armID: string, scenarios: ToolServerMatrixScenario[]): BenchmarkSafetySummary {
  return summarizeScenarioArms(
    scenarios
      .flatMap((scenario) => scenario.arms)
      .filter((arm) => arm.arm_id === armID),
    { includeBaseline: true },
  );
}

export function sortBenchmarkArms(
  arms: ToolServerMatrixArm[],
  scenarios: ToolServerMatrixScenario[],
  options: { baselineFirst?: boolean } = {},
): ToolServerMatrixArm[] {
  const baselineFirst = options.baselineFirst ?? true;

  return [...arms].sort((left, right) => {
    if (baselineFirst && left.kind !== right.kind) {
      if (left.kind === "baseline") return -1;
      if (right.kind === "baseline") return 1;
    }

    const leftSummary = armSafetySummary(left.id, scenarios);
    const rightSummary = armSafetySummary(right.id, scenarios);

    return (
      rightSummary.safePass - leftSummary.safePass ||
      leftSummary.unsafePass - rightSummary.unsafePass ||
      leftSummary.fail - rightSummary.fail ||
      right.aggregate.pass_rate - left.aggregate.pass_rate ||
      left.label.localeCompare(right.label)
    );
  });
}

function emptySummary(): BenchmarkSafetySummary {
  return {
    candidateCells: 0,
    safePass: 0,
    unsafePass: 0,
    fail: 0,
    missingEvidence: 0,
  };
}

function mergeSummary(left: BenchmarkSafetySummary, right: BenchmarkSafetySummary): BenchmarkSafetySummary {
  return {
    candidateCells: left.candidateCells + right.candidateCells,
    safePass: left.safePass + right.safePass,
    unsafePass: left.unsafePass + right.unsafePass,
    fail: left.fail + right.fail,
    missingEvidence: left.missingEvidence + right.missingEvidence,
  };
}

function summarizeScenarioArms(
  arms: ToolServerMatrixScenario["arms"],
  options: { includeBaseline?: boolean } = {},
): BenchmarkSafetySummary {
  const includeBaseline = options.includeBaseline ?? false;
  const summary = emptySummary();

  for (const arm of arms) {
    if (!includeBaseline && arm.classification === "baseline") {
      continue;
    }

    summary.candidateCells += 1;

    switch (arm.classification) {
      case "safe_pass":
        summary.safePass += 1;
        break;
      case "unsafe_pass":
        summary.unsafePass += 1;
        break;
      case "fail":
        summary.fail += 1;
        break;
      case "missing_evidence":
        summary.missingEvidence += 1;
        break;
    }
  }

  return summary;
}
