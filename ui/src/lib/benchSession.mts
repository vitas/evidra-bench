import type { BenchApiRequestConfig } from "./benchApi.mts";
import { requestBenchApi } from "./benchApi.mts";

export interface BenchSessionStatus {
  authenticated: boolean;
  tenant_id?: string;
}

export function benchSessionStatus(config: BenchApiRequestConfig = {}): Promise<BenchSessionStatus> {
  return requestBenchApi<BenchSessionStatus>("/v1/bench/session", {}, config);
}

export function createBenchSession(apiKey: string, config: BenchApiRequestConfig = {}): Promise<BenchSessionStatus> {
  return requestBenchApi<BenchSessionStatus>("/v1/bench/session", {
    method: "POST",
    body: JSON.stringify({ api_key: apiKey }),
  }, config);
}

export function deleteBenchSession(config: BenchApiRequestConfig = {}): Promise<void> {
  return requestBenchApi<void>("/v1/bench/session", {
    method: "DELETE",
  }, config);
}
