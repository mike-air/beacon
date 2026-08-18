import { useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { Plus } from "lucide-react";
import { useMemo, useRef, useState } from "react";
import { useCreateTask, useProjects, useTasks } from "@/api/queries";
import { STATUS_LABEL, TASK_STATUSES, type Task, type TaskStatus } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { EmptyBoard } from "@/components/ui/illustration";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import { cn } from "@/lib/cn";
import { useOrgContext } from "@/features/org/org-gate";
import { ImportTasks } from "./import-tasks";
import { TaskCard } from "./task-card";
import { TaskDetail } from "./task-detail";
import { GAP, needsRebalance, positionAt } from "./position";
import { useMoveTask } from "./use-move-task";

function AddTask({
  onAdd,
  busy,
  column,
}: {
  onAdd: (title: string) => void;
  busy: boolean;
  column: string;
}) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState("");

  if (!open) {
    return (
      <Button
        variant="ghost"
        size="sm"
        className="mt-1.5 w-full justify-start text-ink-faint"
        onClick={() => setOpen(true)}
      >
        <Plus className="size-3.5" />
        Add task
      </Button>
    );
  }

  return (
    <form
      className="mt-1.5"
      onSubmit={(e) => {
        e.preventDefault();
        const title = draft.trim();
        if (!title) return;
        onAdd(title);
        setDraft(""); // stay open: adding one task usually means adding three
      }}
    >
      <Input
        autoFocus
        value={draft}
        aria-label={`New task in ${column}`}
        placeholder="What needs doing?"
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") setOpen(false);
        }}
      />
      <div className="mt-1.5 flex gap-1.5">
        <Button type="submit" size="sm" busy={busy}>
          Add
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={() => setOpen(false)}>
          Done
        </Button>
      </div>
    </form>
  );
}

export function BoardPage() {
  const { org } = useOrgContext();
  const { projectID } = useParams({ from: "/app/projects/$projectID" });
  const search = useSearch({ from: "/app/projects/$projectID" });
  const navigate = useNavigate();
  const toast = useToast();

  const projects = useProjects(org.id);
  const query = useTasks(org.id, projectID);
  const create = useCreateTask(org.id, projectID);
  const move = useMoveTask(org.id, projectID);

  const [dragId, setDragId] = useState<string | null>(null);
  const [overColumn, setOverColumn] = useState<TaskStatus | null>(null);
  const [dropIndex, setDropIndex] = useState<number | null>(null);
  // Which cards to flash after a realtime update. A ref, not state: writing it
  // must not itself cause the render that reads it.
  const seen = useRef<Map<string, string>>(new Map());

  const project = projects.data?.items.find((p) => p.id === projectID);
  const items = useMemo(() => query.data?.items ?? [], [query.data]);

  const columns = useMemo(() => {
    const out = {} as Record<TaskStatus, Task[]>;
    for (const s of TASK_STATUSES) {
      out[s] = items.filter((t) => t.status === s).sort((a, b) => a.position - b.position);
    }
    return out;
  }, [items]);

  // A card whose updated_at changed while it was already on screen was moved
  // by somebody else. Flash it once so the board does not silently rearrange.
  const flashing = new Set<string>();
  for (const t of items) {
    const previous = seen.current.get(t.id);
    if (previous && previous !== t.updated_at) flashing.add(t.id);
    seen.current.set(t.id, t.updated_at);
  }

  const openTaskId = search.task;
  const openTask = items.find((t) => t.id === openTaskId);

  function setOpenTask(id: string | undefined) {
    // The open card lives in the URL, so a task is a link you can send.
    void navigate({
      to: "/projects/$projectID",
      params: { projectID },
      search: id ? { task: id } : {},
      replace: true,
    });
  }

  function commitMove(task: Task, status: TaskStatus, index: number) {
    const target = columns[status].filter((t) => t.id !== task.id);
    const positions = target.map((t) => t.position);
    const position = positionAt(positions, index);

    if (needsRebalance([...positions, position].sort((a, b) => a - b))) {
      // Honest rather than silent: the column has been subdivided so often
      // that two positions are about to collide. Beacon has no bulk-reorder
      // endpoint to fix it with (DEVIATIONS.md), so say so.
      toast({
        tone: "warning",
        title: "This column needs renumbering",
        description: "Cards have been reordered so many times the gaps have collapsed.",
      });
    }
    move.mutate({ task, status, position });
  }

  const isEmpty = query.isSuccess && items.length === 0;

  return (
    <div className="mx-auto max-w-6xl px-4 py-6">
      <div className="mb-4 flex items-center gap-3">
        <h1 className="font-display text-xl tracking-tight text-ink">{project?.name ?? "Board"}</h1>
        {projects.data?.board === "v2" && <Badge tone="volt">new board</Badge>}
        <div className="flex-1" />
        <span className="font-mono text-label text-ink-faint">
          {items.length} {items.length === 1 ? "task" : "tasks"}
        </span>
        <ImportTasks projectID={projectID} />
      </div>

      {query.isPending && (
        <div className="grid gap-3 sm:grid-cols-3">
          {TASK_STATUSES.map((s) => (
            <Skeleton key={s} className="h-72 rounded-(--radius-card)" />
          ))}
        </div>
      )}

      {isEmpty && (
        <EmptyState
          illustration={<EmptyBoard />}
          title="Nothing on the board"
          description="Add the first task to any column. Everyone in the org sees it appear."
          action={
            <Button
              onClick={() =>
                create.mutate({ title: "First task", status: "todo", position: GAP })
              }
              busy={create.isPending}
            >
              <Plus className="size-3.5" />
              Add a task
            </Button>
          }
        />
      )}

      {query.isSuccess && !isEmpty && (
        <div className="grid gap-3 sm:grid-cols-3">
          {TASK_STATUSES.map((status) => {
            const column = columns[status];
            const isOver = overColumn === status;
            return (
              <section
                key={status}
                aria-label={STATUS_LABEL[status]}
                onDragOver={(e) => {
                  e.preventDefault();
                  setOverColumn(status);
                }}
                onDragLeave={(e) => {
                  // Leaving a child still fires on the parent; only clear when
                  // the pointer has actually left the column.
                  if (!e.currentTarget.contains(e.relatedTarget as Node)) {
                    setOverColumn(null);
                    setDropIndex(null);
                  }
                }}
                onDrop={(e) => {
                  e.preventDefault();
                  // The id comes off the dataTransfer, not React state. State
                  // is what the closure captured at render time; dataTransfer
                  // is what the drag is actually carrying, which is the whole
                  // reason the API has one. It also means a drop still works
                  // if the dragstart render has not committed yet.
                  const id = e.dataTransfer.getData("text/plain") || dragId;
                  const task = items.find((t) => t.id === id);
                  if (task) {
                    commitMove(task, status, dropIndex ?? column.length);
                  }
                  setDragId(null);
                  setOverColumn(null);
                  setDropIndex(null);
                }}
                className={cn(
                  "flex min-w-0 flex-col rounded-(--radius-card) border p-2 transition-colors duration-140",
                  isOver ? "border-accent bg-accent-subtle/40" : "border-line bg-page",
                )}
              >
                <h2 className="mb-2 flex items-center gap-2 px-1 text-label font-medium uppercase tracking-wide text-ink-muted">
                  {STATUS_LABEL[status]}
                  <span className="rounded-full bg-well px-1.5 font-mono text-micro">
                    {column.length}
                  </span>
                </h2>

                <div className="space-y-1.5">
                  {column.map((task, i) => (
                    <div
                      key={task.id}
                      onDragOver={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        const box = e.currentTarget.getBoundingClientRect();
                        const below = e.clientY > box.top + box.height / 2;
                        setOverColumn(status);
                        setDropIndex(below ? i + 1 : i);
                      }}
                    >
                      {isOver && dropIndex === i && <DropLine />}
                      <TaskCard
                        task={task}
                        dragging={dragId === task.id}
                        justUpdated={flashing.has(task.id)}
                        onOpen={() => setOpenTask(task.id)}
                        onMoveTo={(s) => commitMove(task, s, columns[s].length)}
                        onDragStart={(e) => {
                          e.dataTransfer.effectAllowed = "move";
                          // Firefox will not start a drag without payload.
                          e.dataTransfer.setData("text/plain", task.id);
                          setDragId(task.id);
                        }}
                        onDragEnd={() => {
                          setDragId(null);
                          setOverColumn(null);
                          setDropIndex(null);
                        }}
                      />
                    </div>
                  ))}
                  {isOver && dropIndex === column.length && <DropLine />}
                </div>

                <AddTask
                  column={STATUS_LABEL[status]}
                  busy={create.isPending}
                  onAdd={(title) =>
                    create.mutate({
                      title,
                      status,
                      position: positionAt(column.map((t) => t.position), column.length),
                    })
                  }
                />
              </section>
            );
          })}
        </div>
      )}

      <TaskDetail
        orgID={org.id}
        projectID={projectID}
        task={openTask}
        open={!!openTaskId}
        onClose={() => setOpenTask(undefined)}
      />
    </div>
  );
}

function DropLine() {
  return <div aria-hidden className="my-1 h-0.5 rounded-full bg-accent" />;
}
