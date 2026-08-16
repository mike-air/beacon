/**
 * Moving a card, optimistically.
 *
 * The card moves the moment you let go, because waiting 80ms for a round trip
 * to redraw a card you just dragged feels broken. That means the cache is
 * ahead of the server, and every optimistic update owes three things:
 *
 *   1. cancel in-flight refetches, or one landing mid-drag overwrites the
 *      optimistic state with the pre-move server copy
 *   2. keep the previous cache, and put it back on failure
 *   3. invalidate on settle — success as well as failure — so the server's
 *      version, not ours, is what is finally on screen
 *
 * On failure the card snaps back AND says why. A silent revert looks like the
 * drag simply did not register, and the user tries again.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { tasks as tasksApi } from "@/api/endpoints";
import { keys } from "@/api/queries";
import { BeaconError } from "@beacon/sdk";
import { useToast } from "@/components/ui/toast";
import type { Task, TaskStatus } from "@/api/types";

type Move = { task: Task; status: TaskStatus; position: number };
type Page = { items: Task[]; limit: number; offset: number; dropped: number };

export function useMoveTask(orgID: string, projectID: string) {
  const qc = useQueryClient();
  const toast = useToast();
  const key = keys.tasks(orgID, projectID);

  return useMutation({
    mutationFn: ({ task, status, position }: Move) =>
      // PATCH is a full replacement of the mutable fields: title has to go
      // back too, or the server would read "" and blank the card.
      tasksApi.update(orgID, projectID, task.id, { title: task.title, status, position }),

    onMutate: async ({ task, status, position }: Move) => {
      await qc.cancelQueries({ queryKey: key });
      const previous = qc.getQueryData<Page>(key);
      qc.setQueryData<Page>(key, (old) =>
        old
          ? {
              ...old,
              items: old.items.map((t) => (t.id === task.id ? { ...t, status, position } : t)),
            }
          : old,
      );
      return { previous };
    },

    onError: (error, _vars, context) => {
      if (context?.previous) qc.setQueryData(key, context.previous);
      const rateLimited = error instanceof BeaconError && error.isRateLimited;
      toast({
        tone: "danger",
        title: "The card moved back",
        description: rateLimited
          ? `Too many changes at once. Try again in ${(error as BeaconError).retryAfter ?? 5}s.`
          : error instanceof BeaconError
            ? error.message
            : "The server did not accept the move.",
      });
    },

    onSettled: () => {
      void qc.invalidateQueries({ queryKey: key });
    },
  });
}
