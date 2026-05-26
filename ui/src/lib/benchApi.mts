import { buildBenchApiURL } from "./apiBase.mts";

export interface BenchApiRequestConfig {
  apiBase?: string;
  fetchImpl?: typeof fetch;
}

export async function fetchBenchApi(
  path: string,
  options: RequestInit = {},
  config: BenchApiRequestConfig = {},
): Promise<Response> {
  const method = (options.method || "GET").toUpperCase();
  const headers = new Headers(options.headers);
  headers.delete("authorization");

  if (options.body != null && !headers.has("Content-Type") && !isFormDataBody(options.body)) {
    headers.set("Content-Type", "application/json");
  }

  const fetchImpl = config.fetchImpl ?? fetch;
  return fetchImpl(buildBenchApiURL(config.apiBase, path), {
    ...options,
    method,
    headers,
    credentials: options.credentials ?? "include",
  });
}

export async function requestBenchApi<T = unknown>(
  path: string,
  options: RequestInit = {},
  config: BenchApiRequestConfig = {},
): Promise<T> {
  const res = await fetchBenchApi(path, options, config);

  if (!res.ok) {
    throw new Error(await responseErrorMessage(res));
  }

  if (res.status === 204) {
    return undefined as T;
  }

  const contentType = res.headers.get("Content-Type") ?? "";
  if (contentType.includes("application/json")) {
    return res.json() as Promise<T>;
  }
  return res.text() as Promise<T>;
}

function isFormDataBody(body: BodyInit): boolean {
  return typeof FormData !== "undefined" && body instanceof FormData;
}

async function responseErrorMessage(res: Response): Promise<string> {
  try {
    const body = await res.clone().json();
    if (body && typeof body === "object") {
      const errorBody = body as { error?: unknown; message?: unknown };
      const message = errorBody.error ?? errorBody.message;
      if (typeof message === "string" && message.trim() !== "") {
        return message;
      }
    }
  } catch {}

  try {
    const text = await res.text();
    if (text.trim() !== "") {
      return text;
    }
  } catch {}

  return res.statusText;
}
