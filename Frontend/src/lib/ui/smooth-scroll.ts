export interface SmoothScrollInitState {
  pathname: string;
  reduceMotion: boolean;
  hasScrollWrapper: boolean;
  hasScrollContent: boolean;
}

export function shouldInitGlobalSmoothScroll(state: SmoothScrollInitState): boolean {
  return !state.reduceMotion && state.hasScrollWrapper && state.hasScrollContent;
}
