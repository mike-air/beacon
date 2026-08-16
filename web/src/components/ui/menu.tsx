import * as Menu from "@radix-ui/react-dropdown-menu";
import { cn } from "@/lib/cn";

export const MenuRoot = Menu.Root;
export const MenuTrigger = Menu.Trigger;

export function MenuContent({
  className,
  sideOffset = 5,
  ...rest
}: React.ComponentProps<typeof Menu.Content>) {
  return (
    <Menu.Portal>
      <Menu.Content
        sideOffset={sideOffset}
        {...rest}
        className={cn(
          "z-50 min-w-44 rounded-(--radius-pop) border border-line bg-overlay p-1 shadow-(--shadow-pop) animate-rise",
          className,
        )}
      />
    </Menu.Portal>
  );
}

export function MenuItem({
  className,
  destructive,
  ...rest
}: React.ComponentProps<typeof Menu.Item> & { destructive?: boolean }) {
  return (
    <Menu.Item
      {...rest}
      className={cn(
        "flex cursor-default select-none items-center gap-2 rounded-md px-2.5 py-1.5 text-[13px] outline-none",
        destructive
          ? "text-danger-text data-highlighted:bg-danger-subtle"
          : "text-ink data-highlighted:bg-well",
        className,
      )}
    />
  );
}

export function MenuSeparator() {
  return <Menu.Separator className="my-1 h-px bg-line" />;
}

export function MenuLabel({ className, ...rest }: React.ComponentProps<typeof Menu.Label>) {
  return (
    <Menu.Label
      {...rest}
      className={cn("px-2.5 py-1 text-[11px] font-medium text-ink-faint", className)}
    />
  );
}
