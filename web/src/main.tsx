import "@fontsource/archivo-black/400.css";
import "@fontsource/space-grotesk/400.css";
import "@fontsource/space-grotesk/500.css";
import "@fontsource/space-grotesk/700.css";
import "@fontsource/jetbrains-mono/400.css";
import "./styles/app.css";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Styleguide } from "@/routes/styleguide";
import { ToastHost } from "@/components/ui/toast";
import { TooltipProvider } from "@/components/ui/tooltip";
import { watchTheme } from "@/lib/theme";

watchTheme();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <TooltipProvider>
      <ToastHost>
        <Styleguide />
      </ToastHost>
    </TooltipProvider>
  </StrictMode>,
);
