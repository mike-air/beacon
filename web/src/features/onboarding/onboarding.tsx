/**
 * First run.
 *
 * The goal is not to explain Beacon; it is to get the user to a board that
 * has something on it. Four steps, each one API call, and the two that are
 * not strictly required say so out loud — an onboarding you cannot skip is a
 * wall, not a welcome.
 *
 * Every step is resumable: it reads what already exists rather than trusting
 * that the previous screen completed. A user who closes the tab after making
 * an org comes back to step two, not step one.
 */
import { zodResolver } from "@hookform/resolvers/zod";
import { useNavigate } from "@tanstack/react-router";
import { Check } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { orgs as orgsApi, projects as projectsApi, tasks as tasksApi } from "@/api/endpoints";
import { keys } from "@/api/queries";
import { useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Wordmark } from "@/components/ui/logo";
import { cn } from "@/lib/cn";
import { FormError } from "@/features/auth/form-error";
import { useSubmit } from "@/features/auth/use-submit";
import { setActiveOrg } from "@/features/org/active-org";

const STEPS = ["Organisation", "Team", "Project", "First task"] as const;

function Progress({ step }: { step: number }) {
  return (
    <ol className="mb-8 flex items-center gap-2" aria-label="Progress">
      {STEPS.map((label, i) => {
        const done = i < step;
        const current = i === step;
        return (
          <li key={label} className="flex flex-1 items-center gap-2">
            <span
              aria-current={current ? "step" : undefined}
              className={cn(
                "flex size-5 shrink-0 items-center justify-center rounded-full text-[10px] font-semibold transition-colors",
                done && "bg-success text-on-accent",
                current && "bg-accent text-on-accent",
                !done && !current && "bg-well text-ink-faint",
              )}
            >
              {done ? <Check className="size-3" strokeWidth={3} /> : i + 1}
            </span>
            <span
              className={cn(
                "hidden text-[12px] sm:block",
                current ? "font-medium text-ink" : "text-ink-faint",
              )}
            >
              {label}
            </span>
            {i < STEPS.length - 1 && <span className="h-px flex-1 bg-line" />}
          </li>
        );
      })}
    </ol>
  );
}

function Frame({ step, children }: { step: number; children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col bg-sunken">
      <header className="px-6 py-5">
        <Wordmark live />
      </header>
      <main className="flex flex-1 justify-center px-6 pb-20 pt-4">
        <div className="w-full max-w-md">
          <Progress step={step} />
          {children}
        </div>
      </main>
    </div>
  );
}

function Card({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-(--radius-card) border border-line bg-raised p-5">
      <h1 className="font-display text-xl tracking-tight text-ink">{title}</h1>
      <p className="mt-1.5 text-[13.5px] text-ink-muted">{description}</p>
      <div className="mt-5">{children}</div>
    </div>
  );
}

// ---- step 1: the org -------------------------------------------------------

const orgSchema = z.object({ name: z.string().min(1, "Give it a name").max(200, "Too long") });
type OrgValues = z.infer<typeof orgSchema>;

function StepOrg({ onDone }: { onDone: (orgID: string) => void }) {
  const qc = useQueryClient();
  const form = useForm<OrgValues>({ resolver: zodResolver(orgSchema), defaultValues: { name: "" } });
  const submit = useSubmit<OrgValues>(form.setError, ["name"]);

  return (
    <Card
      title="Name your organisation"
      description="Everything in Beacon lives inside one. You can create more later."
    >
      <form
        noValidate
        onSubmit={form.handleSubmit(async (v) => {
          await submit.run(async () => {
            const org = await orgsApi.create({ name: v.name });
            await qc.invalidateQueries({ queryKey: keys.orgs });
            onDone(org.id);
          });
        })}
      >
        <FormError error={submit.error} />
        <Field label="Organisation name" error={form.formState.errors.name?.message}>
          {(p) => <Input {...p} autoFocus placeholder="Acme Industries" {...form.register("name")} />}
        </Field>
        <Button type="submit" className="mt-4 w-full" busy={form.formState.isSubmitting}>
          Continue
        </Button>
      </form>
    </Card>
  );
}

// ---- step 2: invite --------------------------------------------------------

const inviteSchema = z.object({
  email: z.string().email("That is not an email address"),
  role: z.enum(["owner", "admin", "member"]),
});
type InviteValues = z.infer<typeof inviteSchema>;

function StepInvite({ orgID, onDone }: { orgID: string; onDone: () => void }) {
  const [added, setAdded] = useState<string[]>([]);
  const form = useForm<InviteValues>({
    resolver: zodResolver(inviteSchema),
    defaultValues: { email: "", role: "member" },
  });
  const submit = useSubmit<InviteValues>(form.setError, ["email", "role"]);
  const role = form.watch("role");

  return (
    <Card
      title="Add your team"
      description="They need an account already — Beacon adds existing users to an org, it does not email invitations."
    >
      <form
        noValidate
        onSubmit={form.handleSubmit(async (v) => {
          const ok = await submit.run(async () => {
            await orgsApi.addMember(orgID, { email: v.email, role: v.role });
          });
          if (ok) {
            setAdded((a) => [...a, v.email]);
            form.reset({ email: "", role: v.role });
          }
        })}
      >
        <FormError error={submit.error} />
        <div className="space-y-4">
          <Field label="Email" error={form.formState.errors.email?.message}>
            {(p) => (
              <Input {...p} type="email" placeholder="teammate@acme.com" {...form.register("email")} />
            )}
          </Field>
          <div>
            <p className="mb-1.5 text-[13px] font-medium text-ink">Role</p>
            <Select
              value={role}
              onValueChange={(v) => form.setValue("role", v as InviteValues["role"])}
              options={[
                { value: "member", label: "Member — can use the board" },
                { value: "admin", label: "Admin — can also manage the org" },
                { value: "owner", label: "Owner — full control" },
              ]}
            />
          </div>
        </div>

        {added.length > 0 && (
          <ul className="mt-4 space-y-1.5">
            {added.map((e) => (
              <li key={e} className="flex items-center gap-2 text-[13px] text-ink-muted">
                <Check className="size-3.5 text-success-text" strokeWidth={3} />
                {e}
              </li>
            ))}
          </ul>
        )}

        <div className="mt-5 flex gap-2">
          <Button type="submit" variant="secondary" busy={form.formState.isSubmitting}>
            Add
          </Button>
          <Button type="button" className="flex-1" onClick={onDone}>
            {added.length > 0 ? "Continue" : "Skip for now"}
          </Button>
        </div>
      </form>
    </Card>
  );
}

// ---- step 3: the project ---------------------------------------------------

const projectSchema = z.object({ name: z.string().min(1, "Give it a name").max(200, "Too long") });
type ProjectValues = z.infer<typeof projectSchema>;

function StepProject({ orgID, onDone }: { orgID: string; onDone: (projectID: string) => void }) {
  const form = useForm<ProjectValues>({
    resolver: zodResolver(projectSchema),
    defaultValues: { name: "" },
  });
  const submit = useSubmit<ProjectValues>(form.setError, ["name"]);

  return (
    <Card
      title="Create your first project"
      description="A project is one board: three columns and the work that moves across them."
    >
      <form
        noValidate
        onSubmit={form.handleSubmit(async (v) => {
          await submit.run(async () => {
            const p = await projectsApi.create(orgID, { name: v.name });
            onDone(p.id);
          });
        })}
      >
        <FormError error={submit.error} />
        <Field label="Project name" error={form.formState.errors.name?.message}>
          {(p) => <Input {...p} autoFocus placeholder="Website relaunch" {...form.register("name")} />}
        </Field>
        <Button type="submit" className="mt-4 w-full" busy={form.formState.isSubmitting}>
          Continue
        </Button>
      </form>
    </Card>
  );
}

// ---- step 4: the first task ------------------------------------------------

const taskSchema = z.object({ title: z.string().min(1, "Give it a title").max(200, "Too long") });
type TaskValues = z.infer<typeof taskSchema>;

function StepTask({
  orgID,
  projectID,
  onDone,
}: {
  orgID: string;
  projectID: string;
  onDone: () => void;
}) {
  const form = useForm<TaskValues>({
    resolver: zodResolver(taskSchema),
    defaultValues: { title: "" },
  });
  const submit = useSubmit<TaskValues>(form.setError, ["title"]);

  return (
    <Card
      title="Put something on the board"
      description="One task is enough. An empty board teaches nobody anything."
    >
      <form
        noValidate
        onSubmit={form.handleSubmit(async (v) => {
          const ok = await submit.run(async () => {
            await tasksApi.create(orgID, projectID, {
              title: v.title,
              status: "todo",
              position: 1000,
            });
          });
          if (ok) onDone();
        })}
      >
        <FormError error={submit.error} />
        <Field label="Task" error={form.formState.errors.title?.message}>
          {(p) => (
            <Input {...p} autoFocus placeholder="Draft the launch plan" {...form.register("title")} />
          )}
        </Field>
        <div className="mt-4 flex gap-2">
          <Button type="button" variant="ghost" onClick={onDone}>
            Skip
          </Button>
          <Button type="submit" className="flex-1" busy={form.formState.isSubmitting}>
            Open my board
          </Button>
        </div>
      </form>
    </Card>
  );
}

// ---- the flow --------------------------------------------------------------

export function Onboarding() {
  const navigate = useNavigate();
  const [step, setStep] = useState(0);
  const [orgID, setOrgID] = useState<string | null>(null);
  const [projectID, setProjectID] = useState<string | null>(null);

  // Resume rather than restart: a user who already has an org should never be
  // asked to make a second one just because they reloaded.
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const existing = await orgsApi.list();
      if (cancelled || existing.items.length === 0) return;
      const first = existing.items[0]!;
      setOrgID(first.id);
      setActiveOrg(first.id);
      setStep((s) => (s === 0 ? 1 : s));
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const finish = () => navigate({ to: "/" });

  return (
    <Frame step={step}>
      {step === 0 && (
        <StepOrg
          onDone={(id) => {
            setOrgID(id);
            setActiveOrg(id);
            setStep(1);
          }}
        />
      )}
      {step === 1 && orgID && <StepInvite orgID={orgID} onDone={() => setStep(2)} />}
      {step === 2 && orgID && (
        <StepProject
          orgID={orgID}
          onDone={(id) => {
            setProjectID(id);
            setStep(3);
          }}
        />
      )}
      {step === 3 && orgID && projectID && (
        <StepTask orgID={orgID} projectID={projectID} onDone={finish} />
      )}
    </Frame>
  );
}
