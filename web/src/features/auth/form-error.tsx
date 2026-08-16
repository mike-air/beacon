import { AlertTriangle } from "lucide-react";
import { BeaconError, NetworkError } from "@beacon/sdk";

/**
 * What to say when a submit fails.
 *
 * Field-level failures are attached to their inputs by the form; this is for
 * the ones with nowhere else to go — wrong password, rate limit, server down.
 */
export function FormError({ error }: { error: unknown }) {
  if (!error) return null;

  let message: string;
  if (error instanceof BeaconError) {
    if (error.isValidation && error.fields.length > 0) return null; // shown per field
    message = error.isRateLimited
      ? `Too many attempts. Try again in ${error.retryAfter ?? 60} seconds.`
      : error.message;
  } else if (error instanceof NetworkError) {
    message = "Could not reach the server. Check your connection and try again.";
  } else {
    message = "Something went wrong. Try again.";
  }

  return (
    <div
      role="alert"
      className="mb-4 flex items-start gap-2 rounded-(--radius-ctl) bg-danger-subtle px-3 py-2.5"
    >
      <AlertTriangle className="mt-px size-4 shrink-0 text-danger-text" />
      <p className="text-[13px] text-danger-text">{message}</p>
    </div>
  );
}
