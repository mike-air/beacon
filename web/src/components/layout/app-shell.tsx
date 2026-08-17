/**
 * The signed-in frame.
 *
 * One horizontal bar and nothing down the side. A left rail costs 200-260px
 * of width on every screen to hold four links, and a board is the one view
 * that actually wants that width.
 *
 * The wordmark's beam is wired to the live SSE connection: when the stream
 * drops, the beam stops. That is the only connection indicator in the app and
 * it is telling the truth rather than always claiming "Live".
 */
import { Link, useRouterState } from "@tanstack/react-router";
import { LogOut, Search, Settings, User as UserIcon } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Logo } from "@/components/ui/logo";
import {
  MenuContent,
  MenuItem,
  MenuLabel,
  MenuRoot,
  MenuSeparator,
  MenuTrigger,
} from "@/components/ui/menu";
import { ThemeToggle } from "@/components/theme-toggle";
import { cn } from "@/lib/cn";
import type { OrgWithRole, User } from "@/api/types";
import { OrgSwitcher } from "@/features/org/org-switcher";

export function AppShell({
  user,
  orgs,
  activeOrgID,
  connected,
  onSelectOrg,
  onCreateOrg,
  onSignOut,
  onOpenSearch,
  children,
}: {
  user: User | undefined;
  orgs: OrgWithRole[];
  activeOrgID: string | undefined;
  connected: boolean;
  onSelectOrg: (id: string) => void;
  onCreateOrg: () => void;
  onSignOut: () => void;
  onOpenSearch: () => void;
  children: React.ReactNode;
}) {
  const path = useRouterState({ select: (s) => s.location.pathname });

  const tabs = [
    { to: "/", label: "Board", match: (p: string) => p === "/" || p.startsWith("/projects") },
    { to: "/members", label: "Members", match: (p: string) => p.startsWith("/members") },
    { to: "/settings", label: "Settings", match: (p: string) => p.startsWith("/settings") },
  ] as const;

  return (
    <div className="min-h-screen bg-sunken">
      <header className="sticky top-0 z-40 border-b border-line bg-page/85 backdrop-blur">
        <div className="flex h-13 items-center gap-2 px-4">
          <Link to="/" aria-label="Beacon home" className="flex items-center gap-2 pr-1">
            <Logo live={connected} size={20} />
            <span className="hidden font-display text-title tracking-tight text-ink sm:inline">
              BEACON
            </span>
          </Link>

          <span className="text-ink-faint" aria-hidden>
            /
          </span>

          <OrgSwitcher
            orgs={orgs}
            activeID={activeOrgID}
            onSelect={onSelectOrg}
            onCreate={onCreateOrg}
          />

          <div className="flex-1" />

          <Button
            variant="secondary"
            size="sm"
            onClick={onOpenSearch}
            className="gap-2 text-ink-muted"
          >
            <Search className="size-3.5" />
            <span className="hidden sm:inline">Search</span>
            <kbd className="ml-1 hidden rounded bg-well px-1.5 py-px font-mono text-micro text-ink-faint sm:inline">
              ⌘K
            </kbd>
          </Button>

          <ThemeToggle />

          <MenuRoot>
            <MenuTrigger asChild>
              <button
                aria-label="Account"
                className="rounded-full transition-opacity hover:opacity-85"
              >
                <Avatar name={user?.name || user?.email || "?"} size="md" />
              </button>
            </MenuTrigger>
            <MenuContent align="end" className="min-w-52">
              <MenuLabel>{user?.email ?? "Signed in"}</MenuLabel>
              <MenuSeparator />
              <MenuItem asChild>
                <Link to="/settings">
                  <UserIcon className="size-3.5 text-ink-faint" />
                  Your preferences
                </Link>
              </MenuItem>
              <MenuItem asChild>
                <Link to="/settings">
                  <Settings className="size-3.5 text-ink-faint" />
                  Organisation settings
                </Link>
              </MenuItem>
              <MenuSeparator />
              <MenuItem destructive onSelect={onSignOut}>
                <LogOut className="size-3.5" />
                Sign out
              </MenuItem>
            </MenuContent>
          </MenuRoot>
        </div>

        <nav className="flex gap-1 px-4" aria-label="Sections">
          {tabs.map((t) => {
            const active = t.match(path);
            return (
              <Link
                key={t.to}
                to={t.to}
                className={cn(
                  "-mb-px border-b-2 px-2.5 pb-2 pt-0.5 text-ui transition-colors",
                  active
                    ? "border-accent font-medium text-ink"
                    : "border-transparent text-ink-muted hover:text-ink",
                )}
              >
                {t.label}
              </Link>
            );
          })}
        </nav>
      </header>

      <main>{children}</main>
    </div>
  );
}
