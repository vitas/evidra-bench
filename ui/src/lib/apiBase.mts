export function buildBenchApiURL(apiBase: string | undefined, path: string): string {
  const base = (apiBase ?? "").replace(/\/+$/, "");
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  return `${base}${normalizedPath}`;
}
