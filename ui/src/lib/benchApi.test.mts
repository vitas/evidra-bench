import assert from "node:assert/strict";
import test from "node:test";

import { fetchBenchApi, requestBenchApi } from "./benchApi.mts";

const browserAuthHeader = "authorization";

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
  assert.equal(calls[0].headers.has(browserAuthHeader), false);
  assert.equal(calls[0].credentials, "include");
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
  assert.equal(calls[0].headers.has(browserAuthHeader), false);
  assert.equal(calls[0].credentials, "include");
});

test("requestBenchApi posts review drafts through the same browser session path", async () => {
  const calls: Request[] = [];
  const fetchImpl = async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push(new Request(input, init));
    return Response.json({ version: "run_review.v1", verdict: "valid_failure" });
  };

  const result = await requestBenchApi<{ verdict: string }>("/v1/bench/runs/run-1/review-draft", {
    method: "POST",
  }, {
    apiBase: "https://api.evidra.cc",
    fetchImpl,
  });

  assert.deepEqual(result, { version: "run_review.v1", verdict: "valid_failure" });
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "POST");
  assert.equal(calls[0].headers.has(browserAuthHeader), false);
  assert.equal(calls[0].credentials, "include");
});

test("requestBenchApi strips caller auth headers from browser requests", async () => {
  const calls: Request[] = [];
  const fetchImpl = async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push(new Request(input, init));
    return Response.json({ id: "job-1" });
  };

  const result = await requestBenchApi<{ id: string }>("/v1/bench/trigger", {
    method: "POST",
    headers: { [browserAuthHeader]: "Bearer fixture-token" },
    body: JSON.stringify({ model: "sonnet" }),
  }, {
    apiBase: "https://api.evidra.cc",
    fetchImpl,
  });

  assert.deepEqual(result, { id: "job-1" });
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "POST");
  assert.equal(calls[0].headers.has(browserAuthHeader), false);
  assert.equal(calls[0].headers.get("Content-Type"), "application/json");
  assert.equal(calls[0].credentials, "include");
});

test("fetchBenchApi centralizes artifact requests without parsing the response", async () => {
  const calls: Request[] = [];
  const fetchImpl = async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push(new Request(input, init));
    return new Response("transcript text", {
      headers: { "Content-Type": "text/plain" },
    });
  };

  const res = await fetchBenchApi("/v1/bench/runs/run-1/transcript", {}, {
    apiBase: "https://api.evidra.cc",
    fetchImpl,
  });

  assert.equal(await res.text(), "transcript text");
  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, "https://api.evidra.cc/v1/bench/runs/run-1/transcript");
  assert.equal(calls[0].headers.has(browserAuthHeader), false);
  assert.equal(calls[0].credentials, "include");
});
