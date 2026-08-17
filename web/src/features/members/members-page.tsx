import { zodResolver } from "@hookform/resolvers/zod";
import { UserPlus } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { useAddMember, useMembers } from "@/api/queries";
import { isAdmin } from "@/api/types";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Field, Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, Td, Th, Tr } from "@/components/ui/table";
import { FormError } from "@/features/auth/form-error";
import { useSubmit } from "@/features/auth/use-submit";
import { useOrgContext } from "@/features/org/org-gate";

const schema = z.object({
  email: z.string().email("That is not an email address"),
  role: z.enum(["owner", "admin", "member"]),
});
type Values = z.infer<typeof schema>;

function AddMemberDialog({
  orgID,
  open,
  onOpenChange,
}: {
  orgID: string;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const add = useAddMember(orgID);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { email: "", role: "member" },
  });
  const submit = useSubmit<Values>(form.setError, ["email", "role"]);
  const role = form.watch("role");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        title="Add a member"
        description="They need a Beacon account already — this adds an existing user to the org."
      >
        <form
          noValidate
          onSubmit={form.handleSubmit(async (v) => {
            const ok = await submit.run(async () => {
              await add.mutateAsync(v);
            });
            if (ok) {
              form.reset({ email: "", role: v.role });
              onOpenChange(false);
            }
          })}
        >
          <FormError error={submit.error} />
          <div className="space-y-4">
            <Field label="Email" error={form.formState.errors.email?.message}>
              {(p) => (
                <Input {...p} type="email" autoFocus placeholder="teammate@acme.com" {...form.register("email")} />
              )}
            </Field>
            <div>
              <p className="mb-1.5 text-ui font-medium text-ink">Role</p>
              <Select
                value={role}
                onValueChange={(v) => form.setValue("role", v as Values["role"])}
                options={[
                  { value: "member", label: "Member — can use the board" },
                  { value: "admin", label: "Admin — can also manage the org" },
                  { value: "owner", label: "Owner — full control" },
                ]}
              />
            </div>
          </div>
          <div className="mt-4 flex justify-end gap-2">
            <Button variant="ghost" type="button" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" busy={form.formState.isSubmitting}>
              Add member
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function MembersPage() {
  const { org } = useOrgContext();
  const query = useMembers(org.id);
  const [adding, setAdding] = useState(false);
  const canAdmin = isAdmin(org.role);

  return (
    <div className="mx-auto max-w-4xl px-4 py-7">
      <div className="mb-5 flex items-center gap-3">
        <h1 className="font-display text-xl tracking-tight text-ink">Members</h1>
        <div className="flex-1" />
        {/* The server enforces this; hiding it stops a user being offered a
            button that can only ever give them a 403. */}
        {canAdmin && (
          <Button onClick={() => setAdding(true)}>
            <UserPlus className="size-3.5" />
            Add member
          </Button>
        )}
      </div>

      {query.isPending && <Skeleton className="h-40 rounded-(--radius-card)" />}

      {query.isSuccess && (
        <div className="rounded-(--radius-card) border border-line bg-raised">
          <Table>
            <thead>
              <tr>
                <Th>Member</Th>
                <Th>Email</Th>
                <Th>Role</Th>
              </tr>
            </thead>
            <tbody>
              {query.data.items.map((m) => (
                <Tr key={m.user_id}>
                  <Td>
                    <span className="flex items-center gap-2">
                      <Avatar name={m.name || m.email} size="sm" />
                      {m.name || "—"}
                    </span>
                  </Td>
                  <Td className="text-ink-muted">{m.email}</Td>
                  <Td>
                    <Badge tone={m.role === "member" ? "neutral" : "accent"}>{m.role}</Badge>
                  </Td>
                </Tr>
              ))}
            </tbody>
          </Table>
        </div>
      )}

      <AddMemberDialog orgID={org.id} open={adding} onOpenChange={setAdding} />
    </div>
  );
}
