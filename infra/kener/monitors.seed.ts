import monitorSeed from "../src/lib/server/db/seedMonitorData.ts";
import type { Knex } from "knex";

export async function seed(knex: Knex): Promise<void> {
  for (const monitor of monitorSeed) {
    const monitorPayload = {
      name: monitor.name,
      description: monitor.description,
      image: monitor.image,
      cron: monitor.cron,
      default_status: monitor.default_status,
      status: monitor.status,
      category_name: monitor.category_name,
      monitor_type: monitor.monitor_type,
      down_trigger: monitor.down_trigger,
      degraded_trigger: monitor.degraded_trigger,
      type_data: monitor.type_data,
      day_degraded_minimum_count: monitor.day_degraded_minimum_count,
      day_down_minimum_count: monitor.day_down_minimum_count,
      include_degraded_in_downtime: monitor.include_degraded_in_downtime,
      is_hidden: monitor.is_hidden || "NO",
      monitor_settings_json: monitor.monitor_settings_json || null,
      updated_at: knex.fn.now(),
    };

    const existingMonitor = await knex("monitors").where({ tag: monitor.tag }).first();
    if (existingMonitor) {
      await knex("monitors").where({ tag: monitor.tag }).update(monitorPayload);
      continue;
    }

    await knex("monitors").insert({
      tag: monitor.tag,
      ...monitorPayload,
      created_at: knex.fn.now(),
    });
  }
}
