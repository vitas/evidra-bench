import { useState, useEffect, useCallback, useRef } from "react";
import type { Node, Edge } from "@xyflow/react";

interface TourStep {
  id: string;
  title: string;
  description: string;
  target: string | null;
  action: string;
  placement?: "bottom" | "right" | "left" | "top";
}

const TOUR_STEPS: TourStep[] = [
  {
    id: "welcome",
    title: "Welcome to the Puzzle Designer!",
    description:
      "Build infrastructure puzzles visually. Let's create your first one in 60 seconds.",
    target: null,
    action: "click-next",
  },
  {
    id: "palette",
    title: "These are your building blocks",
    description:
      "Drag them onto the canvas. Stack deploys the app, Break breaks it, Verify checks the fix.",
    target: "palette",
    action: "click-next",
    placement: "right",
  },
  {
    id: "add-stack",
    title: "Start with a Stack",
    description:
      "Drag the Stack block onto the canvas. This is the application your agent will fix.",
    target: "palette-stack",
    action: "node-added-stack",
    placement: "right",
  },
  {
    id: "add-break",
    title: "Now break something!",
    description: "Drag a Break block onto the canvas. This defines what goes wrong.",
    target: "palette-break",
    action: "node-added-break",
    placement: "right",
  },
  {
    id: "connect",
    title: "Connect them",
    description:
      "Drag from the blue dot on Stack to the red dot on Break to define the puzzle flow.",
    target: "canvas",
    action: "edge-added",
    placement: "bottom",
  },
  {
    id: "configure-break",
    title: "Choose what breaks",
    description:
      "Click a Break node and pick a failure type in the config panel. Try 'Wrong image tag'!",
    target: "config-panel",
    action: "click-next",
    placement: "left",
  },
  {
    id: "add-verify",
    title: "Add a verification check",
    description:
      "Drag a Verify block and connect it to Break. This checks if the agent fixed it.",
    target: "palette-verify",
    action: "node-added-verify",
    placement: "right",
  },
  {
    id: "export",
    title: "Export your puzzle!",
    description:
      "Click 'Export YAML' to see the generated scenario. Copy it and run with infra-bench.",
    target: "export-button",
    action: "click-next",
    placement: "top",
  },
];

const LS_KEY = "designer-tour-completed";

interface GuidedTourProps {
  nodes: Node[];
  edges: Edge[];
  active: boolean;
  onComplete: () => void;
}

interface TooltipPos {
  top: number;
  left: number;
}

interface SpotlightRect {
  top: number;
  left: number;
  width: number;
  height: number;
}

export function useTourState() {
  const [active, setActive] = useState(() => {
    if (typeof window === "undefined") return false;
    return localStorage.getItem(LS_KEY) !== "true";
  });

  const complete = useCallback(() => {
    setActive(false);
    localStorage.setItem(LS_KEY, "true");
  }, []);

  const restart = useCallback(() => {
    setActive(true);
    localStorage.removeItem(LS_KEY);
  }, []);

  return { active, complete, restart };
}

export function GuidedTour({ nodes, edges, active, onComplete }: GuidedTourProps) {
  const [step, setStep] = useState(0);
  const [tooltipPos, setTooltipPos] = useState<TooltipPos>({ top: 0, left: 0 });
  const [spotlight, setSpotlight] = useState<SpotlightRect | null>(null);
  const [visible, setVisible] = useState(false);
  const tooltipRef = useRef<HTMLDivElement>(null);
  const prevNodesLen = useRef(nodes.length);
  const prevEdgesLen = useRef(edges.length);

  const currentStep = TOUR_STEPS[step];

  // Reset step counter when tour starts
  useEffect(() => {
    if (active) {
      setStep(0);
      setVisible(false);
      // Small delay so the DOM is ready
      const t = setTimeout(() => setVisible(true), 100);
      return () => clearTimeout(t);
    }
    setVisible(false);
  }, [active]);

  // Auto-advance for pre-populated canvas
  useEffect(() => {
    if (!active || !visible) return;
    const s = TOUR_STEPS[step];
    if (!s) return;

    if (s.action === "node-added-stack" && nodes.some((n) => n.type === "stack")) {
      setStep((prev) => prev + 1);
      return;
    }
    if (s.action === "node-added-break" && nodes.some((n) => n.type === "break")) {
      setStep((prev) => prev + 1);
      return;
    }
    if (s.action === "node-added-verify" && nodes.some((n) => n.type === "verify")) {
      setStep((prev) => prev + 1);
      return;
    }
    if (s.action === "edge-added" && edges.length > 0) {
      setStep((prev) => prev + 1);
      return;
    }
  }, [active, visible, step, nodes, edges]);

  // Detect newly added nodes/edges for non-prepopulated flows
  useEffect(() => {
    if (!active || !visible) return;
    const s = TOUR_STEPS[step];
    if (!s) return;

    const nodesAdded = nodes.length > prevNodesLen.current;
    const edgesAdded = edges.length > prevEdgesLen.current;
    prevNodesLen.current = nodes.length;
    prevEdgesLen.current = edges.length;

    if (s.action === "node-added-stack" && nodesAdded && nodes.some((n) => n.type === "stack")) {
      setStep((prev) => prev + 1);
    }
    if (s.action === "node-added-break" && nodesAdded && nodes.some((n) => n.type === "break")) {
      setStep((prev) => prev + 1);
    }
    if (s.action === "node-added-verify" && nodesAdded && nodes.some((n) => n.type === "verify")) {
      setStep((prev) => prev + 1);
    }
    if (s.action === "edge-added" && edgesAdded) {
      setStep((prev) => prev + 1);
    }
  }, [active, visible, step, nodes.length, edges.length]);

  // Position tooltip relative to target element
  useEffect(() => {
    if (!active || !visible || !currentStep) return;

    function positionTooltip() {
      if (!currentStep.target) {
        // Centered modal
        setSpotlight(null);
        setTooltipPos({
          top: window.innerHeight / 2 - 100,
          left: window.innerWidth / 2 - 180,
        });
        return;
      }

      const el = document.querySelector(`[data-tour="${currentStep.target}"]`);
      if (!el) {
        setSpotlight(null);
        setTooltipPos({
          top: window.innerHeight / 2 - 100,
          left: window.innerWidth / 2 - 180,
        });
        return;
      }

      const rect = el.getBoundingClientRect();
      const pad = 8;

      setSpotlight({
        top: rect.top - pad,
        left: rect.left - pad,
        width: rect.width + pad * 2,
        height: rect.height + pad * 2,
      });

      const tooltipW = 320;
      const tooltipH = 180;
      const placement = currentStep.placement || "bottom";

      let top = 0;
      let left = 0;

      switch (placement) {
        case "right":
          top = rect.top + rect.height / 2 - tooltipH / 2;
          left = rect.right + pad + 12;
          break;
        case "left":
          top = rect.top + rect.height / 2 - tooltipH / 2;
          left = rect.left - pad - tooltipW - 12;
          break;
        case "top":
          top = rect.top - pad - tooltipH - 12;
          left = rect.left + rect.width / 2 - tooltipW / 2;
          break;
        case "bottom":
        default:
          top = rect.bottom + pad + 12;
          left = rect.left + rect.width / 2 - tooltipW / 2;
          break;
      }

      // Clamp to viewport
      top = Math.max(12, Math.min(top, window.innerHeight - tooltipH - 12));
      left = Math.max(12, Math.min(left, window.innerWidth - tooltipW - 12));

      setTooltipPos({ top, left });
    }

    positionTooltip();
    window.addEventListener("resize", positionTooltip);
    // Re-position after a short delay for layout shifts
    const timer = setTimeout(positionTooltip, 300);
    return () => {
      window.removeEventListener("resize", positionTooltip);
      clearTimeout(timer);
    };
  }, [active, visible, step, currentStep]);

  const handleNext = useCallback(() => {
    if (step >= TOUR_STEPS.length - 1) {
      onComplete();
    } else {
      setStep((prev) => prev + 1);
    }
  }, [step, onComplete]);

  const handleSkip = useCallback(() => {
    onComplete();
  }, [onComplete]);

  if (!active || !visible || !currentStep) return null;

  const isLastStep = step === TOUR_STEPS.length - 1;
  const showNext = currentStep.action === "click-next";

  return (
    <div className="fixed inset-0 z-[9999]" style={{ pointerEvents: "none" }}>
      {/* Overlay — built from 4 edge rectangles so the spotlight hole is mouse-transparent */}
      {spotlight ? (
        <>
          {/* Top bar */}
          <div
            className="fixed bg-black/55 transition-all duration-300"
            style={{
              pointerEvents: "auto",
              top: 0,
              left: 0,
              right: 0,
              height: Math.max(0, spotlight.top),
            }}
          />
          {/* Bottom bar */}
          <div
            className="fixed bg-black/55 transition-all duration-300"
            style={{
              pointerEvents: "auto",
              top: spotlight.top + spotlight.height,
              left: 0,
              right: 0,
              bottom: 0,
            }}
          />
          {/* Left bar */}
          <div
            className="fixed bg-black/55 transition-all duration-300"
            style={{
              pointerEvents: "auto",
              top: spotlight.top,
              left: 0,
              width: Math.max(0, spotlight.left),
              height: spotlight.height,
            }}
          />
          {/* Right bar */}
          <div
            className="fixed bg-black/55 transition-all duration-300"
            style={{
              pointerEvents: "auto",
              top: spotlight.top,
              left: spotlight.left + spotlight.width,
              right: 0,
              height: spotlight.height,
            }}
          />
          {/* Glow ring around spotlight */}
          <div
            className="fixed rounded-lg ring-2 ring-[var(--color-accent)] ring-offset-2 ring-offset-transparent transition-all duration-300"
            style={{
              pointerEvents: "none",
              top: spotlight.top,
              left: spotlight.left,
              width: spotlight.width,
              height: spotlight.height,
            }}
          />
        </>
      ) : (
        <div
          className="fixed inset-0 bg-black/55"
          style={{ pointerEvents: "auto" }}
        />
      )}

      {/* Tooltip card */}
      <div
        ref={tooltipRef}
        className="fixed w-[320px] rounded-xl border border-[var(--color-accent)]/40 bg-[var(--color-bg-elevated)] shadow-2xl transition-all duration-300 animate-tour-in"
        style={{
          pointerEvents: "auto",
          top: tooltipPos.top,
          left: tooltipPos.left,
          zIndex: 10000,
        }}
      >
        <div className="p-4">
          {/* Step counter */}
          <div className="text-[0.68rem] text-[var(--color-fg-muted)] mb-1.5">
            Step {step + 1} of {TOUR_STEPS.length}
          </div>

          {/* Title */}
          <h4 className="text-[0.88rem] font-bold text-[var(--color-fg)] mb-1.5 leading-snug">
            {currentStep.title}
          </h4>

          {/* Description */}
          <p className="text-[0.78rem] text-[var(--color-fg-body)] leading-relaxed mb-4">
            {currentStep.description}
          </p>

          {/* Buttons */}
          <div className="flex items-center justify-between">
            <button
              onClick={handleSkip}
              className="text-[0.75rem] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
            >
              Skip tour
            </button>
            {showNext && (
              <button
                onClick={handleNext}
                className="px-4 py-1.5 text-[0.78rem] font-semibold rounded-md bg-[var(--color-accent)] text-white hover:brightness-110 transition-all"
              >
                {isLastStep ? "Done" : "Next"}
              </button>
            )}
            {!showNext && (
              <span className="text-[0.7rem] text-[var(--color-fg-muted)] italic">
                Complete the action to continue
              </span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
