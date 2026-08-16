import { GripVertical, MoveRight } from "lucide-react";
import { STATUS_LABEL, TASK_STATUSES, type Task, type TaskStatus } from "@/api/types";
import {
  MenuContent,
  MenuItem,
  MenuLabel,
  MenuRoot,
  MenuSeparator,
  MenuTrigger,
} from "@/components/ui/menu";
import { cn } from "@/lib/cn";

/**
 * A card.
 *
 * Two ways to move it, because dragging is a mouse gesture and a keyboard has
 * no equivalent. The drag handle is for pointers; the menu on every card does
 * the same job for anyone using a keyboard or a screen reader, and it is not
 * hidden behind a hover state, because a control you cannot see is a control
 * you do not have.
 */
export function TaskCard({
  task,
  dragging,
  justUpdated,
  onOpen,
  onMoveTo,
  onDragStart,
  onDragEnd,
}: {
  task: Task;
  dragging: boolean;
  justUpdated: boolean;
  onOpen: () => void;
  onMoveTo: (status: TaskStatus) => void;
  onDragStart: (e: React.DragEvent) => void;
  onDragEnd: () => void;
}) {
  return (
    <article
      draggable
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      className={cn(
        "group flex items-start gap-1 rounded-(--radius-card) border border-line bg-raised",
        "px-2.5 py-2 transition-[border-color,opacity] duration-140",
        "hover:border-line-strong",
        dragging && "opacity-40",
        justUpdated && "animate-volt-wash",
      )}
    >
      <GripVertical
        aria-hidden
        className="mt-0.5 size-3.5 shrink-0 cursor-grab text-ink-faint opacity-0 transition-opacity group-hover:opacity-100"
      />

      <button
        type="button"
        onClick={onOpen}
        aria-label={`Open "${task.title}"`}
        className="min-w-0 flex-1 py-0.5 text-left"
      >
        <p className="text-[13.5px] leading-snug text-ink">{task.title}</p>
      </button>

      <MenuRoot>
        <MenuTrigger asChild>
          <button
            type="button"
            aria-label={`Move "${task.title}"`}
            className="rounded p-1 text-ink-faint opacity-0 transition-opacity hover:bg-well focus-visible:opacity-100 group-hover:opacity-100"
          >
            <MoveRight className="size-3.5" />
          </button>
        </MenuTrigger>
        <MenuContent align="end">
          <MenuLabel>Move to</MenuLabel>
          <MenuSeparator />
          {TASK_STATUSES.filter((s) => s !== task.status).map((s) => (
            <MenuItem key={s} onSelect={() => onMoveTo(s)}>
              {STATUS_LABEL[s]}
            </MenuItem>
          ))}
        </MenuContent>
      </MenuRoot>
    </article>
  );
}
