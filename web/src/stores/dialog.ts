import { defineStore } from "pinia";
import { ref } from "vue";
import { t } from "../i18n";

export interface PromptState {
  title: string;
  value: string;
  placeholder: string;
  confirmLabel: string;
  resolve: (value: string | null) => void;
}

export interface ConfirmState {
  title: string;
  message: string;
  danger: boolean;
  resolve: (value: boolean) => void;
}

export const useDialogStore = defineStore("dialog", () => {
  const prompt = ref<PromptState | null>(null);
  const confirm = ref<ConfirmState | null>(null);

  function ask(
    title: string,
    value = "",
    opts: { placeholder?: string; confirmLabel?: string } = {},
  ): Promise<string | null> {
    return new Promise((resolve) => {
      prompt.value = {
        title,
        value,
        placeholder: opts.placeholder ?? "",
        confirmLabel: opts.confirmLabel ?? t("dialog.confirm"),
        resolve,
      };
    });
  }

  function askConfirm(title: string, message = "", danger = true): Promise<boolean> {
    return new Promise((resolve) => {
      confirm.value = { title, message, danger, resolve };
    });
  }

  function resolvePrompt(value: string | null) {
    const current = prompt.value;
    prompt.value = null;
    current?.resolve(value);
  }

  function resolveConfirm(value: boolean) {
    const current = confirm.value;
    confirm.value = null;
    current?.resolve(value);
  }

  return { prompt, confirm, ask, askConfirm, resolvePrompt, resolveConfirm };
});
