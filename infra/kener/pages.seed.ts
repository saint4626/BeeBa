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
  for (const page of seedPagesData) {
    const existingPage = await knex("pages").where({ page_path: page.page_path }).first();
    const pagePayload = {
      page_title: page.page_title,
      page_header: page.page_header,
      page_subheader: page.page_subheader,
      page_logo: page.page_logo,
      page_settings_json: page.page_settings_json,
      updated_at: knex.fn.now(),
    };

    let pageId: string | number;
    if (existingPage) {
      await knex("pages").where({ id: existingPage.id }).update(pagePayload);
      pageId = existingPage.id;
    } else {
      const [insertedPage] = await knex("pages")
        .insert({
          page_path: page.page_path,
          ...pagePayload,
          created_at: knex.fn.now(),
        })
        .returning("id");

      pageId = typeof insertedPage === "object" ? insertedPage.id : insertedPage;
    }

    if (page.page_path === "") {
      for (const [position, monitorTag] of homeMonitorTags.entries()) {
        const monitor = await knex("monitors").where({ tag: monitorTag }).first();
        if (!monitor) {
          continue;
        }

        const existingLink = await knex("pages_monitors")
          .where({ page_id: pageId, monitor_tag: monitorTag })
          .first();

        if (existingLink) {
          await knex("pages_monitors")
            .where({ page_id: pageId, monitor_tag: monitorTag })
            .update({ position, updated_at: knex.fn.now() });
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
