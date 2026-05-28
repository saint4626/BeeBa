import seedSiteData from "../src/lib/server/db/seedSiteData.ts";
import type { Knex } from "knex";

export async function seed(knex: Knex): Promise<void> {
  const seedDataRecord = seedSiteData as Record<string, unknown>;

  for (const key in seedDataRecord) {
    if (!Object.prototype.hasOwnProperty.call(seedDataRecord, key)) {
      continue;
    }

    let value = seedDataRecord[key];
    const data_type = typeof value;
    if (data_type === "object") {
      value = JSON.stringify(value);
    }

    const existingEntry = await knex("site_data").where({ key }).first();
    if (existingEntry) {
      await knex("site_data")
        .where({ key })
        .update({ value: String(value ?? ""), data_type });
      continue;
    }

    await knex("site_data").insert([{ key, value: String(value ?? ""), data_type }]);
  }
}
