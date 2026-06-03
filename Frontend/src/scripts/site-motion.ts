import { gsap } from "gsap";
import Lenis from "lenis";
import { logout as logoutSession } from "../lib/api/auth";
import { getPostLogoutRedirectHref } from "../lib/auth/logout-redirect";
import { AUTH_USER_KEY, clearAuthSession } from "../lib/auth/session";
import { PAGE_PROGRESS_TIMING } from "../lib/config/runtime";
import {
  navigateWithPageProgress,
  PAGE_PROGRESS_DONE_EVENT,
  PAGE_PROGRESS_START_EVENT,
  requestPageProgressDone,
  requestPageProgressStart,
  shouldStartPageProgressForForm,
  shouldStartPageProgressForLink,
} from "../lib/ui/page-progress";
import { shouldInitGlobalSmoothScroll } from "../lib/ui/smooth-scroll";
import { showToast } from "../lib/ui/toast";
import type { PublicUser } from "../lib/api/types";

type Cleanup = () => void;
type NavUser = Pick<PublicUser, "username" | "display_name" | "avatar_image_id">;

declare global {
  interface Window {
    __beebaLenis?: Lenis;
    __beebaLenisRafStarted?: boolean;
    __beebaMotionCleanup?: Cleanup;
  }
}

const reduceMotion = () => window.matchMedia("(prefers-reduced-motion: reduce)").matches;
const ADMIN_ACCESS_KEY = "beeba.adminAccess";
let authSSRStateInvalidated = false;
let isSigningOut = false;

function prepareMotionDocument(doc: Document) {
  doc.documentElement.classList.add("beeba-motion");
}

function initLenis(): Cleanup {
  const scrollWrapper = document.querySelector<HTMLElement>("[data-site-scroll]");
  const scrollContent = document.querySelector<HTMLElement>("[data-site-scroll-content]");

  window.__beebaLenis?.destroy();
  window.__beebaLenis = undefined;

  if (!shouldInitGlobalSmoothScroll({
    pathname: window.location.pathname,
    reduceMotion: reduceMotion(),
    hasScrollWrapper: Boolean(scrollWrapper),
    hasScrollContent: Boolean(scrollContent),
  }) || !scrollWrapper || !scrollContent) {
    return () => {};
  }

  window.__beebaLenis = new Lenis({
    wrapper: scrollWrapper,
    content: scrollContent,
    eventsTarget: scrollWrapper,
    duration: 0.8,
    smoothWheel: true,
  });

  if (!window.__beebaLenisRafStarted) {
    window.__beebaLenisRafStarted = true;
    const raf = (time: number) => {
      window.__beebaLenis?.raf(time);
      requestAnimationFrame(raf);
    };
    requestAnimationFrame(raf);
  }

  return () => {
    window.__beebaLenis?.destroy();
    window.__beebaLenis = undefined;
  };
}

function initDropdowns(): Cleanup {
  const dropdowns = Array.from(document.querySelectorAll<HTMLDetailsElement>("[data-ui-dropdown]"));
  const scrollWrapper = document.querySelector<HTMLElement>("[data-site-scroll]");
  const cleanups: Cleanup[] = [];

  const closeDropdowns = () => {
    dropdowns.forEach((dropdown) => {
      dropdown.open = false;
    });
  };

  dropdowns.forEach((dropdown) => {
    const onToggle = () => {
      if (!dropdown.open) return;
      dropdowns.forEach((other) => {
        if (other !== dropdown) other.open = false;
      });
    };
    dropdown.addEventListener("toggle", onToggle);
    cleanups.push(() => dropdown.removeEventListener("toggle", onToggle));
  });

  const onDocumentClick = (event: MouseEvent) => {
    const target = event.target;
    if (!(target instanceof Node)) return;
    dropdowns.forEach((dropdown) => {
      if (!dropdown.contains(target)) dropdown.open = false;
    });
  };
  document.addEventListener("click", onDocumentClick);
  cleanups.push(() => document.removeEventListener("click", onDocumentClick));

  window.addEventListener("scroll", closeDropdowns, { passive: true });
  cleanups.push(() => window.removeEventListener("scroll", closeDropdowns));

  if (scrollWrapper) {
    scrollWrapper.addEventListener("scroll", closeDropdowns, { passive: true });
    cleanups.push(() => scrollWrapper.removeEventListener("scroll", closeDropdowns));
  }

  return () => cleanups.forEach((cleanup) => cleanup());
}

function initGlobalSearch(): Cleanup {
  const searchForm = document.querySelector<HTMLFormElement>("[data-global-search]");
  const searchCategory = document.querySelector<HTMLInputElement>("#global-search-category");
  const searchCategoryLabel = document.querySelector<HTMLElement>("[data-search-category-label]");
  const options = Array.from(document.querySelectorAll<HTMLButtonElement>("[data-search-category-option]"));
  const cleanups: Cleanup[] = [];

  options.forEach((option) => {
    const onClick = () => {
      if (!searchCategory || !searchCategoryLabel) return;
      searchCategory.value = option.dataset.value ?? "";
      searchCategoryLabel.textContent = option.textContent?.trim() ?? "";
      options.forEach((item) => {
        const isActive = item === option;
        item.classList.toggle("ui-dropdown__item--active", isActive);
        item.setAttribute("aria-pressed", isActive ? "true" : "false");
      });
      option.closest<HTMLDetailsElement>("[data-ui-dropdown]")?.removeAttribute("open");
    };
    option.addEventListener("click", onClick);
    cleanups.push(() => option.removeEventListener("click", onClick));
  });

  if (searchForm && searchCategory) {
    const onSubmit = () => {
      const selectedCategory = searchCategory.value.trim();
      const basePath = searchForm.action.replace(/\/catalog(?:\/[^/?#]+)?(?=[?#]|$)/, "/catalog");
      searchForm.action = selectedCategory ? `${basePath}/${encodeURIComponent(selectedCategory)}` : basePath;
      searchCategory.disabled = !selectedCategory;
    };
    searchForm.addEventListener("submit", onSubmit);
    cleanups.push(() => searchForm.removeEventListener("submit", onSubmit));
  }

  return () => cleanups.forEach((cleanup) => cleanup());
}

function initPageProgress(): Cleanup {
  const progress = document.querySelector<HTMLElement>("[data-page-progress]");
  const bar = progress?.querySelector<HTMLElement>("span");
  if (!progress || !bar) {
    return () => {};
  }

  let value = 0;
  let timer: number | undefined;
  let doneTimer: number | undefined;
  let watchdogTimer: number | undefined;

  const set = (next: number) => {
    value = Math.max(value, Math.min(next, 0.94));
    bar.style.transform = `scaleX(${value})`;
  };

  const stopTimer = () => {
    if (timer) {
      window.clearInterval(timer);
      timer = undefined;
    }
  };

  const stopDoneTimer = () => {
    if (doneTimer) {
      window.clearTimeout(doneTimer);
      doneTimer = undefined;
    }
  };

  const stopWatchdogTimer = () => {
    if (watchdogTimer) {
      window.clearTimeout(watchdogTimer);
      watchdogTimer = undefined;
    }
  };

  const start = () => {
    stopTimer();
    stopDoneTimer();
    stopWatchdogTimer();
    value = 0.08;
    progress.classList.remove("page-progress--done");
    progress.classList.add("page-progress--active");
    bar.style.transform = "scaleX(0.08)";
    timer = window.setInterval(() => set(value + (1 - value) * 0.18), PAGE_PROGRESS_TIMING.tickMs);
    watchdogTimer = window.setTimeout(() => done(), PAGE_PROGRESS_TIMING.watchdogMs);
  };

  const done = () => {
    stopTimer();
    stopDoneTimer();
    stopWatchdogTimer();
    progress.classList.add("page-progress--active", "page-progress--done");
    bar.style.transform = "scaleX(1)";
    doneTimer = window.setTimeout(() => {
      progress.classList.remove("page-progress--active", "page-progress--done");
      bar.style.transform = "scaleX(0)";
      value = 0;
      doneTimer = undefined;
    }, PAGE_PROGRESS_TIMING.completeDelayMs);
  };

  window.__beebaPageProgress = { start, done };

  const onClick = (event: MouseEvent) => {
    const target = event.target;
    if (!(target instanceof Element)) return;

    const link = target.closest<HTMLAnchorElement>("a[href]");
    if (!link || !shouldStartPageProgressForLink({
      currentURL: window.location.href,
      href: link.getAttribute("href"),
      defaultPrevented: event.defaultPrevented,
      button: event.button,
      hasModifierKey: event.metaKey || event.ctrlKey || event.shiftKey || event.altKey,
      target: link.target,
      download: link.hasAttribute("download"),
      ariaDisabled: link.getAttribute("aria-disabled") === "true",
    })) {
      return;
    }

    start();
  };

  const onSubmit = (event: SubmitEvent) => {
    const target = event.target;
    if (!(target instanceof HTMLFormElement) || !shouldStartPageProgressForForm({
      defaultPrevented: event.defaultPrevented,
      target: target.target,
    })) {
      return;
    }
    start();
  };

  const onPageShow = () => done();
  const onPageHide = () => {
    stopTimer();
    stopWatchdogTimer();
  };

  document.addEventListener("click", onClick);
  document.addEventListener("submit", onSubmit);
  document.addEventListener("astro:before-preparation", start);
  document.addEventListener("astro:page-load", done);
  window.addEventListener(PAGE_PROGRESS_START_EVENT, start);
  window.addEventListener(PAGE_PROGRESS_DONE_EVENT, done);
  window.addEventListener("pageshow", onPageShow);
  window.addEventListener("pagehide", onPageHide);
  if (document.readyState === "complete") {
    done();
  } else {
    window.addEventListener("load", done, { once: true });
  }

  return () => {
    stopTimer();
    stopDoneTimer();
    stopWatchdogTimer();
    if (window.__beebaPageProgress?.start === start) {
      window.__beebaPageProgress = undefined;
    }
    document.removeEventListener("click", onClick);
    document.removeEventListener("submit", onSubmit);
    document.removeEventListener("astro:before-preparation", start);
    document.removeEventListener("astro:page-load", done);
    window.removeEventListener(PAGE_PROGRESS_START_EVENT, start);
    window.removeEventListener(PAGE_PROGRESS_DONE_EVENT, done);
    window.removeEventListener("pageshow", onPageShow);
    window.removeEventListener("pagehide", onPageHide);
  };
}

function initHome(): Cleanup {
  const cards = document.querySelectorAll<HTMLElement>(".asset-card");
  const pendingCards = document.querySelectorAll<HTMLElement>(".asset-card[data-motion-card]:not([data-motion-ready])");
  const pendingHeroItems = document.querySelectorAll<HTMLElement>("[data-home-animate]:not([data-motion-ready])");

  if (reduceMotion()) {
    pendingCards.forEach((card) => {
      card.dataset.motionReady = "true";
    });
    pendingHeroItems.forEach((item) => {
      item.dataset.motionReady = "true";
    });
    return () => {};
  }

  let heroTween: gsap.core.Tween | undefined;
  if (pendingHeroItems.length) {
    heroTween = gsap.fromTo(
      pendingHeroItems,
      { autoAlpha: 0, y: 22 },
      {
        autoAlpha: 1,
        y: 0,
        duration: 0.7,
        stagger: 0.08,
        ease: "power3.out",
        onComplete: () => {
          pendingHeroItems.forEach((item) => {
            item.dataset.motionReady = "true";
          });
          gsap.set(pendingHeroItems, { clearProps: "opacity,visibility,transform" });
        },
      },
    );
  }

  let revealTween: gsap.core.Tween | undefined;
  if (pendingCards.length) {
    revealTween = gsap.fromTo(
      pendingCards,
      { autoAlpha: 0 },
      {
        autoAlpha: 1,
        duration: 0.45,
        stagger: 0.04,
        ease: "power2.out",
        delay: 0.08,
        clearProps: "transform",
        onComplete: () => {
          pendingCards.forEach((card) => {
            card.dataset.motionReady = "true";
          });
          gsap.set(pendingCards, { clearProps: "opacity,visibility,transform" });
        },
      },
    );
  }

  const cleanups: Cleanup[] = [];
  cards.forEach((card) => {
    const media = card.querySelector<HTMLElement>(".asset-card__image, .asset-card__placeholder");
    if (!media) return;
    gsap.set(card, { clearProps: "transform" });
    const enter = () => gsap.to(media, { scale: 1.055, duration: 0.32, ease: "power2.out" });
    const leave = () => gsap.to(media, { scale: 1, duration: 0.32, ease: "power2.out" });
    card.addEventListener("mouseenter", enter);
    card.addEventListener("mouseleave", leave);
    card.addEventListener("focus", enter);
    card.addEventListener("blur", leave);
    cleanups.push(() => {
      card.removeEventListener("mouseenter", enter);
      card.removeEventListener("mouseleave", leave);
      card.removeEventListener("focus", enter);
      card.removeEventListener("blur", leave);
    });
  });

  return () => {
    heroTween?.kill();
    revealTween?.kill();
    cleanups.forEach((cleanup) => cleanup());
  };
}

function parseNavUser(raw: string | null | undefined): NavUser | null {
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as Partial<NavUser>;
    if (!parsed.username) return null;
    return {
      username: parsed.username,
      display_name: parsed.display_name ?? null,
      avatar_image_id: parsed.avatar_image_id ?? null,
    };
  } catch {
    return null;
  }
}

function getStoredUser(): NavUser | null {
  try {
    return parseNavUser(window.sessionStorage.getItem(AUTH_USER_KEY));
  } catch {
    return null;
  }
}

function getSSRNavUser(doc: Document = document): NavUser | null {
  const nav = doc.querySelector<HTMLElement>("[data-auth-nav]");
  return parseNavUser(nav?.dataset.authUser);
}

function getNavUser(doc: Document = document): NavUser | null {
  return getStoredUser() ?? (authSSRStateInvalidated ? null : getSSRNavUser(doc));
}

function hasStoredAdminAccess(doc: Document = document): boolean {
  const ssrAccess = doc.querySelector<HTMLElement>("[data-auth-nav]")?.dataset.authAdminAccess;
  if (ssrAccess === "1") return true;
  try {
    const stored = window.sessionStorage.getItem(ADMIN_ACCESS_KEY);
    if (stored === "1") return true;
    if (stored === "0") return false;
  } catch {
    // Session storage can be unavailable in hardened browser modes.
  }
  return false;
}

function setStoredAdminAccess(isAllowed: boolean) {
  try {
    window.sessionStorage.setItem(ADMIN_ACCESS_KEY, isAllowed ? "1" : "0");
  } catch {
    // Session storage can be unavailable in hardened browser modes.
  }
}

function clearStoredAdminAccess() {
  try {
    window.sessionStorage.removeItem(ADMIN_ACCESS_KEY);
  } catch {
    // Session storage can be unavailable in hardened browser modes.
  }
}

function hasCookie(name: string): boolean {
  return document.cookie
    .split(";")
    .some((cookie) => cookie.trim().startsWith(`${name}=`));
}

async function syncCookieSessionUser(): Promise<"synced" | "cleared" | "unchanged"> {
  const storedUser = getStoredUser();
  if (!storedUser && !hasCookie("beeba_csrf_token")) {
    return "unchanged";
  }

  const response = await fetch("/api/v1/auth/me", {
    credentials: "include",
    headers: { Accept: "application/json" },
  });

  if (response.status === 401 || response.status === 403) {
    if (storedUser || getSSRNavUser()) {
      authSSRStateInvalidated = true;
      clearAuthSession();
      return "cleared";
    }
    return "unchanged";
  }
  if (!response.ok) {
    return "unchanged";
  }

  const payload = (await response.json()) as { data?: PublicUser };
  if (!payload.data?.username) {
    return "unchanged";
  }

  authSSRStateInvalidated = false;
  window.sessionStorage.setItem(AUTH_USER_KEY, JSON.stringify(payload.data));
  return "synced";
}

async function syncAdminAccess(): Promise<boolean> {
  if (!getNavUser()) {
    clearStoredAdminAccess();
    return false;
  }

  const response = await fetch("/api/v1/admin/status", {
    credentials: "include",
    headers: { Accept: "application/json" },
  });

  if (response.status === 401 || response.status === 403) {
    setStoredAdminAccess(false);
    return false;
  }
  if (!response.ok) {
    return hasStoredAdminAccess();
  }

  const payload = (await response.json()) as { data?: { roles?: string[] } };
  const roles = payload.data?.roles ?? [];
  const isAllowed = roles.some((role) => role === "moderator" || role === "admin" || role === "owner");
  setStoredAdminAccess(isAllowed);
  return isAllowed;
}

function renderAuthNavDocument(doc: Document) {
  const login = doc.querySelector<HTMLElement>("[data-auth-login]");
  const manage = doc.querySelector<HTMLAnchorElement>("[data-auth-manage]");
  const upload = doc.querySelector<HTMLAnchorElement>("[data-auth-upload]");
  const uploadHref = upload?.dataset.uploadHref ?? "";
  const menu = doc.querySelector<HTMLDetailsElement>("[data-auth-menu]");
  const summary = doc.querySelector<HTMLElement>("[data-auth-summary]");
  const avatar = doc.querySelector<HTMLImageElement>("[data-auth-avatar]");
  const initials = doc.querySelector<HTMLElement>("[data-auth-initials]");

  if (!login || !menu || !summary || !avatar || !initials) {
    return;
  }

  const user = getNavUser(doc);
  const isAuthenticated = Boolean(user?.username);
  const canManage = isAuthenticated && hasStoredAdminAccess(doc);
  login.hidden = isAuthenticated;
  if (manage) {
    manage.hidden = !canManage;
  }
  menu.hidden = !isAuthenticated;
  if (upload) {
    upload.classList.toggle("button--disabled", !isAuthenticated);
    upload.setAttribute("aria-disabled", isAuthenticated ? "false" : "true");
    if (isAuthenticated) {
      upload.setAttribute("href", uploadHref);
    } else {
      upload.removeAttribute("href");
    }
  }
  doc.querySelectorAll<HTMLAnchorElement>("[data-auth-cta-upload]").forEach((cta) => {
    const target = isAuthenticated ? cta.dataset.uploadHref : cta.dataset.loginHref;
    if (target) {
      cta.href = target;
    }
  });
  if (user?.username) {
    const label = user.display_name || user.username;
    initials.textContent = label.slice(0, 2).toUpperCase();
    summary.setAttribute("title", label);
    if (user.avatar_image_id) {
      avatar.src = `/api/v1/media/${user.avatar_image_id}`;
      avatar.hidden = false;
      initials.hidden = true;
    } else {
      avatar.removeAttribute("src");
      avatar.hidden = true;
      initials.hidden = false;
    }
  } else {
    avatar.removeAttribute("src");
    avatar.hidden = true;
    initials.hidden = false;
    initials.textContent = "BB";
    summary.removeAttribute("title");
    clearStoredAdminAccess();
  }
}

function initAuthNav(): Cleanup {
  const upload = document.querySelector<HTMLAnchorElement>("[data-auth-upload]");
  const uploadClick = (event: MouseEvent) => {
    if (upload?.getAttribute("aria-disabled") === "true") {
      event.preventDefault();
    }
  };
  const menu = document.querySelector<HTMLDetailsElement>("[data-auth-menu]");
  const avatar = document.querySelector<HTMLImageElement>("[data-auth-avatar]");
  const initials = document.querySelector<HTMLElement>("[data-auth-initials]");
  const logout = document.querySelector<HTMLButtonElement>("[data-auth-logout]");
  if (!menu || !avatar || !initials || !logout) {
    return () => {};
  }

  upload?.addEventListener("click", uploadClick);

  const avatarError = () => {
    avatar.removeAttribute("src");
    avatar.hidden = true;
    initials.hidden = false;
  };
  avatar.addEventListener("error", avatarError);

  const render = () => {
    renderAuthNavDocument(document);
  };

  const refreshManagementAccess = () => {
    if (isSigningOut) return;
    void syncAdminAccess()
      .then(() => render())
      .catch(() => {
        // Keep current cached management visibility if the admin status probe is temporarily unavailable.
      });
  };

  const signOut = async () => {
    if (isSigningOut) return;
    isSigningOut = true;
    authSSRStateInvalidated = true;
    requestPageProgressStart();
    try {
      await logoutSession();
    } catch {
      showToast("Session cleared locally.", "info");
    }
    clearAuthSession();
    clearStoredAdminAccess();
    authSSRStateInvalidated = true;
    menu.open = false;
    render();
    window.dispatchEvent(new CustomEvent("beeba:session-logout"));
    const redirectHref = getPostLogoutRedirectHref(window.location);
    if (redirectHref) {
      navigateWithPageProgress(redirectHref);
    } else {
      isSigningOut = false;
      requestPageProgressDone();
    }
  };

  render();
  void syncCookieSessionUser()
    .then((state) => {
      if (state !== "unchanged") {
        render();
      }
      refreshManagementAccess();
    })
    .catch(() => {
      // Keep the SSR-rendered state if the session probe is temporarily unavailable.
    });
  logout.addEventListener("click", signOut);
  const onSessionChanged = () => {
    const hasSessionUser = Boolean(getStoredUser());
    authSSRStateInvalidated = !hasSessionUser;
    render();
    if (hasSessionUser) {
      refreshManagementAccess();
    } else {
      clearStoredAdminAccess();
    }
  };
  const onStorage = (event: StorageEvent) => {
    if (event.key === AUTH_USER_KEY && !event.newValue) {
      authSSRStateInvalidated = true;
    }
    render();
  };
  window.addEventListener("beeba:session-changed", onSessionChanged);
  window.addEventListener("storage", onStorage);

  return () => {
    upload?.removeEventListener("click", uploadClick);
    avatar.removeEventListener("error", avatarError);
    logout.removeEventListener("click", signOut);
    window.removeEventListener("beeba:session-changed", onSessionChanged);
    window.removeEventListener("storage", onStorage);
  };
}

function initCopyButtons(): Cleanup {
  const buttons = document.querySelectorAll<HTMLButtonElement>("[data-copy-text]");
  if (!buttons.length) {
    return () => {};
  }

  const cleanups: Cleanup[] = [];
  buttons.forEach((button) => {
    const originalLabel = button.getAttribute("aria-label") ?? "";
    const originalTitle = button.getAttribute("title") ?? "";
    let timer: number | undefined;
    const onClick = async () => {
      const text = button.dataset.copyText;
      if (!text) return;
      try {
        await navigator.clipboard.writeText(text);
        const copiedLabel = button.dataset.copyLabel ?? "Copied";
        button.classList.add("site-footer__icon-link--copied");
        button.setAttribute("aria-label", copiedLabel);
        button.setAttribute("title", copiedLabel);
        showToast(copiedLabel, { kind: "success" });
        window.clearTimeout(timer);
        timer = window.setTimeout(() => {
          button.classList.remove("site-footer__icon-link--copied");
          button.setAttribute("aria-label", originalLabel);
          button.setAttribute("title", originalTitle);
        }, 1400);
      } catch {
        button.setAttribute("title", text);
        showToast("Could not copy to clipboard", { kind: "error" });
      }
    };
    button.addEventListener("click", onClick);
    cleanups.push(() => {
      window.clearTimeout(timer);
      button.removeEventListener("click", onClick);
    });
  });

  return () => cleanups.forEach((cleanup) => cleanup());
}

function initContentCarousels(): Cleanup {
  const carousels = Array.from(document.querySelectorAll<HTMLElement>("[data-content-carousel]"));
  if (!carousels.length) {
    return () => {};
  }

  const cleanups: Cleanup[] = [];
  carousels.forEach((carousel) => {
    const track = carousel.querySelector<HTMLElement>("[data-carousel-track]");
    const slides = Array.from(carousel.querySelectorAll<HTMLElement>("[data-carousel-slide]"));
    const thumbs = Array.from(carousel.querySelectorAll<HTMLButtonElement>("[data-carousel-thumb]"));
    const prev = carousel.querySelector<HTMLButtonElement>("[data-carousel-prev]");
    const next = carousel.querySelector<HTMLButtonElement>("[data-carousel-next]");
    if (!track || slides.length <= 1) {
      return;
    }

    let activeIndex = 0;
    let scrollTimer: number | undefined;

    const setActive = (index: number) => {
      activeIndex = Math.max(0, Math.min(index, slides.length - 1));
      thumbs.forEach((thumb, thumbIndex) => {
        thumb.setAttribute("aria-current", thumbIndex === activeIndex ? "true" : "false");
      });
    };

    const goTo = (index: number) => {
      const target = slides[Math.max(0, Math.min(index, slides.length - 1))];
      target?.scrollIntoView({ behavior: reduceMotion() ? "auto" : "smooth", block: "nearest", inline: "center" });
      setActive(index);
    };

    const updateFromScroll = () => {
      window.clearTimeout(scrollTimer);
      scrollTimer = window.setTimeout(() => {
        const slideWidth = track.clientWidth || 1;
        setActive(Math.round(track.scrollLeft / slideWidth));
      }, 80);
    };

    const onPrev = () => goTo(activeIndex - 1);
    const onNext = () => goTo(activeIndex + 1);
    prev?.addEventListener("click", onPrev);
    next?.addEventListener("click", onNext);
    track.addEventListener("scroll", updateFromScroll, { passive: true });
    thumbs.forEach((thumb) => {
      const onClick = () => goTo(Number(thumb.dataset.slideIndex ?? "0"));
      thumb.addEventListener("click", onClick);
      cleanups.push(() => thumb.removeEventListener("click", onClick));
    });
    cleanups.push(() => {
      window.clearTimeout(scrollTimer);
      prev?.removeEventListener("click", onPrev);
      next?.removeEventListener("click", onNext);
      track.removeEventListener("scroll", updateFromScroll);
    });
  });

  return () => cleanups.forEach((cleanup) => cleanup());
}

function initMotion() {
  window.__beebaMotionCleanup?.();
  const cleanups = [initLenis(), initPageProgress(), initHome(), initAuthNav(), initDropdowns(), initGlobalSearch(), initCopyButtons(), initContentCarousels()];
  window.__beebaMotionCleanup = () => cleanups.forEach((cleanup) => cleanup());
}

document.addEventListener("astro:page-load", initMotion);
document.addEventListener("astro:before-swap", (event) => {
  const transitionEvent = event as Event & { newDocument?: Document };
  if (transitionEvent.newDocument) {
    prepareMotionDocument(transitionEvent.newDocument);
    renderAuthNavDocument(transitionEvent.newDocument);
  }
});
prepareMotionDocument(document);
initMotion();
