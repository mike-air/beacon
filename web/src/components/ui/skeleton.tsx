import { cn } from "@/lib/cn";

export function Skeleton({ className, ...rest }: React.ComponentProps<"div">) {
  return (
    <div
      aria-hidden
      {...rest}
      className={cn("animate-pulse rounded-(--radius-ctl) bg-well", className)}
    />
  );
}
