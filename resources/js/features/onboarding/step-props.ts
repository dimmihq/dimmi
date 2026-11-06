/*
 * What every setup step is handed: the workspace being set up, its label
 * among the steps, and the ways on and back. Each step saves its own
 * work before it calls onNext.
 */
export interface StepProps {
  workspace: string;
  label: string;
  onNext: () => void;
  onBack: () => void;
}
