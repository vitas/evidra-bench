export function formatDuration(seconds: number, options: { compact?: boolean } = {}): string {
  if (!Number.isFinite(seconds)) return "0.0s";
  if (options.compact || seconds < 60) return `${seconds.toFixed(1)}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = Math.round(seconds % 60);
  return `${minutes}m ${remainingSeconds}s`;
}

export function formatCurrency(usd: number, options: { precision?: number; smallPrecision?: number } = {}): string {
  if (!Number.isFinite(usd) || usd === 0) return "$0.00";
  const precision = options.precision ?? 2;
  const smallPrecision = options.smallPrecision ?? 3;
  if (Math.abs(usd) < 0.01) return `$${usd.toFixed(smallPrecision)}`;
  return `$${usd.toFixed(precision)}`;
}

export function formatCompactTokens(value: number): string {
  if (!Number.isFinite(value)) return "0";
  if (Math.abs(value) >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (Math.abs(value) >= 1_000) return `${(value / 1_000).toFixed(1)}k`;
  return String(value);
}

export function formatIntegerTokens(value: number): string {
  if (!Number.isFinite(value)) return "0";
  return Math.round(value).toLocaleString("en-US");
}

export function formatPercent(value: number, digits = 1): string {
  if (!Number.isFinite(value)) return "0.0%";
  return `${value.toFixed(digits)}%`;
}

export function formatSignedNumber(value: number, digits = 1): string {
  if (!Number.isFinite(value)) return "0.0";
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(digits)}`;
}

export function formatSignedPercent(value: number, digits = 1): string {
  if (!Number.isFinite(value)) return "0.0pp";
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(digits)}pp`;
}

export function formatDateTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  const day = String(date.getDate()).padStart(2, "0");
  const month = date.toLocaleString("en-US", { month: "short" });
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${day} ${month} ${hours}:${minutes}`;
}

export function formatDateShort(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return `${date.getDate()} ${date.toLocaleString("en-US", { month: "short" })}`;
}

export function formatDateLabel(dateKey: string, options: { year?: boolean } = {}): string {
  if (!dateKey || dateKey === "unknown") return dateKey;
  const date = new Date(`${dateKey}T12:00:00`);
  if (Number.isNaN(date.getTime())) return dateKey;
  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    ...(options.year ? { year: "numeric" as const } : {}),
  });
}
