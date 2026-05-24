import assert from "node:assert/strict";
import test from "node:test";

import { requestBenchApi } from "./benchApi.mts";

test("requestBenchApi allows public GET requests without auth", async () => {
  const calls: Request[] = [];
  const fetchImpl = async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push(new Request(input, init));
    return Response.json({ ok: true });
  };

  const result = await requestBenchApi<{ ok: boolean }>("/v1/bench/runs", {}, {
    apiBase: "https://api.evidra.cc",
    fetchImpl,
  });

  assert.deepEqual(result, { ok: true });
  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, "https://api.evidra.cc/v1/bench/runs");
  assert.equal(calls[0].headers.has("Authorization"), false);
});

test("requestBenchApi sends unauthenticated writes to the backend", async () => {
  const calls: Request[] = [];
  const fetchImpl = async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push(new Request(input, init));
    return Response.json({ ok: true });
  };

  const result = await requestBenchApi<{ ok: boolean }>("/v1/bench/runs/run-1/review", {
    method: "PUT",
    body: JSON.stringify({ version: "run_review.v1" }),
  }, {
    apiBase: "https://api.evidra.cc",
    fetchImpl,
  });

  assert.deepEqual(result, { ok: true });
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "PUT");
  assert.equal(calls[0].headers.has("Authorization"), false);
});

test("requestBenchApi sends bearer token for authenticated writes", async () => {
  const calls: Request[] = [];
  const fetchImpl = async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push(new Request(input, init));
    return Response.json({ id: "job-1" });
  };

  const result = await requestBenchApi<{ id: string }>("/v1/bench/trigger", {
    method: "POST",
    body: JSON.stringify({ model: "sonnet" }),
  }, {
    apiBase: "https://api.evidra.cc",
    authToken: "secret-token",
    fetchImpl,
  });

  assert.deepEqual(result, { id: "job-1" });
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "POST");
  assert.equal(calls[0].headers.get("Authorization"), "Bearer secret-token");
  assert.equal(calls[0].headers.get("Content-Type"), "application/json");
});
