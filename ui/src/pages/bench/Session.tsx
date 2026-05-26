import { useEffect, useState, type FormEvent } from "react";
import { usePageTitle } from "../../hooks/usePageTitle";
import { benchSessionStatus, createBenchSession, deleteBenchSession, type BenchSessionStatus } from "../../lib/benchSession.mts";
import { BENCH_SESSION_PATH } from "../../lib/routes.mts";

const API_BASE = import.meta.env.VITE_BENCH_API_URL || "";

export function Session() {
  usePageTitle("Session", { canonicalPath: BENCH_SESSION_PATH });
  const [status, setStatus] = useState<BenchSessionStatus>({ authenticated: false });
  const [apiKey, setAPIKey] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    benchSessionStatus({ apiBase: API_BASE })
      .then((next) => {
        if (!cancelled) setStatus(next);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Failed to load session");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function signIn(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const next = await createBenchSession(apiKey, { apiBase: API_BASE });
      setStatus(next);
      setAPIKey("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to sign in");
    } finally {
      setSaving(false);
    }
  }

  async function signOut() {
    setSaving(true);
    setError(null);
    try {
      await deleteBenchSession({ apiBase: API_BASE });
      setStatus({ authenticated: false });
      setAPIKey("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to sign out");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="max-w-xl">
      <div className="mb-5">
        <h1 className="text-[1.35rem] font-bold text-fg tracking-tight">Session</h1>
        <p className="mt-0.5 text-[0.82rem] text-fg-muted">Deployment access</p>
      </div>

      <div className="rounded-lg border border-border bg-bg-elevated p-4">
        {loading ? (
          <p className="py-8 text-center text-[0.84rem] text-fg-muted">Loading session...</p>
        ) : status.authenticated ? (
          <div className="space-y-4">
            <div className="rounded-md border border-border-subtle bg-bg-alt/70 px-3 py-2">
              <div className="text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">Tenant</div>
              <div className="mt-0.5 font-mono text-[0.84rem] text-fg">{status.tenant_id || "authenticated"}</div>
            </div>
            <button
              type="button"
              onClick={signOut}
              disabled={saving}
              className="rounded-md border border-border bg-bg-elevated px-3 py-1.5 text-[0.78rem] font-semibold text-fg-muted transition-colors hover:border-accent hover:text-fg disabled:cursor-default disabled:opacity-50"
            >
              {saving ? "Signing out..." : "Sign Out"}
            </button>
          </div>
        ) : (
          <form onSubmit={signIn} className="space-y-4">
            <label>
              <span className="mb-1.5 block text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">
                API Key
              </span>
              <input
                value={apiKey}
                onChange={(e) => setAPIKey(e.target.value)}
                type="password"
                autoComplete="current-password"
                required
                className="w-full rounded-md border border-border bg-bg-elevated px-3 py-2 font-mono text-[0.84rem] text-fg focus:border-accent focus:outline-none"
              />
            </label>
            <button
              type="submit"
              disabled={saving}
              className="rounded-md border border-accent bg-accent px-3 py-1.5 text-[0.78rem] font-semibold text-white transition-opacity disabled:cursor-default disabled:opacity-50"
            >
              {saving ? "Signing in..." : "Sign In"}
            </button>
          </form>
        )}

        {error && (
          <div className="mt-4 rounded-md bg-[var(--color-danger-badge-bg)] px-3 py-2 text-[0.8rem] text-[var(--color-danger-badge-fg)]">
            {error}
          </div>
        )}
      </div>
    </div>
  );
}
