import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import { BeeloyPaperProvider } from "./ui/theme";
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BeeloyPaperProvider><App /></BeeloyPaperProvider>
  </StrictMode>,
);
