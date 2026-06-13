import assert from "node:assert/strict";
import { localizedPath } from "../src/lib/i18n/path.ts";
import { en } from "../src/lib/i18n/locales/en.ts";
import { ru } from "../src/lib/i18n/locales/ru.ts";

assert.equal(localizedPath("en", "/confirm-password-change?token=bb_pc_secret"), "/confirm-password-change?token=bb_pc_secret");
assert.equal(localizedPath("ru", "/confirm-email-change?token=bb_ec_secret"), "/ru/confirm-email-change?token=bb_ec_secret");

assert.equal(en.profile.passwordConfirmationQueued, "Password confirmation email queued. Open the link to finish the change.");
assert.equal(en.profile.emailChangeConfirmationQueuedPrefix, "Confirmation email queued for");
assert.equal(en.accountConfirm.passwordAction, "Confirm password change");
assert.equal(en.accountConfirm.emailAction, "Confirm email change");

assert.deepEqual(deepKeys(ru.accountConfirm), deepKeys(en.accountConfirm));

function deepKeys(value: unknown, prefix = ""): string[] {
  if (!value || typeof value !== "object") {
    return [prefix];
  }

  return Object.entries(value)
    .flatMap(([key, child]) => deepKeys(child, prefix ? `${prefix}.${key}` : key))
    .sort();
}
