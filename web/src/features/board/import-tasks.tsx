/**
 * Bulk import: a CSV in, a column of cards out.
 *
 * The server does this in one transaction — every row lands or none do — and
 * this dialog is built to make that promise legible rather than to soften it.
 * When rows fail, the failure list IS the screen: the whole point of an
 * all-or-nothing import is that the user fixes their file once and retries,
 * so the errors have to be specific enough to act on without guessing which
 * line was meant.
 *
 * The file is read here rather than uploaded as multipart because this API's
 * contract is generated from Go structs, and a typed string field survives
 * that trip where a multipart body would not. See ImportTasksInput.
 */
import { Upload } from "lucide-react";
import { useRef, useState } from "react";
import { BeaconError } from "@beacon/sdk";
import { useImportTasks } from "@/api/queries";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTrigger } from "@/components/ui/dialog";
import { useToast } from "@/components/ui/toast";
import { useOrgContext } from "@/features/org/org-gate";

const SAMPLE = `title,status
Fix the login bug,todo
Ship the invoice screen,in_progress`;

export function ImportTasks({ projectID }: { projectID: string }) {
  const { org } = useOrgContext();
  const toast = useToast();
  const importTasks = useImportTasks(org.id, projectID);

  const [open, setOpen] = useState(false);
  const [csv, setCSV] = useState("");
  const [fileName, setFileName] = useState<string | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  // Row errors live in their own state rather than being read off the
  // mutation: they survive the user editing the textarea, so the list they
  // are fixing against does not vanish the moment they start fixing it.
  const [rowErrors, setRowErrors] = useState<
    { line: number; column?: string; message: string }[]
  >([]);
  const [failure, setFailure] = useState<string | null>(null);

  function reset() {
    setCSV("");
    setFileName(null);
    setRowErrors([]);
    setFailure(null);
    importTasks.reset();
  }

  async function onFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setFileName(file.name);
    setCSV(await file.text());
    setRowErrors([]);
    setFailure(null);
    // Clear the input's value so choosing the SAME file again still fires a
    // change event — otherwise a user who edits and re-picks their file sees
    // nothing happen.
    e.target.value = "";
  }

  function submit() {
    setRowErrors([]);
    setFailure(null);

    importTasks.mutate(csv, {
      onSuccess: (created) => {
        toast({
          tone: "success",
          title: `Imported ${created.length} ${created.length === 1 ? "task" : "tasks"}`,
        });
        setOpen(false);
        reset();
      },
      onError: (err) => {
        if (err instanceof BeaconError && err.rows.length > 0) {
          setRowErrors(
            err.rows.map((r) => ({
              line: r.line ?? 0,
              ...(r.column ? { column: r.column } : {}),
              message: r.message ?? "",
            })),
          );
          return;
        }
        // Anything else — an empty file, too many rows, a project that is not
        // yours, the server being down. The message is the server's, which is
        // written for this and says more than "import failed" would.
        setFailure(err instanceof Error ? err.message : "The import failed");
      },
    });
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) reset();
      }}
    >
      <DialogTrigger asChild>
        <Button variant="secondary" size="sm">
          <Upload className="size-3.5" />
          Import
        </Button>
      </DialogTrigger>

      <DialogContent
        title="Import tasks"
        description="Paste CSV or choose a file. Every row is created, or none are."
        className="max-w-lg"
      >
        <div className="space-y-3">
          <input
            ref={fileInput}
            type="file"
            accept=".csv,text/csv"
            onChange={onFile}
            className="sr-only"
          />
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => fileInput.current?.click()}>
              Choose a file
            </Button>
            {fileName && <span className="truncate text-ui text-ink-muted">{fileName}</span>}
          </div>

          <div>
            <label htmlFor="csv" className="mb-1.5 block text-ui font-medium text-ink">
              CSV
            </label>
            <textarea
              id="csv"
              value={csv}
              onChange={(e) => {
                setCSV(e.target.value);
                setFailure(null);
              }}
              rows={7}
              spellCheck={false}
              placeholder={SAMPLE}
              className="w-full rounded-(--radius-ctl) border border-line bg-page px-3 py-2 font-mono text-ui text-ink placeholder:text-ink-faint focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
            />
            <p className="mt-1.5 text-caption text-ink-muted">
              Needs a <code className="font-mono">title</code> column.{" "}
              <code className="font-mono">status</code> and{" "}
              <code className="font-mono">position</code> are optional; status is one of todo,
              in_progress or done.
            </p>
          </div>

          {failure && (
            <p role="alert" className="text-ui text-danger-text">
              {failure}
            </p>
          )}

          {rowErrors.length > 0 && (
            <div role="alert" className="rounded-(--radius-ctl) border border-danger/40 bg-danger-subtle p-3">
              <p className="text-ui font-medium text-danger-text">
                {rowErrors.length} {rowErrors.length === 1 ? "row" : "rows"} could not be imported.
                Nothing was written.
              </p>
              <ul className="mt-2 max-h-40 space-y-1 overflow-y-auto">
                {rowErrors.map((r, i) => (
                  <li key={`${r.line}-${i}`} className="font-mono text-caption text-ink-muted">
                    <span className="text-ink">line {r.line}</span>
                    {r.column ? ` · ${r.column}` : ""} — {r.message}
                  </li>
                ))}
              </ul>
            </div>
          )}

          <div className="flex justify-end gap-2 pt-1">
            <Button variant="ghost" size="sm" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={submit}
              busy={importTasks.isPending}
              disabled={csv.trim() === ""}
            >
              Import
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
