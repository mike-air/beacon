/**
 * Everything signed-in hangs off one question: which org?
 *
 * This resolves it once — from the URL if it names one, else the last one
 * used, else the first the user belongs to — and sends a user with none to
 * onboarding. Every screen below can then assume an org exists.
 */
import { Outlet, useNavigate } from "@tanstack/react-router";
import { createContext, useContext, useEffect, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useMe, useOrgs, invalidateOrg } from "@/api/queries";
import { connectSse } from "@/api/sse";
import { eventsPath } from "@/api/endpoints";
import { getActiveOrg, setActiveOrg } from "./active-org";
import { AppShell } from "@/components/layout/app-shell";
import { useAuth } from "@/features/auth/auth-context";
import { Skeleton } from "@/components/ui/skeleton";
import { CommandPalette } from "@/features/search/command-palette";
import type { OrgWithRole, User } from "@/api/types";

export type OrgContext = { org: OrgWithRole; user: User | undefined };

const Ctx = createContext<OrgContext | null>(null);

/**
 * The resolved org, for any screen inside the shell.
 *
 * Children read this rather than mounting their own OrgGate — nesting the
 * gate renders the whole shell once per level, which is how this app briefly
 * shipped with two header bars stacked on top of each other.
 */
export function useOrgContext(): OrgContext {
  const v = useContext(Ctx);
  if (!v) throw new Error("useOrgContext must be used inside <OrgGate>");
  return v;
}

export function OrgGate() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const { signOut } = useAuth();
  const meQuery = useMe();
  const orgsQuery = useOrgs();
  const [activeID, setActiveID] = useState<string | null>(getActiveOrg);
  const [connected, setConnected] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);

  const orgs = useMemo(() => orgsQuery.data?.items ?? [], [orgsQuery.data]);
  const active = orgs.find((o) => o.id === activeID) ?? orgs[0];

  // A stored org the user no longer belongs to must not strand them.
  useEffect(() => {
    if (active && active.id !== activeID) {
      setActiveID(active.id);
      setActiveOrg(active.id);
    }
  }, [active, activeID]);

  // No orgs at all means this account has not been set up yet.
  useEffect(() => {
    if (orgsQuery.isSuccess && orgs.length === 0) void navigate({ to: "/welcome" });
  }, [orgsQuery.isSuccess, orgs.length, navigate]);

  // ⌘K from anywhere, and Escape is handled by the dialog itself.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key.toLowerCase() === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setSearchOpen((v) => !v);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  // One stream per org. Every event invalidates rather than patching: the
  // server drops events for slow consumers, so the event is a hint to refetch.
  useEffect(() => {
    if (!active) return;
    const orgID = active.id;
    return connectSse({
      path: eventsPath(orgID),
      onStatusChange: setConnected,
      onEvent: () => invalidateOrg(qc, orgID),
    });
  }, [active, qc]);

  if (orgsQuery.isPending || meQuery.isPending) {
    return (
      <div className="min-h-screen bg-sunken p-6">
        <Skeleton className="h-13 w-full rounded-(--radius-card)" />
        <div className="mt-4 grid gap-3 sm:grid-cols-3">
          <Skeleton className="h-64" />
          <Skeleton className="h-64" />
          <Skeleton className="h-64" />
        </div>
      </div>
    );
  }

  if (!active) return null; // the effect above is navigating to /welcome

  return (
    <AppShell
      user={meQuery.data}
      orgs={orgs}
      activeOrgID={active.id}
      connected={connected}
      onSelectOrg={(id) => {
        setActiveID(id);
        setActiveOrg(id);
        void navigate({ to: "/" });
      }}
      onCreateOrg={() => void navigate({ to: "/welcome" })}
      onSignOut={signOut}
      onOpenSearch={() => setSearchOpen(true)}
    >
      <Ctx.Provider value={{ org: active, user: meQuery.data }}>
        <Outlet />
        <CommandPalette orgID={active.id} open={searchOpen} onOpenChange={setSearchOpen} />
      </Ctx.Provider>
    </AppShell>
  );
}
