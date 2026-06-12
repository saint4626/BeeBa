<script setup lang="ts">
import { defineComponent, h, onMounted, ref, type PropType } from "vue";
import { Flag, Heart, MessageSquare, Send, X } from "lucide";
import { readAuthSession } from "../../lib/auth/session";
import { createComment, likeContent, listComments, reportContent, unlikeContent } from "../../lib/api/social";
import { ui, type Locale } from "../../lib/i18n";
import type { PublicComment } from "../../lib/api/types";
import { navigateWithPageProgress } from "../../lib/ui/page-progress";
import { showToast } from "../../lib/ui/toast";

type IconNode = Array<[string, Record<string, string>]>;

const Icon = defineComponent({
  name: "LucideInlineIcon",
  props: {
    node: { type: Array as PropType<IconNode>, required: true },
    size: { type: Number, default: 15 },
  },
  setup(props) {
    return () =>
      h(
        "svg",
        {
          width: props.size,
          height: props.size,
          viewBox: "0 0 24 24",
          fill: "none",
          stroke: "currentColor",
          "stroke-width": "2",
          "stroke-linecap": "round",
          "stroke-linejoin": "round",
          "aria-hidden": "true",
          focusable: "false",
        },
        props.node.map(([tag, attrs]) => h(tag, attrs)),
      );
  },
});

const MessageSquareIcon = MessageSquare as IconNode;
const HeartIcon = Heart as IconNode;
const FlagIcon = Flag as IconNode;
const SendIcon = Send as IconNode;
const CloseIcon = X as IconNode;

const props = defineProps<{
  contentId: string;
  initialLikes: number;
  initialLiked: boolean;
  loginHref: string;
  locale?: Locale;
}>();

const locale = props.locale ?? "en";
const t = ui[locale].social;
const isAuthenticated = ref(false);
const comments = ref<PublicComment[]>([]);
const body = ref("");
const reason = ref("");
const details = ref("");
const likes = ref(props.initialLikes);
const liked = ref(props.initialLiked);
const loading = ref(false);
const error = ref("");
const commentDialogOpen = ref(false);
const reportDialogOpen = ref(false);
const mounted = ref(false);

onMounted(() => {
  mounted.value = true;
  const session = readAuthSession();
  if (session) {
    isAuthenticated.value = true;
  }
  refreshComments();
});

async function refreshComments() {
  try {
    comments.value = await listComments(props.contentId);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.commentsFailed;
  }
}

async function toggleLike() {
  if (!isAuthenticated.value) {
    navigateWithPageProgress(props.loginHref);
    return;
  }
  error.value = "";
  loading.value = true;
  try {
    const result = liked.value
      ? await unlikeContent("", props.contentId)
      : await likeContent("", props.contentId);
    liked.value = result.liked;
    likes.value = result.likes_count;
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.likeFailed;
  } finally {
    loading.value = false;
  }
}

async function submitComment() {
  if (!isAuthenticated.value) return;
  const text = body.value.trim();
  if (!text) {
    error.value = t.commentRequired;
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    const comment = await createComment("", props.contentId, text);
    body.value = "";
    commentDialogOpen.value = false;
    if (comment.status === "visible") {
      comments.value = [comment, ...comments.value];
      showToast(t.commentPublished, "success");
    } else {
      await refreshComments();
      showToast(t.commentQueued, "success");
    }
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.commentFailed;
    showToast(error.value, "error");
  } finally {
    loading.value = false;
  }
}

async function submitReport() {
  if (!isAuthenticated.value) return;
  const reportReason = reason.value.trim();
  if (!reportReason) {
    error.value = t.reportRequired;
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    await reportContent("", props.contentId, {
      reason: reportReason,
      details: details.value.trim(),
    });
    reason.value = "";
    details.value = "";
    reportDialogOpen.value = false;
    showToast(t.reportSubmitted, "success");
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.reportFailed;
    showToast(error.value, "error");
  } finally {
    loading.value = false;
  }
}

function openCommentDialog() {
  if (!isAuthenticated.value) {
    navigateWithPageProgress(props.loginHref);
    return;
  }
  error.value = "";
  commentDialogOpen.value = true;
}

function openReportDialog() {
  if (!isAuthenticated.value) {
    navigateWithPageProgress(props.loginHref);
    return;
  }
  error.value = "";
  reportDialogOpen.value = true;
}

function closeCommentDialog() {
  if (loading.value) return;
  commentDialogOpen.value = false;
}

function closeReportDialog() {
  if (loading.value) return;
  reportDialogOpen.value = false;
}
</script>

<template>
  <section class="social-panel" aria-labelledby="social-title">
    <div class="discussion-toolbar">
      <div class="discussion-titleblock">
        <p class="eyebrow">{{ t.eyebrow }}</p>
        <h2 id="social-title">{{ t.title }}</h2>
        <span>{{ comments.length }} {{ t.visibleComments }}</span>
      </div>
      <div class="discussion-actions">
        <button class="button button--secondary discussion-action" type="button" :disabled="loading" @click="openCommentDialog">
          <Icon :node="MessageSquareIcon" :size="15" />
          <span>{{ t.addComment }}</span>
        </button>
        <button class="button button--secondary discussion-action" type="button" :disabled="loading" @click="toggleLike">
          <Icon :node="HeartIcon" :size="15" />
          <span>{{ liked ? t.unlike : t.like }}</span>
          <strong>{{ likes }}</strong>
        </button>
        <button class="button button--ghost discussion-action" type="button" :disabled="loading" @click="openReportDialog">
          <Icon :node="FlagIcon" :size="15" />
          <span>{{ t.report }}</span>
        </button>
      </div>
    </div>

    <div v-if="!isAuthenticated" class="social-auth-cta">
      <p>{{ t.signInCopy }}</p>
      <a class="button button--secondary" :href="loginHref">{{ t.signIn }}</a>
    </div>

    <div v-if="error" class="message message--error">{{ error }}</div>

    <div v-if="comments.length > 0" class="comment-list">
      <article v-for="comment in comments" :key="comment.id" class="comment-item">
        <div class="comment-item__head">
          <span class="comment-avatar">{{ (comment.display_name || comment.username).slice(0, 2).toUpperCase() }}</span>
          <div>
            <strong>{{ comment.display_name || comment.username }}</strong>
            <span>{{ t.visibleComment }}</span>
          </div>
        </div>
        <p>{{ comment.body }}</p>
      </article>
    </div>
    <div v-else class="discussion-empty">
      <strong>{{ t.emptyTitle }}</strong>
      <p>{{ t.emptyCopy }}</p>
    </div>

    <Teleport v-if="mounted" to="body">
      <div v-if="commentDialogOpen" class="modal-backdrop" role="presentation" @click.self="closeCommentDialog">
        <section class="comment-modal" role="dialog" aria-modal="true" aria-labelledby="comment-modal-title">
          <div class="comment-modal__head">
            <h3 id="comment-modal-title">{{ t.addCommentTitle }}</h3>
            <button type="button" :aria-label="t.closeComment" @click="closeCommentDialog">
              <Icon :node="CloseIcon" :size="18" />
            </button>
          </div>
          <form class="comment-composer" @submit.prevent="submitComment">
            <textarea
              v-model="body"
              rows="5"
              maxlength="2000"
              :disabled="loading"
              :placeholder="t.thoughtsPlaceholder"
              autofocus
            ></textarea>
            <div class="comment-modal__actions">
              <button class="button button--secondary" type="button" :disabled="loading" @click="closeCommentDialog">{{ ui[locale].common.cancel }}</button>
              <button class="button button--primary" type="submit" :disabled="loading">
                <Icon :node="SendIcon" :size="15" />
                <span>{{ t.commentAction }}</span>
              </button>
            </div>
          </form>
        </section>
      </div>

      <div v-if="reportDialogOpen" class="modal-backdrop" role="presentation" @click.self="closeReportDialog">
        <section class="comment-modal comment-modal--narrow" role="dialog" aria-modal="true" aria-labelledby="report-modal-title">
          <div class="comment-modal__head">
            <h3 id="report-modal-title">{{ t.reportTitle }}</h3>
            <button type="button" :aria-label="t.closeReport" @click="closeReportDialog">
              <Icon :node="CloseIcon" :size="18" />
            </button>
          </div>
          <form class="comment-composer" @submit.prevent="submitReport">
            <label class="field">
              {{ t.reportReason }}
              <input v-model="reason" type="text" maxlength="120" :disabled="loading" />
            </label>
            <label class="field">
              {{ t.details }}
              <textarea v-model="details" rows="4" maxlength="2000" :disabled="loading"></textarea>
            </label>
            <div class="comment-modal__actions">
              <button class="button button--secondary" type="button" :disabled="loading" @click="closeReportDialog">{{ ui[locale].common.cancel }}</button>
              <button class="button button--primary" type="submit" :disabled="loading">
                <Icon :node="FlagIcon" :size="15" />
                <span>{{ t.report }}</span>
              </button>
            </div>
          </form>
        </section>
      </div>
    </Teleport>
  </section>
</template>
