import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { cn } from "@/lib/cn";

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;

export function DialogContent({
  className,
  children,
  title,
  description,
  ...rest
}: React.ComponentProps<typeof DialogPrimitive.Content> & {
  title: string;
  description?: string;
}) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/40 animate-fade" />
      <DialogPrimitive.Content
        {...rest}
        className={cn(
          "fixed left-1/2 top-[38%] z-50 w-full max-w-md -translate-x-1/2 -translate-y-1/2",
          "rounded-(--radius-pop) border border-line bg-overlay p-5 shadow-(--shadow-pop)",
          "animate-rise focus:outline-none",
          className,
        )}
      >
        <DialogPrimitive.Title className="text-[15px] font-semibold text-ink">
          {title}
        </DialogPrimitive.Title>
        {description ? (
          <DialogPrimitive.Description className="mt-1 text-[13px] text-ink-muted">
            {description}
          </DialogPrimitive.Description>
        ) : (
          // Radix warns without a description; render an empty one intentionally.
          <DialogPrimitive.Description className="sr-only">{title}</DialogPrimitive.Description>
        )}
        <div className="mt-4">{children}</div>
        <DialogPrimitive.Close
          aria-label="Close"
          className="absolute right-3.5 top-3.5 rounded-md p-1 text-ink-faint transition-colors hover:bg-well hover:text-ink"
        >
          <X className="size-4" />
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}
