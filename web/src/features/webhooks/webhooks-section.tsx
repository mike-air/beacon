import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Copy, Plus, Trash2, Webhook as WebhookIcon } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { webhooks as api } from "@/api/endpoints";
import { keys, useWebhooks } from "@/api/queries";
import { isAdmin, type Role, type Webhook } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Field, Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import { FormError } from "@/features/auth/form-error";
import { useSubmit } from "@/features/auth/use-submit";

/** The events Beacon actually publishes (internal/http/notify.go). */
const EVENTS = ["task.created", "task.updated", "task.deleted"] as const;

const schema = z.object({
  url: z.string().url("That is not a URL").max(2000, "Too long"),
});
type Values = z.infer<typeof schema>;

/**
 * The secret is returned in full exactly once, in the create response, and
 * never again. So it is shown once, here, with that fact stated — a client
 * that quietly drops it leaves the user unable to verify a single delivery.
 */
function SecretOnce({ secret, onDone }: { secret: string; onDone: () => void }) {
  const toast = useToast();
  return (
    <div className="rounded-(--radius-ctl) bg-warning-subtle p-3.5">
      <p className="text-[13px] font-medium text-warning-text">
        Copy this signing secret now
      </p>
      <p className="mt-1 text-[12.5px] text-ink-muted">
        It is shown once. Beacon cannot show it again.
      </p>
      <div className="mt-2.5 flex items-center gap-2">
        <code className="min-w-0 flex-1 truncate rounded bg-page px-2.5 py-1.5 font-mono text-[12px] text-ink">
          {secret}
        </code>
        <Button
          size="sm"
          variant="secondary"
          onClick={() => {
            void navigator.clipboard.writeText(secret);
            toast({ title: "Secret copied", tone: "success" });
          }}
        >
          <Copy className="size-3.5" />
          Copy
        </Button>
      </div>
      <Button size="sm" variant="ghost" className="mt-2" onClick={onDone}>
        I have saved it
      </Button>
    </div>
  );
}

function RegisterDialog({
  orgID,
  open,
  onOpenChange,
}: {
  orgID: string;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const qc = useQueryClient();
  const [selected, setSelected] = useState<string[]>([...EVENTS]);
  const [created, setCreated] = useState<Webhook | null>(null);
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { url: "" } });
  const submit = useSubmit<Values>(form.setError, ["url"]);

  const register = useMutation({
    mutationFn: (v: Values) => api.register(orgID, { url: v.url, events: selected }),
    onSuccess: (w) => {
      setCreated(w);
      form.reset();
      void qc.invalidateQueries({ queryKey: keys.webhooks(orgID) });
    },
  });

  function close() {
    setCreated(null);
    onOpenChange(false);
  }

  return (
    <Dialog open={open} onOpenChange={(v) => (v ? onOpenChange(true) : close())}>
      <DialogContent
        title="Register a webhook"
        description="Beacon POSTs to this URL when the events you pick happen."
      >
        {created?.secret ? (
          <SecretOnce secret={created.secret} onDone={close} />
        ) : (
          <form
            noValidate
            onSubmit={form.handleSubmit(async (v) => {
              await submit.run(async () => {
                await register.mutateAsync(v);
              });
            })}
          >
            <FormError error={submit.error} />
            <Field label="Endpoint URL" error={form.formState.errors.url?.message}>
              {(p) => (
                <Input {...p} autoFocus placeholder="https://example.com/hooks/beacon" {...form.register("url")} />
              )}
            </Field>
            <fieldset className="mt-4">
              <legend className="mb-2 text-[13px] font-medium text-ink">Events</legend>
              <div className="space-y-2">
                {EVENTS.map((e) => (
                  <Checkbox
                    key={e}
                    label={e}
                    checked={selected.includes(e)}
                    onCheckedChange={(v) =>
                      setSelected((prev) => (v ? [...prev, e] : prev.filter((x) => x !== e)))
                    }
                  />
                ))}
              </div>
              {selected.length === 0 && (
                <p className="mt-2 text-[12px] text-ink-muted">
                  None selected means every event.
                </p>
              )}
            </fieldset>
            <div className="mt-4 flex justify-end gap-2">
              <Button variant="ghost" type="button" onClick={close}>
                Cancel
              </Button>
              <Button type="submit" busy={form.formState.isSubmitting}>
                Register
              </Button>
            </div>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}

export function WebhooksSection({ orgID, role }: { orgID: string; role: Role }) {
  const canAdmin = isAdmin(role);
  const query = useWebhooks(orgID, canAdmin);
  const qc = useQueryClient();
  const toast = useToast();
  const [registering, setRegistering] = useState(false);

  const remove = useMutation({
    mutationFn: (id: string) => api.remove(orgID, id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: keys.webhooks(orgID) });
      toast({ title: "Webhook deleted", tone: "success" });
    },
  });

  // Members get no request fired at all — the endpoint is admin-only, and
  // querying it just to render a 403 is a round trip that teaches nobody
  // anything.
  if (!canAdmin) {
    return (
      <p className="rounded-(--radius-card) border border-line bg-raised px-4 py-3.5 text-[13px] text-ink-muted">
        Webhooks are managed by owners and admins.
      </p>
    );
  }

  return (
    <div>
      <div className="mb-2 flex items-center justify-end">
        <Button size="sm" onClick={() => setRegistering(true)}>
          <Plus className="size-3.5" />
          Register webhook
        </Button>
      </div>

      {query.isPending && <Skeleton className="h-20 rounded-(--radius-card)" />}

      {query.isSuccess && query.data.items.length === 0 && (
        <p className="rounded-(--radius-card) border border-line bg-raised px-4 py-3.5 text-[13px] text-ink-muted">
          No webhooks yet. Register one to get task events POSTed to your own service.
        </p>
      )}

      {query.isSuccess && query.data.items.length > 0 && (
        <ul className="space-y-2">
          {query.data.items.map((w) => (
            <li
              key={w.id}
              className="flex items-start gap-3 rounded-(--radius-card) border border-line bg-raised px-4 py-3"
            >
              <WebhookIcon aria-hidden className="mt-0.5 size-4 shrink-0 text-ink-faint" />
              <div className="min-w-0 flex-1">
                <p className="truncate font-mono text-[12.5px] text-ink">{w.url}</p>
                <div className="mt-1.5 flex flex-wrap gap-1.5">
                  {w.events.length === 0 ? (
                    <Badge>all events</Badge>
                  ) : (
                    w.events.map((e) => (
                      <Badge key={e} tone="accent">
                        {e}
                      </Badge>
                    ))
                  )}
                  <Badge tone={w.active ? "success" : "neutral"}>
                    {w.active ? "active" : "paused"}
                  </Badge>
                </div>
              </div>
              <Button
                variant="ghost"
                size="sm"
                aria-label={`Delete webhook ${w.url}`}
                className="text-danger-text"
                busy={remove.isPending}
                onClick={() => remove.mutate(w.id)}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </li>
          ))}
        </ul>
      )}

      <RegisterDialog orgID={orgID} open={registering} onOpenChange={setRegistering} />
    </div>
  );
}
