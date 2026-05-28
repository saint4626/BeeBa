import Toastify from "toastify-js";
import "toastify-js/src/toastify.css";

export type ToastKind = "success" | "error" | "info";

interface ToastOptions {
  kind?: ToastKind;
  duration?: number;
}

export function showToast(message: string, options: ToastOptions | ToastKind = {}) {
  const resolvedOptions = typeof options === "string" ? { kind: options } : options;
  const kind = resolvedOptions.kind ?? "info";
  Toastify({
    text: message,
    duration: resolvedOptions.duration ?? 2800,
    close: true,
    gravity: "bottom",
    position: "right",
    stopOnFocus: true,
    escapeMarkup: true,
    oldestFirst: false,
    ariaLive: kind === "error" ? "assertive" : "polite",
    className: `beeba-toast beeba-toast--${kind}`,
  }).showToast();
}
