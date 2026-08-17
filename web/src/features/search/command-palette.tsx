/**
 * ⌘K.
 *
 * Three things worth noticing:
 *
 * Typing is debounced, and the request is aborted when the query moves on.
 * Without both, a fast typist fires one search per keystroke and the answers
 * arrive out of order — so the results for "beac" can land after the results
 * for "beacon" and overwrite them. TanStack Query's key handles the ordering;
 * the abort signal stops paying for the ones nobody wants.
 *
 * A query under two characters is not sent at all, because the server rejects
 * it with a 422. Showing a hint is better than showing an error the user could
 * not have avoided.
 *
 * The engine that answered is shown. Beacon falls back from Meilisearch to
 * Postgres when Meili is down, and the results are noticeably worse. A client
 * that hides that difference is hiding a live incident from the person best
 * placed to report it.
 */
import { useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { CornerDownLeft, FileText, Folder, MessageSquare, Search } from "lucide-react";
import { useEffect, useState } from "react";
import { orgs as orgsApi } from "@/api/endpoints";
import { keys, shouldRetry } from "@/api/queries";
import type { SearchHit } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Highlight } from "@/components/ui/highlight";
import { EmptySearch } from "@/components/ui/illustration";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/cn";
import { useDebounced } from "@/lib/use-debounced";

const ICONS: Record<string, typeof FileText> = {
  project: Folder,
  task: FileText,
  comment: MessageSquare,
};

export function CommandPalette({
  orgID,
  open,
  onOpenChange,
}: {
  orgID: string;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const navigate = useNavigate();
  const [raw, setRaw] = useState("");
  const [cursor, setCursor] = useState(0);
  const q = useDebounced(raw.trim(), 200);

  useEffect(() => {
    if (!open) {
      setRaw("");
      setCursor(0);
    }
  }, [open]);

  const query = useQuery({
    queryKey: keys.search(orgID, q),
    queryFn: ({ signal }) => orgsApi.search(orgID, q, { limit: 20 }, { signal }),
    // The server's own floor. Sending a shorter query buys a guaranteed 422.
    enabled: open && q.length >= 2,
    retry: shouldRetry,
  });

  const hits = query.data?.hits ?? [];

  function go(hit: SearchHit) {
    onOpenChange(false);
    if (hit.kind === "project") {
      void navigate({ to: "/projects/$projectID", params: { projectID: hit.id }, search: {} });
    } else {
      // Tasks and comments do not carry their project id in a search hit, so
      // the palette can only land you on the list. Adding project_id to the
      // hit is a server change worth making.
      void navigate({ to: "/" });
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent title="Search" description="Find anything in this organisation.">
        <div className="flex items-center gap-2">
          <Search aria-hidden className="size-4 shrink-0 text-ink-faint" />
          <Input
            autoFocus
            value={raw}
            aria-label="Search"
            placeholder="Search projects, tasks and comments"
            onChange={(e) => {
              setRaw(e.target.value);
              setCursor(0);
            }}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown") {
                e.preventDefault();
                setCursor((c) => Math.min(c + 1, hits.length - 1));
              } else if (e.key === "ArrowUp") {
                e.preventDefault();
                setCursor((c) => Math.max(c - 1, 0));
              } else if (e.key === "Enter") {
                const hit = hits[cursor];
                if (hit) go(hit);
              }
            }}
          />
        </div>

        <div className="mt-3 max-h-80 overflow-y-auto">
          {raw.trim().length > 0 && raw.trim().length < 2 && (
            <p className="px-1 py-6 text-center text-ui text-ink-faint">
              Two characters or more.
            </p>
          )}

          {query.isPending && q.length >= 2 && (
            <div className="space-y-1.5">
              <Skeleton className="h-11" />
              <Skeleton className="h-11" />
            </div>
          )}

          {query.isSuccess && hits.length === 0 && (
            <div className="py-2">
              <div className="mx-auto w-32 opacity-90">
                <EmptySearch />
              </div>
              <p className="mt-2 text-center text-ui text-ink-muted">
                Nothing matched “{q}”.
              </p>
            </div>
          )}

          {query.isSuccess && hits.length > 0 && (
            <ul>
              {hits.map((hit, i) => {
                const Icon = ICONS[hit.kind] ?? FileText;
                return (
                  <li key={`${hit.kind}-${hit.id}`}>
                    <button
                      type="button"
                      onMouseEnter={() => setCursor(i)}
                      onClick={() => go(hit)}
                      className={cn(
                        "flex w-full items-start gap-2.5 rounded-(--radius-ctl) px-2.5 py-2 text-left transition-colors",
                        i === cursor ? "bg-well" : "hover:bg-well/60",
                      )}
                    >
                      <Icon aria-hidden className="mt-0.5 size-3.5 shrink-0 text-ink-faint" />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-ui text-ink">
                          <Highlight text={hit.title} />
                        </span>
                        {hit.snippet && (
                          <span className="mt-0.5 block truncate text-caption text-ink-muted">
                            <Highlight text={hit.snippet} />
                          </span>
                        )}
                      </span>
                      {i === cursor && (
                        <CornerDownLeft aria-hidden className="mt-0.5 size-3 text-ink-faint" />
                      )}
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        {query.isSuccess && (
          <div className="mt-3 flex items-center gap-2 border-t border-line pt-3">
            <span className="font-mono text-micro text-ink-faint">
              {hits.length} {hits.length === 1 ? "result" : "results"}
            </span>
            <div className="flex-1" />
            {/* Postgres means Meilisearch is down. Say so. */}
            <Badge tone={query.data.source === "meili" ? "neutral" : "warning"}>
              {query.data.source === "meili" ? "meilisearch" : "postgres fallback"}
            </Badge>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
