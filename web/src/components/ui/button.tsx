import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/cn";

const button = cva(
  [
    "inline-flex items-center justify-center gap-1.5 select-none",
    "font-medium rounded-(--radius-ctl) transition-colors duration-140",
    "disabled:opacity-50 disabled:pointer-events-none",
  ],
  {
    variants: {
      variant: {
        primary: "bg-accent text-on-accent hover:bg-accent-hover active:bg-accent-active",
        secondary: "bg-raised text-ink border border-line-strong hover:bg-well",
        ghost: "text-ink-muted hover:bg-well hover:text-ink",
        danger: "bg-danger text-on-accent hover:opacity-90",
      },
      size: {
        sm: "h-7 px-2.5 text-ui",
        md: "h-9 px-3.5 text-sm",
        lg: "h-10 px-5 text-sm",
      },
    },
    defaultVariants: { variant: "primary", size: "md" },
  },
);

type ButtonProps = Omit<React.ComponentProps<"button">, "color"> &
  VariantProps<typeof button> & {
    /** busy implies disabled — the caller never passes both. (ch13) */
    busy?: boolean;
    /**
     * Render the child instead of a <button>, keeping the styles. This is how
     * a link gets to look like a button without a <button> wrapping an <a>,
     * which is invalid HTML and confuses every screen reader. (ch13)
     */
    asChild?: boolean;
  };

export function Button({
  className,
  variant,
  size,
  busy,
  asChild,
  children,
  ...rest
}: ButtonProps) {
  const Comp = asChild ? Slot : "button";
  return (
    <Comp
      {...(asChild ? {} : { type: "button" as const })}
      {...rest}
      disabled={busy || rest.disabled}
      className={cn(button({ variant, size }), className)}
    >
      {asChild ? (
        children
      ) : (
        <>
          {busy && <Loader2 aria-hidden className="size-3.5 animate-spin" />}
          {children}
        </>
      )}
    </Comp>
  );
}
