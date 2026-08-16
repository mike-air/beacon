import { useParams } from "@tanstack/react-router";
import { Plus } from "lucide-react";
import { useState } from "react";
import { useCreateTask, useProjects, useTasks } from "@/api/queries";
import { STATUS_LABEL, TASK_STATUSES, type TaskStatus } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { EmptyBoard } from "@/components/ui/illustration";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useOrgContext } from "@/features/org/org-gate";

/**
 * Position is a float so a card can land between two others without
 * renumbering the column. A card added to the end of a column gets the last
 * position plus a wide gap; a card dropped between two gets their midpoint.
 */
const GAP = 1000;

function nextPosition(positions: number[]): number {
  return positions.length === 0 ? GAP : Math.max(...positions) + GAP;
}

function Column({
  status,
  count,
  children,
  onAdd,
  adding,
}: {
  status: TaskStatus;
  count: number;
  children: React.ReactNode;
  onAdd: (title: string) => void;
  adding: boolean;
}) {
  const [draft, setDraft] = useState("");
  const [open, setOpen] = useState(false);

  return (
    <section className="flex min-w-0 flex-col rounded-(--radius-card) bg-sunken p-2.5">
      <h2 className="mb-2.5 flex items-center gap-2 px-1 text-[11.5px] font-medium uppercase tracking-wide text-ink-muted">
        {STATUS_LABEL[status]}
        <span className="rounded-full bg-well px-1.5 font-mono text-[10px]">{count}</span>
      </h2>

      <div className="min-h-2 space-y-2">{children}</div>

      {open ? (
        <form
          className="mt-2"
          onSubmit={(e) => {
            e.preventDefault();
            const t = draft.trim();
            if (!t) return;
            onAdd(t);
            setDraft("");
          }}
        >
          <Input
            autoFocus
            value={draft}
            placeholder="What needs doing?"
            onChange={(e) => setDraft(e.target.value)}
            onBlur={() => !draft && setOpen(false)}
            onKeyDown={(e) => e.key === "Escape" && setOpen(false)}
          />
          <div className="mt-1.5 flex gap-1.5">
            <Button type="submit" size="sm" busy={adding}>
              Add
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
          </div>
        </form>
      ) : (
        <Button
          variant="ghost"
          size="sm"
          className="mt-2 w-full justify-start text-ink-faint"
          onClick={() => setOpen(true)}
        >
          <Plus className="size-3.5" />
          Add task
        </Button>
      )}
    </section>
  );
}

export function BoardPage() {
  const { org } = useOrgContext();
  const { projectID } = useParams({ from: "/app/projects/$projectID" });
  const projects = useProjects(org.id);
  const query = useTasks(org.id, projectID);
  const create = useCreateTask(org.id, projectID);

  const project = projects.data?.items.find((p) => p.id === projectID);
  const items = query.data?.items ?? [];

  return (
    <div className="mx-auto max-w-6xl px-4 py-6">
      <div className="mb-4 flex items-center gap-3">
        <h1 className="font-display text-xl tracking-tight text-ink">
          {project?.name ?? "Board"}
        </h1>
        {projects.data?.board === "v2" && <Badge tone="volt">new board</Badge>}
        <div className="flex-1" />
        <span className="font-mono text-[11.5px] text-ink-faint">
          {items.length} {items.length === 1 ? "task" : "tasks"}
        </span>
      </div>

      {query.isPending && (
        <div className="grid gap-3 sm:grid-cols-3">
          <Skeleton className="h-72 rounded-(--radius-card)" />
          <Skeleton className="h-72 rounded-(--radius-card)" />
          <Skeleton className="h-72 rounded-(--radius-card)" />
        </div>
      )}

      {query.isSuccess && items.length === 0 && (
        <EmptyState
          illustration={<EmptyBoard />}
          title="Nothing on the board"
          description="Add the first task to any column and it appears for everyone in the org."
        />
      )}

      {query.isSuccess && items.length > 0 && (
        <div className="grid gap-3 sm:grid-cols-3">
          {TASK_STATUSES.map((status) => {
            const column = items
              .filter((t) => t.status === status)
              .sort((a, b) => a.position - b.position);
            return (
              <Column
                key={status}
                status={status}
                count={column.length}
                adding={create.isPending}
                onAdd={(title) =>
                  create.mutate({
                    title,
                    status,
                    position: nextPosition(column.map((t) => t.position)),
                  })
                }
              >
                {column.map((t) => (
                  <article
                    key={t.id}
                    className="rounded-(--radius-card) border border-line bg-raised px-3 py-2.5 transition-colors hover:border-line-strong"
                  >
                    <p className="text-[13.5px] text-ink">{t.title}</p>
                  </article>
                ))}
              </Column>
            );
          })}
        </div>
      )}
    </div>
  );
}
