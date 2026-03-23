export interface ScenarioCategoryRecord {
  id: string;
  category: string;
}

export function categoriesFromScenarios(scenarios: ScenarioCategoryRecord[]): string[] {
  return [...new Set(scenarios.map((scenario) => scenario.category).filter(Boolean))].sort();
}

export function scenarioIdsForCategory(scenarios: ScenarioCategoryRecord[], category: string): string[] {
  if (!category) return [];
  return scenarios
    .filter((scenario) => scenario.category === category)
    .map((scenario) => scenario.id)
    .sort();
}
