import { useId } from "react";
import { cn } from "@/lib/cn";

type InputProps = Omit<React.ComponentProps<"input">, "size"> & {
  invalid?: boolean;
};

export function Input({ className, invalid, ...rest }: InputProps) {
  return (
    <input
      {...rest}
      aria-invalid={invalid || undefined}
      className={cn(
        "h-9 w-full rounded-(--radius-ctl) border bg-raised px-3 text-sm text-ink",
        "placeholder:text-ink-faint transition-colors duration-140",
        invalid ? "border-danger" : "border-line-strong hover:border-ink-faint focus:border-accent",
        "focus:outline-none focus-visible:outline-2 focus-visible:outline-ring",
        className,
      )}
    />
  );
}

export function Label({ className, ...rest }: React.ComponentProps<"label">) {
  return (
    <label
      {...rest}
      className={cn("mb-1.5 block text-ui font-medium text-ink", className)}
    />
  );
}

/** Label + input + error in one accessible unit — the error is announced. */
export function Field({
  label,
  error,
  children,
  hint,
}: {
  label: string;
  error?: string;
  hint?: string;
  children: (props: { id: string; "aria-describedby"?: string; invalid: boolean }) => React.ReactNode;
}) {
  const id = useId();
  const descId = `${id}-desc`;
  return (
    <div>
      <Label htmlFor={id}>{label}</Label>
      {children({ id, "aria-describedby": error || hint ? descId : undefined, invalid: !!error })}
      {error ? (
        <p id={descId} role="alert" className="mt-1 text-caption text-danger-text">{error}</p>
      ) : hint ? (
        <p id={descId} className="mt-1 text-caption text-ink-faint">{hint}</p>
      ) : null}
    </div>
  );
}
