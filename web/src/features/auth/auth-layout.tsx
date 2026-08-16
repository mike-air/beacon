import { Wordmark } from "@/components/ui/logo";

/**
 * The signed-out frame. One column, centred, no navigation — there is exactly
 * one thing to do on these screens and nowhere else to go.
 */
export function AuthLayout({
  title,
  subtitle,
  children,
  footer,
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
}) {
  return (
    <div className="flex min-h-screen flex-col bg-sunken">
      <header className="px-6 py-5">
        <Wordmark live />
      </header>
      <main className="flex flex-1 items-start justify-center px-6 pb-20 pt-6 sm:pt-14">
        <div className="w-full max-w-sm">
          <h1 className="font-display text-2xl tracking-tight text-ink">{title}</h1>
          {subtitle && <p className="mt-1.5 text-[13.5px] text-ink-muted">{subtitle}</p>}
          <div className="mt-6 rounded-(--radius-card) border border-line bg-raised p-5">
            {children}
          </div>
          {footer && <div className="mt-4 text-center text-[13px] text-ink-muted">{footer}</div>}
        </div>
      </main>
    </div>
  );
}
