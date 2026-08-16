import { useState } from "react";
import type { UseFormSetError, FieldValues, Path } from "react-hook-form";
import { BeaconError } from "@beacon/sdk";

/**
 * One submit handler for every form in the app.
 *
 * It does the two things that are easy to forget and always the same: route
 * the server's 422 detail onto the right inputs, and keep whatever is left
 * over for the form-level banner. A field the server names but the form does
 * not have is added to the banner rather than silently dropped — otherwise a
 * failed save shows no error at all.
 */
export function useSubmit<T extends FieldValues>(
  setError: UseFormSetError<T>,
  knownFields: readonly Path<T>[],
) {
  const [error, setSubmitError] = useState<unknown>(null);

  async function run(fn: () => Promise<void>) {
    setSubmitError(null);
    try {
      await fn();
      return true;
    } catch (e) {
      if (e instanceof BeaconError && e.isValidation) {
        const unmatched: string[] = [];
        for (const [field, message] of Object.entries(e.fieldErrors())) {
          if ((knownFields as readonly string[]).includes(field)) {
            setError(field as Path<T>, { message });
          } else {
            unmatched.push(`${field}: ${message}`);
          }
        }
        setSubmitError(
          unmatched.length > 0
            ? new BeaconError({
                status: e.status,
                code: e.code,
                message: unmatched.join(", "),
              })
            : null,
        );
      } else {
        setSubmitError(e);
      }
      return false;
    }
  }

  return { error, run, clear: () => setSubmitError(null) };
}
