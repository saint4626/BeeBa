export const PAGE_PROGRESS_START_EVENT = "beeba:page-progress-start";
export const PAGE_PROGRESS_DONE_EVENT = "beeba:page-progress-done";

export interface PageProgressController {
  start: () => void;
  done: () => void;
}

declare global {
  interface Window {
    __beebaPageProgress?: PageProgressController;
  }
}

export interface PageProgressLinkState {
  currentURL: string;
  href: string | null;
  defaultPrevented?: boolean;
  button?: number;
  hasModifierKey?: boolean;
  target?: string;
  download?: boolean;
  ariaDisabled?: boolean;
}

export interface PageProgressFormState {
  defaultPrevented: boolean;
  target?: string;
}

export function shouldStartPageProgressForLink(state: PageProgressLinkState): boolean {
  if (
    state.defaultPrevented ||
    (state.button ?? 0) !== 0 ||
    state.hasModifierKey ||
    state.target ||
    state.download ||
    state.ariaDisabled
  ) {
    return false;
  }

  const href = state.href?.trim();
  if (!href || href.startsWith("mailto:") || href.startsWith("tel:")) {
    return false;
  }

  const url = new URL(href, state.currentURL);
  const current = new URL(state.currentURL);
  return !(
    url.origin === current.origin &&
    url.pathname === current.pathname &&
    url.search === current.search &&
    url.hash
  );
}

export function shouldStartPageProgressForForm(state: PageProgressFormState): boolean {
  return !state.defaultPrevented && !state.target;
}

export function requestPageProgressStart(win: Window = window): void {
  if (win.__beebaPageProgress) {
    win.__beebaPageProgress.start();
    return;
  }
  win.dispatchEvent(new CustomEvent(PAGE_PROGRESS_START_EVENT));
}

export function requestPageProgressDone(win: Window = window): void {
  if (win.__beebaPageProgress) {
    win.__beebaPageProgress.done();
    return;
  }
  win.dispatchEvent(new CustomEvent(PAGE_PROGRESS_DONE_EVENT));
}

export function navigateWithPageProgress(href: string, mode: "assign" | "replace" = "assign"): void {
  requestPageProgressStart();
  if (mode === "replace") {
    window.location.replace(href);
  } else {
    window.location.assign(href);
  }
}
