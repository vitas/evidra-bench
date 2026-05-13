import type { PuzzleMetadata } from "./yaml-generator";

interface PublishButtonProps {
  metadata: PuzzleMetadata;
}

export function PublishButton({ metadata: _metadata }: PublishButtonProps) {
  return (
    <button
      disabled
      className="inline-flex items-center gap-1 text-[0.72rem] font-medium text-fg-muted/60"
      title="Publishing requires a server-side authenticated workflow"
    >
      <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
          d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
      </svg>
      Publish disabled
    </button>
  );
}
