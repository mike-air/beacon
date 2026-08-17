import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/cn";

const badge = cva(
  "inline-flex items-center gap-1 rounded-(--radius-ctl) px-2 py-px text-label font-mono font-medium",
  {
    variants: {
      tone: {
        neutral: "bg-well text-ink-muted",
        accent: "bg-accent-subtle text-accent-text",
        success: "bg-success-subtle text-success-text",
        warning: "bg-warning-subtle text-warning-text",
        danger: "bg-danger-subtle text-danger-text",
        volt: "bg-volt text-on-volt",
      },
    },
    defaultVariants: { tone: "neutral" },
  },
);

type BadgeProps = React.ComponentProps<"span"> & VariantProps<typeof badge>;

export function Badge({ className, tone, ...rest }: BadgeProps) {
  return <span {...rest} className={cn(badge({ tone }), className)} />;
}
