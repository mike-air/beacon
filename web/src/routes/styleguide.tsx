import { useState } from "react";
import { Plus, Search, Trash2, Users } from "lucide-react";
import { tokenValues, type TokenName } from "@/design/tokens.gen";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogTrigger } from "@/components/ui/dialog";
import { Field, Input } from "@/components/ui/input";
import { MenuContent, MenuItem, MenuLabel, MenuRoot, MenuSeparator, MenuTrigger } from "@/components/ui/menu";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, Td, Th, Tr } from "@/components/ui/table";
import { Tooltip } from "@/components/ui/tooltip";
import { useToast } from "@/components/ui/toast";
import { Wordmark } from "@/components/ui/logo";
import { ThemeToggle } from "@/components/theme-toggle";

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="border-b border-line py-8">
      <h2 className="mb-4 font-display text-[13px] tracking-wide text-ink-muted">{title}</h2>
      {children}
    </section>
  );
}

function Swatch({ name }: { name: TokenName }) {
  const t = tokenValues[name];
  return (
    <div className="w-40 overflow-hidden rounded-(--radius-card) border border-line">
      <div className="h-12" style={{ background: `var(--${name})` }} />
      <div className="border-t border-line bg-raised px-2.5 py-1.5">
        <div className="font-mono text-[11px] text-ink">{name}</div>
        <div className="mt-0.5 text-[10.5px] leading-snug text-ink-faint">{t.why}</div>
      </div>
    </div>
  );
}

const groups: { label: string; names: TokenName[] }[] = [
  { label: "Surfaces", names: ["bg-sunken", "bg-page", "bg-raised", "bg-overlay", "bg-well"] },
  { label: "Ink", names: ["ink", "ink-muted", "ink-faint", "ink-inverse", "on-accent"] },
  { label: "Lines", names: ["line", "line-strong", "ring"] },
  { label: "Accent — poster purple", names: ["accent", "accent-hover", "accent-active", "accent-text", "accent-subtle"] },
  { label: "Volt — the signature, never semantic", names: ["volt", "on-volt", "volt-text", "volt-subtle"] },
  { label: "Status — poster teal + orange, plus red", names: ["success", "success-text", "success-subtle", "warning", "warning-text", "warning-subtle", "danger", "danger-text", "danger-subtle"] },
];

export function Styleguide() {
  const toast = useToast();
  const [role, setRole] = useState("member");
  const [email, setEmail] = useState("");

  return (
    <div className="min-h-screen bg-page">
      <header className="sticky top-0 z-40 border-b border-line bg-page/85 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-5xl items-center gap-3 px-6">
          <Wordmark live />
          <Badge tone="accent">styleguide</Badge>
          <div className="flex-1" />
          <span className="flex items-center gap-1.5 font-mono text-[11.5px] text-ink-muted">
            <span className="size-2 rounded-full bg-volt animate-beacon" />
            LIVE
          </span>
          <ThemeToggle />
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-6 pb-24">
        <div className="py-10">
          <h1 className="font-display text-3xl tracking-tight text-ink">The design system</h1>
          <p className="mt-2 max-w-2xl text-ink-muted">
            Every color here comes from <code className="font-mono text-[12.5px] text-accent-text">tokens.source.ts</code>.
            Nothing in this page carries a hex code, and the theme switch re-resolves all of it.
          </p>
        </div>

        {groups.map((g) => (
          <Section key={g.label} title={g.label}>
            <div className="flex flex-wrap gap-2.5">
              {g.names.map((n) => (
                <Swatch key={n} name={n} />
              ))}
            </div>
          </Section>
        ))}

        <Section title="Type">
          <div className="space-y-3">
            <p className="font-display text-3xl tracking-tight text-ink">Archivo Black — display</p>
            <p className="text-ink">Space Grotesk — body and UI, 14px, the working weight.</p>
            <p className="text-ink-muted">Space Grotesk muted — descriptions and secondary lines.</p>
            <p className="font-mono text-[13px] text-ink-muted">JetBrains Mono — 4 tasks · TSK-1042 · ⌘K</p>
          </div>
        </Section>

        <Section title="Buttons">
          <div className="flex flex-wrap items-center gap-2.5">
            <Button><Plus className="size-3.5" />New task</Button>
            <Button variant="secondary">Secondary</Button>
            <Button variant="ghost">Ghost</Button>
            <Button variant="danger"><Trash2 className="size-3.5" />Delete</Button>
            <Button busy>Saving</Button>
            <Button disabled>Disabled</Button>
            <Button size="sm" variant="secondary">Small</Button>
            <Button size="lg">Large</Button>
          </div>
        </Section>

        <Section title="Badges">
          <div className="flex flex-wrap items-center gap-2">
            <Badge>neutral</Badge>
            <Badge tone="accent">api</Badge>
            <Badge tone="success">done</Badge>
            <Badge tone="warning">rate limited</Badge>
            <Badge tone="danger">failed</Badge>
            <Badge tone="volt">live</Badge>
          </div>
        </Section>

        <Section title="Forms">
          <div className="grid max-w-md gap-4">
            <Field label="Email" hint="Invites go out immediately.">
              {(p) => (
                <Input
                  {...p}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="teammate@acme.com"
                />
              )}
            </Field>
            <Field label="Organisation slug" error="Already taken.">
              {(p) => <Input {...p} defaultValue="acme" />}
            </Field>
            <div>
              <p className="mb-1.5 text-[13px] font-medium text-ink">Role</p>
              <Select
                value={role}
                onValueChange={setRole}
                options={[
                  { value: "owner", label: "Owner" },
                  { value: "admin", label: "Admin" },
                  { value: "member", label: "Member" },
                ]}
              />
            </div>
            <Checkbox label="Notify me when someone comments" defaultChecked />
          </div>
        </Section>

        <Section title="Overlays">
          <div className="flex flex-wrap items-center gap-2.5">
            <Dialog>
              <DialogTrigger asChild>
                <Button variant="secondary">Open dialog</Button>
              </DialogTrigger>
              <DialogContent
                title="Delete project"
                description="This removes every task in it. It cannot be undone."
              >
                <div className="flex justify-end gap-2">
                  <Button variant="ghost">Cancel</Button>
                  <Button variant="danger">Delete project</Button>
                </div>
              </DialogContent>
            </Dialog>

            <Popover>
              <PopoverTrigger asChild>
                <Button variant="secondary"><Search className="size-3.5" />Filter</Button>
              </PopoverTrigger>
              <PopoverContent>
                <p className="mb-2 text-[13px] font-medium text-ink">Filter tasks</p>
                <div className="space-y-2">
                  <Checkbox label="Assigned to me" />
                  <Checkbox label="Has attachments" />
                </div>
              </PopoverContent>
            </Popover>

            <MenuRoot>
              <MenuTrigger asChild>
                <Button variant="secondary"><Users className="size-3.5" />Member</Button>
              </MenuTrigger>
              <MenuContent align="start">
                <MenuLabel>Change role</MenuLabel>
                <MenuItem>Make admin</MenuItem>
                <MenuItem>Make member</MenuItem>
                <MenuSeparator />
                <MenuItem destructive>Remove from org</MenuItem>
              </MenuContent>
            </MenuRoot>

            <Tooltip content="Keyboard: C">
              <Button variant="ghost">Hover me</Button>
            </Tooltip>

            <Button
              variant="secondary"
              onClick={() =>
                toast({
                  title: "Rate limited",
                  description: "Too many requests. Retrying in 2s.",
                  tone: "warning",
                })
              }
            >
              Toast
            </Button>
          </div>
        </Section>

        <Section title="Data">
          <Table>
            <thead>
              <tr>
                <Th>Member</Th>
                <Th>Role</Th>
                <Th>Status</Th>
              </tr>
            </thead>
            <tbody>
              {[
                { n: "Michael Anderson", r: "owner", s: "active" as const },
                { n: "Efua Kusi", r: "admin", s: "active" as const },
                { n: "Sam Otoo", r: "member", s: "invited" as const },
              ].map((m) => (
                <Tr key={m.n}>
                  <Td>
                    <span className="flex items-center gap-2">
                      <Avatar name={m.n} size="sm" />
                      {m.n}
                    </span>
                  </Td>
                  <Td className="font-mono text-[12.5px] text-ink-muted">{m.r}</Td>
                  <Td>
                    <Badge tone={m.s === "active" ? "success" : "neutral"}>{m.s}</Badge>
                  </Td>
                </Tr>
              ))}
            </tbody>
          </Table>
        </Section>

        <Section title="Loading">
          <div className="max-w-sm space-y-2 rounded-(--radius-card) border border-line bg-raised p-3.5">
            <Skeleton className="h-4 w-2/3" />
            <Skeleton className="h-3 w-1/3" />
            <Skeleton className="h-3 w-1/2" />
          </div>
        </Section>

        <Section title="Motion — subtle, and it stops for prefers-reduced-motion">
          <div className="flex flex-wrap items-center gap-6 text-[13px] text-ink-muted">
            <span className="flex items-center gap-2">
              <span className="size-2.5 rounded-full bg-volt animate-beacon" /> live pulse
            </span>
            <span className="flex items-center gap-2">
              <span className="rounded-md px-2 py-1 animate-volt-wash">row updated by a teammate</span>
            </span>
          </div>
        </Section>
      </main>
    </div>
  );
}
