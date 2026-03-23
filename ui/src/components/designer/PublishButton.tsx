import { useState, useCallback } from "react";
import type { PuzzleMetadata } from "./yaml-generator";

const API_BASE = import.meta.env.VITE_EVIDRA_API_URL || "https://api.evidra.cc";
const API_KEY = import.meta.env.VITE_EVIDRA_API_KEY || "";

interface PublishButtonProps {
  metadata: PuzzleMetadata;
}

export function PublishButton({ metadata }: PublishButtonProps) {
  const [status, setStatus] = useState<"idle" | "publishing" | "done" | "error">("idle");
  const [message, setMessage] = useState("");

  const handlePublish = useCallback(async () => {
    if (!metadata.name || !metadata.title) {
      setStatus("error");
      setMessage("Name and title required");
      setTimeout(() => setStatus("idle"), 3000);
      return;
    }

    setStatus("publishing");
    try {
      const payload = {
        scenarios: [{
          id: metadata.name,
          title: metadata.title,
          description: metadata.description || "",
          category: metadata.category || "kubernetes",
          tags: [],
          chaos: false,
          evidra: false,
        }],
      };

      const res = await fetch(`${API_BASE}/v1/bench/scenarios/sync`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(API_KEY ? { Authorization: `Bearer ${API_KEY}` } : {}),
        },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(body.error || res.statusText);
      }

      setStatus("done");
      setMessage("Published!");
      setTimeout(() => setStatus("idle"), 3000);
    } catch (err) {
      setStatus("error");
      setMessage(err instanceof Error ? err.message : "Failed");
      setTimeout(() => setStatus("idle"), 4000);
    }
  }, [metadata]);

  return (
    <button
      onClick={handlePublish}
      disabled={status === "publishing"}
      className={`inline-flex items-center gap-1 text-[0.72rem] font-medium transition-colors ${
        status === "done"
          ? "text-green-400"
          : status === "error"
            ? "text-red-400"
            : "text-fg-muted hover:text-accent"
      }`}
      title="Publish scenario metadata to evidra API"
    >
      <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
          d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
      </svg>
      {status === "publishing" ? "Publishing..." : status === "done" ? message : status === "error" ? message : "Publish"}
    </button>
  );
}
