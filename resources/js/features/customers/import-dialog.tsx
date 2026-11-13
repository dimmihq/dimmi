/*
 * Importing people or companies from a CSV, in four steps that slide one
 * into the next: upload the file, map its columns to fields, choose what
 * happens to records already here while a dry run counts the outcome,
 * then run it in the background and follow its progress. Closing the
 * dialog while it runs leaves the import running. From the upload, past
 * imports list with their problem rows and an undo. Only members who
 * may manage imports see the button.
 */
import { RiHistoryLine, RiUploadLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { FileUpload } from "@/components/base/file-upload/file-upload";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Resize, Swap } from "@/features/inbox/motion";
import { attributesQuery } from "@/lib/api/attributes";
import {
  downloadImportErrors,
  type Import,
  type ImportKind,
  type ImportMode,
  type ImportPlan,
  importQuery,
  previewImport,
  startImport,
  uploadImport,
} from "@/lib/api/imports";
import { callerQuery } from "@/lib/api/workspace";
import { formatNumber } from "@/lib/i18n/format";
import { ImportHistory } from "./import-history";
import { fieldChoices, initialFields, mappingProblem, planFrom } from "./import-plan";
import { MapStep, OptionsStep, RunStep } from "./import-steps";
import { useWorkspace } from "./names";

const maxBytes = 10 * 1024 * 1024;
const pollEvery = 1000;
const steps = ["upload", "map", "options", "run"] as const;

type Step = (typeof steps)[number];

const copy: Record<ImportKind, { title: string; noun: string; upload: string }> = {
  contacts: {
    title: "Import people",
    noun: "people",
    upload: "A CSV with a header row and an email or external ID column. 50,000 rows at most.",
  },
  companies: {
    title: "Import companies",
    noun: "companies",
    upload: "A CSV with a header row and a domain or external ID column. 50,000 rows at most.",
  },
};

const descriptions: Record<Exclude<Step, "upload">, string> = {
  map: "Pick the field each column fills. Columns you don't import are ignored.",
  options: "Choose what happens to records already here. Nothing changes until you import.",
  run: "The import runs in the background. You can close this window.",
};

function save(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  URL.revokeObjectURL(url);
}

/*
 * useImportRun follows a started import until it is over, then refreshes
 * the lists it changed.
 */
function useImportRun(workspace: string, id: string | undefined) {
  const client = useQueryClient();
  const { data: run } = useQuery({
    ...importQuery(workspace, id ?? ""),
    enabled: id !== undefined,
    refetchInterval: (query) =>
      query.state.data?.status === "completed" || query.state.data?.status === "failed" ? false : pollEvery,
  });
  const finished = run?.status === "completed" || run?.status === "failed";

  useEffect(() => {
    if (finished) {
      void client.invalidateQueries({ queryKey: ["workspace", workspace] });
    }
  }, [finished, client, workspace]);

  return { run, finished };
}

function Footer({
  step,
  history,
  finished,
  problem,
  canStart,
  starting,
  rows,
  onGo,
  onStart,
  onHistory,
  onClose,
}: {
  step: Step;
  history: boolean;
  finished: boolean;
  problem: string | null;
  canStart: boolean;
  starting: boolean;
  rows: number;
  onGo: (step: Step) => void;
  onStart: () => void;
  onHistory: (open: boolean) => void;
  onClose: () => void;
}) {
  const closeLabel = step === "upload" ? "Cancel" : finished ? "Done" : "Close";

  if (history) {
    return (
      <ModalFooter className="mt-6">
        <Button variant="secondary" onClick={() => onHistory(false)}>
          Back
        </Button>
        <Button onClick={onClose}>Done</Button>
      </ModalFooter>
    );
  }

  return (
    <ModalFooter className="mt-6">
      {step === "upload" ? (
        <Button variant="secondary" leadingIcon={RiHistoryLine} onClick={() => onHistory(true)} className="me-auto">
          Past imports
        </Button>
      ) : null}
      {step === "map" || step === "options" ? (
        <Button variant="secondary" onClick={() => onGo(step === "map" ? "upload" : "map")}>
          Back
        </Button>
      ) : (
        <Button variant="secondary" onClick={onClose}>
          {closeLabel}
        </Button>
      )}
      {step === "map" ? (
        <Button disabled={problem !== null} onClick={() => onGo("options")}>
          Continue
        </Button>
      ) : null}
      {step === "options" ? (
        <Button disabled={!canStart} loading={starting} onClick={onStart}>
          Import {formatNumber(rows)} rows
        </Button>
      ) : null}
    </ModalFooter>
  );
}

type View = Step | "history";

/*
 * heading titles the dialog and says which step it is on.
 */
function heading(view: View, text: (typeof copy)[ImportKind]): { title: string; description: string } {
  if (view === "history") {
    return {
      title: `Past imports of ${text.noun}`,
      description: "Imports can be undone for 7 days after they finish.",
    };
  }

  const said = view === "upload" ? text.upload : descriptions[view];

  return { title: text.title, description: `Step ${steps.indexOf(view) + 1} of 4 · ${said}` };
}

function Notes({ problem, failure }: { problem: string | null; failure: Error | null }) {
  return (
    <>
      {problem ? <p className="mt-3 text-body-2-regular text-text-tertiary">{problem}</p> : null}
      {failure ? (
        <p role="alert" className="mt-3 text-body-2-regular text-text-error-primary">
          {failure.message}
        </p>
      ) : null}
    </>
  );
}

function ImportDialog({ kind, mayUndo, onClose }: { kind: ImportKind; mayUndo: boolean; onClose: () => void }) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const text = copy[kind];
  const [history, setHistory] = useState(false);
  const [step, setStep] = useState<Step>("upload");
  const [forward, setForward] = useState(true);
  const [draft, setDraft] = useState<Import | null>(null);
  const [fields, setFields] = useState<string[]>([]);
  const [mode, setMode] = useState<ImportMode>("update");
  const { data: attributes = [] } = useQuery(attributesQuery(workspace));
  const choices = fieldChoices(kind, attributes);
  const id = draft?.id ?? "";
  const go = (next: Step) => {
    setForward(steps.indexOf(next) > steps.indexOf(step));
    setStep(next);
  };

  const upload = useMutation({
    mutationFn: (file: File) => uploadImport(workspace, kind, file),
    onSuccess: (created) => {
      setDraft(created);
      setFields(initialFields(created.columns, created.suggested_mapping));
      go("map");
    },
  });
  const preview = useMutation({ mutationFn: (plan: ImportPlan) => previewImport(workspace, id, plan) });
  const start = useMutation({
    mutationFn: (plan: ImportPlan) => startImport(workspace, id, plan),
    onSuccess: (started) => {
      client.setQueryData(importQuery(workspace, started.id).queryKey, started);
      go("run");
    },
  });
  const { run, finished } = useImportRun(workspace, start.data?.id);
  const problem = mappingProblem(kind, fields);
  const failure = upload.error ?? preview.error ?? start.error;
  const view: View = history ? "history" : step;
  const check = (next: ImportMode) => {
    setMode(next);
    preview.mutate({ mapping: planFrom(fields), mode: next });
  };

  return (
    <Modal size="wide" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader {...heading(view, text)} />
      <ModalBody className="max-h-[min(560px,calc(100dvh-14rem))] overflow-y-auto">
        <Resize>
          <Swap view={view} forward={forward}>
            {view === "history" ? (
              <ImportHistory workspace={workspace} kind={kind} noun={text.noun} mayUndo={mayUndo} />
            ) : null}
            {view === "upload" ? (
              <FileUpload allowedExtensions={["csv"]} maxBytes={maxBytes} onUploadComplete={(f) => upload.mutate(f)} />
            ) : null}
            {step === "map" && draft ? (
              <MapStep draft={draft} choices={choices} fields={fields} onChange={setFields} />
            ) : null}
            {step === "options" ? (
              <OptionsStep
                noun={text.noun}
                mode={mode}
                onMode={check}
                preview={preview.data}
                checking={preview.isPending}
              />
            ) : null}
            {step === "run" && run ? (
              <RunStep
                run={run}
                noun={text.noun}
                onDownload={() => void downloadImportErrors(workspace, run.id).then((f) => save(f.blob, f.filename))}
              />
            ) : null}
          </Swap>
        </Resize>
        {view === "history" ? null : <Notes problem={view === "map" ? problem : null} failure={failure} />}
      </ModalBody>
      <Footer
        step={step}
        history={history}
        finished={finished}
        problem={problem}
        canStart={preview.data !== undefined && !preview.isPending && !start.isPending}
        starting={start.isPending}
        rows={draft?.row_count ?? 0}
        onGo={(next) => {
          go(next);
          if (next === "options") {
            check(mode);
          }
        }}
        onStart={() => start.mutate({ mapping: planFrom(fields), mode })}
        onHistory={(open) => {
          setForward(open);
          setHistory(open);
        }}
        onClose={onClose}
      />
    </Modal>
  );
}

/*
 * ImportButton opens the import of the kind, for members who may manage
 * imports.
 */
export function ImportButton({ kind }: { kind: ImportKind }) {
  const workspace = useWorkspace();
  const [open, setOpen] = useState(false);
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const allowed = caller?.member?.permissions.includes("imports:manage") ?? false;
  const mayUndo = caller?.member?.permissions.includes("imports:rollback") ?? false;

  if (!allowed) {
    return null;
  }

  return (
    <>
      <Button variant="secondary" size="small" leadingIcon={RiUploadLine} onClick={() => setOpen(true)}>
        Import
      </Button>
      {open ? <ImportDialog key={kind} kind={kind} mayUndo={mayUndo} onClose={() => setOpen(false)} /> : null}
    </>
  );
}
