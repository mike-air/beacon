import { cn } from "@/lib/cn";

/** Rules, not boxes: a table gets horizontal hairlines and nothing else. */
export function Table({ className, ...rest }: React.ComponentProps<"table">) {
  return (
    <div className="overflow-x-auto">
      <table {...rest} className={cn("w-full border-collapse text-sm", className)} />
    </div>
  );
}

export function Th({ className, ...rest }: React.ComponentProps<"th">) {
  return (
    <th
      {...rest}
      className={cn(
        "border-b border-line px-3 py-2 text-left text-label font-medium text-ink-muted",
        className,
      )}
    />
  );
}

export function Td({ className, ...rest }: React.ComponentProps<"td">) {
  return <td {...rest} className={cn("border-b border-line px-3 py-2.5 text-ink", className)} />;
}

export function Tr({ className, ...rest }: React.ComponentProps<"tr">) {
  return <tr {...rest} className={cn("transition-colors hover:bg-sunken", className)} />;
}
