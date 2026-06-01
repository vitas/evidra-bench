import { useState } from "react";
import type { RunReview } from "../../../lib/runReview.mts";

export function useRunReview() {
  const [review, setReview] = useState<RunReview | null>(null);
  const [reviewLoading, setReviewLoading] = useState(false);
  const [reviewError, setReviewError] = useState<string | null>(null);
  const [reviewSaving, setReviewSaving] = useState(false);
  const [reviewSaveError, setReviewSaveError] = useState<string | null>(null);
  const [reviewSaved, setReviewSaved] = useState(false);
  const [reviewDrafting, setReviewDrafting] = useState(false);
  const [reviewDraftError, setReviewDraftError] = useState<string | null>(null);
  const [reviewDraftSeed, setReviewDraftSeed] = useState<RunReview | null>(null);
  const [reviewAutoDraftedRunID, setReviewAutoDraftedRunID] = useState<string | null>(null);

  return {
    review,
    setReview,
    reviewLoading,
    setReviewLoading,
    reviewError,
    setReviewError,
    reviewSaving,
    setReviewSaving,
    reviewSaveError,
    setReviewSaveError,
    reviewSaved,
    setReviewSaved,
    reviewDrafting,
    setReviewDrafting,
    reviewDraftError,
    setReviewDraftError,
    reviewDraftSeed,
    setReviewDraftSeed,
    reviewAutoDraftedRunID,
    setReviewAutoDraftedRunID,
  };
}
