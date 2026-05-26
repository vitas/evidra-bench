import assert from "node:assert/strict";
import test from "node:test";

import { benchSessionStatus, createBenchSession, deleteBenchSession } from "./benchSession.mts";

test("bench session helpers call the backend session endpoints", async () => {
  const calls: Request[] = [];
  const fetchImpl = async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push(new Request(input, init));
    return Response.json({ authenticated: true, tenant_id: "tenant-browser" });
  };

  const status = await benchSessionStatus({ apiBase: "https://api.evidra.cc", fetchImpl });
  const created = await createBenchSession("secret-key", { apiBase: "https://api.evidra.cc", fetchImpl });

  assert.deepEqual(status, { authenticated: true, tenant_id: "tenant-browser" });
  assert.deepEqual(created, { authenticated: true, tenant_id: "tenant-browser" });
  assert.equal(calls[0].url, "https://api.evidra.cc/v1/bench/session");
  assert.equal(calls[0].method, "GET");
  assert.equal(calls[0].credentials, "include");
  assert.equal(calls[1].method, "POST");
  assert.equal(calls[1].headers.has("authorization"), false);
  assert.equal(calls[1].credentials, "include");
  assert.equal(await calls[1].text(), JSON.stringify({ api_key: "secret-key" }));
});

test("deleteBenchSession clears the backend session", async () => {
  const calls: Request[] = [];
  const fetchImpl = async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push(new Request(input, init));
    return new Response(null, { status: 204 });
  };

  await deleteBenchSession({ apiBase: "https://api.evidra.cc", fetchImpl });

  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, "https://api.evidra.cc/v1/bench/session");
  assert.equal(calls[0].method, "DELETE");
  assert.equal(calls[0].credentials, "include");
});
