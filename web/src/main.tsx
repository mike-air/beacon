import "@fontsource/archivo-black/400.css";
import "@fontsource/space-grotesk/400.css";
import "@fontsource/space-grotesk/500.css";
import "@fontsource/space-grotesk/700.css";
import "@fontsource/jetbrains-mono/400.css";
import "./styles/app.css";

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

watchTheme();

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
