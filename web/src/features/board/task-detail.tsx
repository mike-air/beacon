import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Paperclip, Send, Trash2, Upload } from "lucide-react";
import { useRef, useState } from "react";
import { tasks as tasksApi } from "@/api/endpoints";
import { BeaconError } from "@beacon/sdk";
import { keys, useAttachments, useComments } from "@/api/queries";
import { STATUS_LABEL, TASK_STATUSES, type Task, type TaskStatus } from "@/api/types";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";

function bytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 ** 2) return `${(n / 1024).toFixed(0)} KB`;
  if (n < 1024 ** 3) return `${(n / 1024 ** 2).toFixed(1)} MB`;
  return `${(n / 1024 ** 3).toFixed(1)} GB`;
}

function when(iso: string): string {
  const d = new Date(iso);
  const mins = Math.round((Date.now() - d.getTime()) / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  if (mins < 1440) return `${Math.round(mins / 60)}h ago`;
  return d.toLocaleDateString();
}

export function TaskDetail({
  orgID,
  projectID,
  task,
  open,
  onClose,
}: {
  orgID: string;
  projectID: string;
  task: Task | undefined;
  open: boolean;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const toast = useToast();
  const comments = useComments(orgID, projectID, task?.id);
  const attachments = useAttachments(orgID, projectID, task?.id);
  const [draft, setDraft] = useState("");
  const fileInput = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);

  const addComment = useMutation({
    mutationFn: (body: string) => tasksApi.comment(orgID, projectID, task!.id, { body }),
    onSuccess: () => {
      setDraft("");
      void qc.invalidateQueries({ queryKey: keys.comments(orgID, projectID, task!.id) });
    },
    onError: (e) =>
      toast({
        tone: "danger",
        title: "Comment not saved",
        description: e instanceof BeaconError ? e.message : "Try again.",
      }),
  });

  const setStatus = useMutation({
    mutationFn: (status: TaskStatus) =>
      tasksApi.update(orgID, projectID, task!.id, {
        title: task!.title,
        status,
        position: task!.position,
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: keys.tasks(orgID, projectID) }),
  });

  /**
   * The bytes never touch Beacon. It hands back a presigned PUT and the
   * browser uploads straight to storage — which is why the reserve call and
   * the upload are two separate failures with two separate messages. A row
   * exists after the first one whether or not the second succeeds.
   */
  async function upload(file: File) {
    if (!task) return;
    setUploading(true);
    try {
      const reserved = await tasksApi.createAttachment(orgID, projectID, task.id, {
        filename: file.name,
        content_type: file.type || "application/octet-stream",
        size: file.size,
      });
      if (!reserved.upload_url) {
        toast({
          tone: "warning",
          title: "Storage is not configured",
          description: "This Beacon has no file storage, so the upload was skipped.",
        });
        return;
      }
      const res = await fetch(reserved.upload_url, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": file.type || "application/octet-stream" },
      });
      if (!res.ok) throw new Error(`upload failed: ${res.status}`);
      toast({ tone: "success", title: "Uploaded", description: file.name });
      void qc.invalidateQueries({ queryKey: keys.attachments(orgID, projectID, task.id) });
    } catch (e) {
      const storageDisabled = e instanceof BeaconError && e.status === 501;
      toast({
        tone: storageDisabled ? "warning" : "danger",
        title: storageDisabled ? "Storage is not configured" : "Upload failed",
        description: e instanceof Error ? e.message : "Try again.",
      });
    } finally {
      setUploading(false);
      if (fileInput.current) fileInput.current.value = "";
    }
  }

  const storageDisabled =
    attachments.error instanceof BeaconError && attachments.error.isNotImplemented;

  if (!task) return null;

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent title={task.title} className="max-w-lg">
        <div className="mb-4 flex items-center gap-2">
          <Select
            value={task.status}
            onValueChange={(v) => setStatus.mutate(v as TaskStatus)}
            className="w-44"
            options={TASK_STATUSES.map((s) => ({ value: s, label: STATUS_LABEL[s] }))}
          />
          <Badge tone={task.status === "done" ? "success" : "neutral"}>
            {STATUS_LABEL[task.status]}
          </Badge>
          <div className="flex-1" />
          <span className="font-mono text-[10.5px] text-ink-faint">{when(task.updated_at)}</span>
        </div>

        {/* ---- attachments ---- */}
        <section className="mb-5">
          <h3 className="mb-2 flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-ink-muted">
            <Paperclip className="size-3" />
            Files
          </h3>
          {attachments.isPending && <Skeleton className="h-9" />}
          {/* A 501 is not a failure to explain away: this Beacon simply has no
              storage configured. Saying nothing leaves an empty box that looks
              like a bug in the client. */}
          {attachments.isError && (
            <p className="rounded-(--radius-ctl) bg-warning-subtle px-2.5 py-2 text-[12.5px] text-warning-text">
              {storageDisabled
                ? "File storage is not configured on this server."
                : "Could not load files."}
            </p>
          )}
          {attachments.isSuccess && attachments.data.items.length === 0 && (
            <p className="text-[12.5px] text-ink-faint">Nothing attached yet.</p>
          )}
          {attachments.isSuccess && attachments.data.items.length > 0 && (
            <ul className="space-y-1.5">
              {attachments.data.items.map((a) => (
                <li
                  key={a.id}
                  className="flex items-center gap-2 rounded-(--radius-ctl) border border-line px-2.5 py-1.5"
                >
                  <Paperclip className="size-3.5 shrink-0 text-ink-faint" />
                  <span className="min-w-0 flex-1 truncate text-[13px] text-ink">{a.filename}</span>
                  <span className="font-mono text-[10.5px] text-ink-faint">
                    {bytes(a.size_bytes)}
                  </span>
                </li>
              ))}
            </ul>
          )}
          <input
            ref={fileInput}
            type="file"
            className="sr-only"
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) void upload(f);
            }}
          />
          {!storageDisabled && (
            <Button
              variant="secondary"
              size="sm"
              className="mt-2"
              busy={uploading}
              onClick={() => fileInput.current?.click()}
            >
              <Upload className="size-3.5" />
              Attach a file
            </Button>
          )}
        </section>

        {/* ---- comments ---- */}
        <section>
          <h3 className="mb-2 text-[11px] font-medium uppercase tracking-wide text-ink-muted">
            Comments
          </h3>

          {comments.isPending && (
            <div className="space-y-2">
              <Skeleton className="h-12" />
              <Skeleton className="h-12" />
            </div>
          )}

          {comments.isError && (
            <p className="rounded-(--radius-ctl) bg-danger-subtle px-2.5 py-2 text-[12.5px] text-danger-text">
              Could not load comments.
            </p>
          )}

          {comments.isSuccess && comments.data.items.length === 0 && (
            <p className="text-[12.5px] text-ink-faint">No comments yet.</p>
          )}

          {comments.isSuccess && comments.data.items.length > 0 && (
            <ul className="max-h-52 space-y-3 overflow-y-auto pr-1">
              {comments.data.items.map((c) => (
                <li key={c.id} className="flex gap-2">
                  <Avatar name={c.author_id.slice(0, 2)} size="sm" />
                  <div className="min-w-0 flex-1">
                    <p className="text-[13px] text-ink">{c.body}</p>
                    <p className="mt-0.5 font-mono text-[10.5px] text-ink-faint">
                      {when(c.created_at)}
                    </p>
                  </div>
                </li>
              ))}
            </ul>
          )}

          <form
            className="mt-3 flex gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              const body = draft.trim();
              if (body) addComment.mutate(body);
            }}
          >
            <Input
              value={draft}
              aria-label="Write a comment"
              placeholder="Write a comment"
              onChange={(e) => setDraft(e.target.value)}
            />
            <Button type="submit" busy={addComment.isPending} disabled={!draft.trim()}>
              <Send className="size-3.5" />
            </Button>
          </form>
        </section>

        <div className="mt-5 flex justify-end border-t border-line pt-4">
          <Button
            variant="ghost"
            className="text-danger-text"
            onClick={() => {
              void tasksApi.remove(orgID, projectID, task.id).then(() => {
                void qc.invalidateQueries({ queryKey: keys.tasks(orgID, projectID) });
                onClose();
              });
            }}
          >
            <Trash2 className="size-3.5" />
            Delete task
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
