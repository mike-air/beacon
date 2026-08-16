import { cn } from "@/lib/cn";

export function Skeleton({ className, ...rest }: React.ComponentProps<"div">) {
  return (
    <div
      aria-hidden
      {...rest}
      className={cn("animate-pulse rounded-md bg-well", className)}
    />
  );
}
