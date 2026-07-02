import type { Locale } from "../i18n";

export interface ServerLanguageOption {
  value: string;
  label: string;
}

const LANGUAGE_OPTIONS: Array<{ value: string; labels: Record<Locale, string> }> = [
  { value: "en", labels: { en: "English", ru: "Английский" } },
  { value: "ru", labels: { en: "Russian", ru: "Русский" } },
  { value: "de", labels: { en: "German", ru: "Немецкий" } },
  { value: "fr", labels: { en: "French", ru: "Французский" } },
  { value: "es", labels: { en: "Spanish", ru: "Испанский" } },
  { value: "pt-br", labels: { en: "Portuguese (Brazil)", ru: "Португальский (Бразилия)" } },
  { value: "zh-cn", labels: { en: "Chinese (Simplified)", ru: "Китайский (упрощённый)" } },
  { value: "ja", labels: { en: "Japanese", ru: "Японский" } },
  { value: "ko", labels: { en: "Korean", ru: "Корейский" } },
  { value: "multi", labels: { en: "Multilingual", ru: "Мультиязычный" } },
];

export function serverLanguageOptions(locale: Locale, emptyLabel: string, currentValue = ""): ServerLanguageOption[] {
  const options: ServerLanguageOption[] = [
    { value: "", label: emptyLabel },
    ...LANGUAGE_OPTIONS.map((option) => ({ value: option.value, label: option.labels[locale] })),
  ];
  const normalized = normalizeServerLanguageValue(currentValue);
  if (normalized && !options.some((option) => option.value === normalized)) {
    options.push({ value: normalized, label: currentValue.trim() });
  }
  return options;
}

export function serverLanguageLabel(value: string, locale: Locale, emptyLabel: string): string {
  const normalized = normalizeServerLanguageValue(value);
  if (!normalized) return emptyLabel;
  return LANGUAGE_OPTIONS.find((option) => option.value === normalized)?.labels[locale] ?? value;
}

export function normalizeServerLanguageValue(value: string): string {
  return value.trim().replaceAll("_", "-").toLowerCase();
}
