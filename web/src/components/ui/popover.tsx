import * as PopoverPrimitive from "@radix-ui/react-popover";
import { cn } from "@/lib/cn";

export const Popover = PopoverPrimitive.Root;
export const PopoverTrigger = PopoverPrimitive.Trigger;

export function PopoverContent({
  className,
  sideOffset = 6,
  ...rest
}: React.ComponentProps<typeof PopoverPrimitive.Content>) {
  return (
    <PopoverPrimitive.Portal>
      <PopoverPrimitive.Content
        sideOffset={sideOffset}
        {...rest}
        className={cn(
          "z-50 w-64 rounded-(--radius-pop) border border-line bg-overlay p-3.5 shadow-(--shadow-pop) animate-rise focus:outline-none",
          className,
        )}
      />
    </PopoverPrimitive.Portal>
  );
}
