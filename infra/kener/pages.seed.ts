import seedPagesData from "../src/lib/server/db/seedPagesData.ts";
import type { Knex } from "knex";

const homeMonitorTags = [
  "beeba-web",
  "beeba-api",
  "beeba-openapi",
  "beeba-search",
  "beeba-storage",
];

export async function seed(knex: Knex): Promise<void> {
  const pageCount = await knex("pages").count("id as CNT").first();

  if (pageCount && pageCount.CNT == 0) {
    for (const page of seedPagesData) {
      const [insertedPage] = await knex("pages")
        .insert({
          page_path: page.page_path,
          page_title: page.page_title,
          page_header: page.page_header,
          page_subheader: page.page_subheader,
          page_logo: page.page_logo,
          page_settings_json: page.page_settings_json,
          created_at: knex.fn.now(),
          updated_at: knex.fn.now(),
        })
        .returning("id");

      if (page.page_path === "") {
        const pageId = typeof insertedPage === "object" ? insertedPage.id : insertedPage;

        for (const [position, monitorTag] of homeMonitorTags.entries()) {
          const monitor = await knex("monitors").where({ tag: monitorTag }).first();
          if (!monitor) {
            continue;
          }

          await knex("pages_monitors").insert({
            page_id: pageId,
            monitor_tag: monitorTag,
            monitor_settings_json: "",
            position,
            created_at: knex.fn.now(),
            updated_at: knex.fn.now(),
          });
        }
      }
    }
  }
}
