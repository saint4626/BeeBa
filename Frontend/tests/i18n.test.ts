import assert from "node:assert/strict";
import { localizedPath } from "../src/lib/i18n/path.ts";
import { en } from "../src/lib/i18n/locales/en.ts";
import { ru } from "../src/lib/i18n/locales/ru.ts";

assert.equal(localizedPath("en", "/catalog?q=avatar#top"), "/catalog?q=avatar#top");
assert.equal(localizedPath("ru", "/catalog?q=avatar#top"), "/ru/catalog?q=avatar#top");
assert.equal(localizedPath("en", "/ru/catalog/worlds?tags=featured"), "/catalog/worlds?tags=featured");
assert.equal(localizedPath("ru", "/ru/profile"), "/ru/profile");
assert.equal(localizedPath("ru", "/"), "/ru");

assert.deepEqual(deepKeys(ru), deepKeys(en));
assert.equal(ru.nav.home, "Главная");
assert.equal(ru.nav.catalog, "Каталог");
assert.ok(!JSON.stringify(ru).includes("Р“Р"));

function deepKeys(value: unknown, prefix = ""): string[] {
  if (!value || typeof value !== "object") {
    return [prefix];
  }

  return Object.entries(value)
    .flatMap(([key, child]) => deepKeys(child, prefix ? `${prefix}.${key}` : key))
    .sort();
}
