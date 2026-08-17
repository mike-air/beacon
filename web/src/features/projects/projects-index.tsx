import { zodResolver } from "@hookform/resolvers/zod";
import { Link } from "@tanstack/react-router";
import { Plus } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { useCreateProject, useProjects } from "@/api/queries";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { EmptyProjects } from "@/components/ui/illustration";
import { Field, Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { FormError } from "@/features/auth/form-error";
import { useSubmit } from "@/features/auth/use-submit";
import { useOrgContext } from "@/features/org/org-gate";

const schema = z.object({ name: z.string().min(1, "Give it a name").max(200, "Too long") });
type Values = z.infer<typeof schema>;

function NewProjectDialog({
  orgID,
  open,
  onOpenChange,
}: {
  orgID: string;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const create = useCreateProject(orgID);
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { name: "" } });
  const submit = useSubmit<Values>(form.setError, ["name"]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent title="New project" description="A project is one board.">
        <form
          noValidate
          onSubmit={form.handleSubmit(async (v) => {
            const ok = await submit.run(async () => {
              await create.mutateAsync(v.name);
            });
            if (ok) {
              form.reset();
              onOpenChange(false);
            }
          })}
        >
          <FormError error={submit.error} />
          <Field label="Project name" error={form.formState.errors.name?.message}>
            {(p) => <Input {...p} autoFocus placeholder="Website relaunch" {...form.register("name")} />}
          </Field>
          <div className="mt-4 flex justify-end gap-2">
            <Button variant="ghost" type="button" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" busy={form.formState.isSubmitting}>
              Create project
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function ProjectsIndex() {
  const { org } = useOrgContext();
  const query = useProjects(org.id);
  const [creating, setCreating] = useState(false);

  return (
    <div className="mx-auto max-w-5xl px-4 py-7">
      <div className="mb-5 flex items-center gap-3">
        <h1 className="font-display text-xl tracking-tight text-ink">Projects</h1>
        {query.data?.board === "v2" && <Badge tone="volt">new board</Badge>}
        <div className="flex-1" />
        <Button onClick={() => setCreating(true)}>
          <Plus className="size-3.5" />
          New project
        </Button>
      </div>

      {query.isPending && (
        <div className="space-y-2">
          <Skeleton className="h-16 rounded-(--radius-card)" />
          <Skeleton className="h-16 rounded-(--radius-card)" />
        </div>
      )}

      {query.isSuccess && query.data.items.length === 0 && (
        <EmptyState
          illustration={<EmptyProjects />}
          title="No projects yet"
          description="A project holds one board. Make the first one and put a task on it."
          action={
            <Button onClick={() => setCreating(true)}>
              <Plus className="size-3.5" />
              New project
            </Button>
          }
        />
      )}

      {query.isSuccess && query.data.items.length > 0 && (
        <ul className="space-y-2">
          {query.data.items.map((p) => (
            <li key={p.id}>
              <Link
                to="/projects/$projectID"
                params={{ projectID: p.id }}
                className="block rounded-(--radius-card) border border-line bg-raised px-4 py-3.5 transition-colors hover:border-line-strong hover:bg-well/40"
              >
                <span className="font-medium text-ink">{p.name}</span>
                <span className="ml-2 font-mono text-label text-ink-faint">
                  updated {new Date(p.updated_at).toLocaleDateString()}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}

      <NewProjectDialog orgID={org.id} open={creating} onOpenChange={setCreating} />
    </div>
  );
}
