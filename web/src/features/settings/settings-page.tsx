import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { me } from "@/api/endpoints";
import { keys, usePreferences } from "@/api/queries";
import { isAdmin } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import { useOrgContext } from "@/features/org/org-gate";
import { WebhooksSection } from "@/features/webhooks/webhooks-section";

/**
 * Radix reserves the empty string for "nothing is selected", so an option
 * cannot use it as a value — the trigger renders blank instead of showing the
 * label. Beacon's API, meanwhile, uses "" to mean "no preference, fall back to
 * the cascade". AUTO is the sentinel between the two, translated at the edges.
 */
const AUTO = "__auto__";

const LOCALES = [
  { value: AUTO, label: "Follow my browser" },
  { value: "en", label: "English" },
  { value: "de", label: "Deutsch" },
  { value: "fr", label: "Français" },
  { value: "pt-BR", label: "Português (Brasil)" },
];

/** Whatever the browser knows, plus UTC. Beacon stores IANA names, not offsets. */
const TIMEZONES = [
  { value: AUTO, label: "Follow my device" },
  { value: "UTC", label: "UTC" },
  { value: "Africa/Accra", label: "Africa/Accra" },
  { value: "Europe/London", label: "Europe/London" },
  { value: "Europe/Berlin", label: "Europe/Berlin" },
  { value: "America/New_York", label: "America/New_York" },
  { value: "Asia/Tokyo", label: "Asia/Tokyo" },
];

/** "" (the API's "no preference") <-> AUTO (what the Select can hold). */
const toSelect = (v: string) => (v === "" ? AUTO : v);
const toApi = (v: string) => (v === AUTO ? "" : v);

function Row({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-2 border-b border-line py-4 last:border-0 sm:flex-row sm:items-center">
      <div className="sm:w-56">
        <p className="text-ui font-medium text-ink">{label}</p>
        {hint && <p className="mt-0.5 text-caption text-ink-muted">{hint}</p>}
      </div>
      <div className="sm:max-w-xs sm:flex-1">{children}</div>
    </div>
  );
}

export function SettingsPage() {
  const { org } = useOrgContext();
  const prefs = usePreferences();
  const qc = useQueryClient();
  const toast = useToast();
  const [locale, setLocale] = useState<string | null>(null);
  const [timezone, setTimezone] = useState<string | null>(null);

  const save = useMutation({
    mutationFn: () =>
      me.setPreferences({
        locale: toApi(locale ?? prefs.data?.locale ?? ""),
        timezone: toApi(timezone ?? prefs.data?.timezone ?? ""),
      }),
    onSuccess: (data) => {
      qc.setQueryData(keys.preferences, data);
      setLocale(null);
      setTimezone(null);
      toast({ title: "Preferences saved", tone: "success" });
    },
    onError: () =>
      toast({ title: "Could not save", description: "Try again.", tone: "danger" }),
  });

  const dirty = locale !== null || timezone !== null;

  return (
    <div className="mx-auto max-w-3xl px-4 py-7">
      <h1 className="font-display text-xl tracking-tight text-ink">Settings</h1>

      <section className="mt-6">
        <h2 className="mb-1 font-display text-ui tracking-wide text-ink-muted">Organisation</h2>
        <div className="rounded-(--radius-card) border border-line bg-raised px-4">
          <Row label="Name">
            <p className="text-sm text-ink">{org.name}</p>
          </Row>
          <Row label="Slug" hint="Used in URLs.">
            <p className="font-mono text-ui text-ink-muted">{org.slug}</p>
          </Row>
          <Row label="Your role">
            <span className="flex items-center gap-2">
              <Badge tone={isAdmin(org.role) ? "accent" : "neutral"}>{org.role}</Badge>
              {!isAdmin(org.role) && (
                <span className="text-caption text-ink-muted">
                  Members cannot change organisation settings.
                </span>
              )}
            </span>
          </Row>
        </div>
      </section>

      <section className="mt-8">
        <h2 className="mb-1 font-display text-ui tracking-wide text-ink-muted">Webhooks</h2>
        <WebhooksSection orgID={org.id} role={org.role} />
      </section>

      <section className="mt-8">
        <h2 className="mb-1 font-display text-ui tracking-wide text-ink-muted">You</h2>
        <div className="rounded-(--radius-card) border border-line bg-raised px-4">
          {prefs.isPending && (
            <div className="space-y-3 py-4">
              <Skeleton className="h-9" />
              <Skeleton className="h-9" />
            </div>
          )}

          {prefs.isSuccess && (
            <>
              <Row label="Language" hint="Empty follows your browser.">
                <Select
                  value={toSelect(locale ?? prefs.data.locale)}
                  onValueChange={setLocale}
                  options={LOCALES}
                />
              </Row>
              <Row label="Time zone" hint="An IANA name, never an offset.">
                <Select
                  value={toSelect(timezone ?? prefs.data.timezone)}
                  onValueChange={setTimezone}
                  options={TIMEZONES}
                />
              </Row>
              {/* The server resolves a cascade — stored preference, then org
                  default, then Accept-Language. Showing what it actually chose
                  makes an empty preference legible instead of mysterious. */}
              <Row label="Right now" hint="What the server resolved for this request.">
                <div className="space-y-0.5 font-mono text-caption text-ink-muted">
                  <p>{prefs.data.resolved_locale} · {prefs.data.now_local}</p>
                  <p>{prefs.data.greeting} · {prefs.data.example_price}</p>
                </div>
              </Row>
            </>
          )}
        </div>

        {dirty && (
          <div className="mt-3 flex justify-end gap-2">
            <Button
              variant="ghost"
              onClick={() => {
                setLocale(null);
                setTimezone(null);
              }}
            >
              Discard
            </Button>
            <Button busy={save.isPending} onClick={() => save.mutate()}>
              Save preferences
            </Button>
          </div>
        )}
      </section>
    </div>
  );
}
