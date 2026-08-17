import * as ToastPrimitive from "@radix-ui/react-toast";
import { AlertTriangle, CheckCircle2, Info, XCircle } from "lucide-react";
import { createContext, useCallback, useContext, useMemo, useState } from "react";
import { cn } from "@/lib/cn";

type Tone = "info" | "success" | "warning" | "danger";

type ToastInput = { title: string; description?: string; tone?: Tone };
type ToastItem = ToastInput & { id: number };

const ToastCtx = createContext<((t: ToastInput) => void) | null>(null);

/** The one way anything in the app says something out loud. */
export function useToast() {
  const ctx = useContext(ToastCtx);
  if (!ctx) throw new Error("useToast must be used inside <ToastHost>");
  return ctx;
}

const icons = {
  info: Info,
  success: CheckCircle2,
  warning: AlertTriangle,
  danger: XCircle,
} as const;

const toneText = {
  info: "text-ink-muted",
  success: "text-success-text",
  warning: "text-warning-text",
  danger: "text-danger-text",
} as const;

export function ToastHost({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const push = useCallback((t: ToastInput) => {
    setItems((prev) => [...prev, { ...t, id: Date.now() + Math.random() }]);
  }, []);
  const value = useMemo(() => push, [push]);

  return (
    <ToastCtx.Provider value={value}>
      <ToastPrimitive.Provider swipeDirection="right" duration={5000}>
        {children}
        {items.map((t) => {
          const Icon = icons[t.tone ?? "info"];
          return (
            <ToastPrimitive.Root
              key={t.id}
              onOpenChange={(open) =>
                !open && setItems((prev) => prev.filter((x) => x.id !== t.id))
              }
              className={cn(
                "flex items-start gap-2.5 rounded-(--radius-pop) border border-line bg-overlay",
                "p-3.5 shadow-(--shadow-pop) animate-rise",
                "data-[swipe=end]:translate-x-[var(--radix-toast-swipe-end-x)]",
              )}
            >
              <Icon className={cn("mt-px size-4 shrink-0", toneText[t.tone ?? "info"])} />
              <div className="min-w-0">
                <ToastPrimitive.Title className="text-ui font-medium text-ink">
                  {t.title}
                </ToastPrimitive.Title>
                {t.description && (
                  <ToastPrimitive.Description className="mt-0.5 text-caption text-ink-muted">
                    {t.description}
                  </ToastPrimitive.Description>
                )}
              </div>
            </ToastPrimitive.Root>
          );
        })}
        <ToastPrimitive.Viewport className="fixed bottom-4 right-4 z-100 flex w-80 flex-col gap-2 outline-none" />
      </ToastPrimitive.Provider>
    </ToastCtx.Provider>
  );
}
