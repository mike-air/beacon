import * as CheckboxPrimitive from "@radix-ui/react-checkbox";
import { Check } from "lucide-react";
import { useId } from "react";
import { cn } from "@/lib/cn";

export function Checkbox({
  label,
  className,
  ...rest
}: React.ComponentProps<typeof CheckboxPrimitive.Root> & { label: string }) {
  const id = useId();
  return (
    <div className="flex items-center gap-2">
      <CheckboxPrimitive.Root
        id={id}
        {...rest}
        className={cn(
          "flex size-4.5 shrink-0 items-center justify-center rounded border border-line-strong bg-raised",
          "transition-colors duration-140 data-[state=checked]:border-accent data-[state=checked]:bg-accent",
          className,
        )}
      >
        <CheckboxPrimitive.Indicator>
          <Check className="size-3 text-on-accent" strokeWidth={3} />
        </CheckboxPrimitive.Indicator>
      </CheckboxPrimitive.Root>
      <label htmlFor={id} className="select-none text-sm text-ink">
        {label}
      </label>
    </div>
  );
}
