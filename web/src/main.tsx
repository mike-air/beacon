import "@fontsource/archivo-black/400.css";
import "@fontsource/space-grotesk/400.css";
import "@fontsource/space-grotesk/500.css";
import "@fontsource/space-grotesk/700.css";
import "@fontsource/jetbrains-mono/400.css";
import "./styles/app.css";

import { configureBeacon } from "@beacon/sdk";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { shouldRetry } from "@/api/queries";
import { ToastHost } from "@/components/ui/toast";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AuthProvider } from "@/features/auth/auth-context";
import { watchTheme } from "@/lib/theme";
import { router } from "@/routes/router";
import { clearToken, getToken } from "@/api/session";
import { API_BASE } from "@/api/config";

watchTheme();

/**
 * The SDK is configured once, here, before anything renders.
 *
 * Everything Beacon-specific about a request — the bearer token, the
 * Idempotency-Key on mutations, Retry-After on 429, the timeout, and what to
 * do when the token expires — is declared in this one call. No component ever
 * assembles a request, and no call site can forget any of it.
 *
 * onUnauthenticated is a full document load rather than a route change for the
 * same reason sign-out is: it guarantees no component state survives an
 * expired session. Beacon issues one non-refreshable token, so there is
 * nothing to retry with; ending the session is the only correct response.
 */
configureBeacon({
  baseUrl: API_BASE,
  getToken,
  onUnauthenticated: () => {
    clearToken();
    window.location.assign("/sign-in");
  },
});

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: shouldRetry,
      // The SSE stream already tells us when something changed, so polling on
      // every window focus is duplicate work. A minute of staleness is the
      // budget for whatever the stream drops.
      staleTime: 60_000,
      refetchOnWindowFocus: false,
    },
    mutations: { retry: false },
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <ToastHost>
          <AuthProvider>
            <RouterProvider router={router} />
          </AuthProvider>
        </ToastHost>
      </TooltipProvider>
    </QueryClientProvider>
  </StrictMode>,
);
