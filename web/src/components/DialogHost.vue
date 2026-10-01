<script setup lang="ts">
import { nextTick, ref, watch } from "vue";
import { useDialogStore } from "../stores/dialog";

const dialogs = useDialogStore();
const input = ref<HTMLInputElement | null>(null);

watch(
  () => dialogs.prompt,
  (state) => {
    if (state) nextTick(() => input.value?.focus());
  },
);

function onPromptKey(event: KeyboardEvent) {
  if (event.key === "Enter") dialogs.resolvePrompt(dialogs.prompt!.value);
  if (event.key === "Escape") dialogs.resolvePrompt(null);
}
</script>

<template>
  <Teleport to="body">
    <div v-if="dialogs.prompt" class="overlay" @click.self="dialogs.resolvePrompt(null)">
      <div class="dialog" @keydown="onPromptKey">
        <h3>{{ dialogs.prompt.title }}</h3>
        <input
          ref="input"
          v-model="dialogs.prompt.value"
          :placeholder="dialogs.prompt.placeholder"
        />
        <div class="dialog-actions">
          <button @click="dialogs.resolvePrompt(null)">取消</button>
          <button class="primary" @click="dialogs.resolvePrompt(dialogs.prompt!.value)">
            {{ dialogs.prompt.confirmLabel }}
          </button>
        </div>
      </div>
    </div>

    <div
      v-if="dialogs.confirm"
      class="overlay"
      @click.self="dialogs.resolveConfirm(false)"
    >
      <div class="dialog">
        <h3>{{ dialogs.confirm.title }}</h3>
        <p v-if="dialogs.confirm.message">{{ dialogs.confirm.message }}</p>
        <div class="dialog-actions">
          <button @click="dialogs.resolveConfirm(false)">取消</button>
          <button
            :class="dialogs.confirm.danger ? 'danger' : 'primary'"
            @click="dialogs.resolveConfirm(true)"
          >
            确定
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
