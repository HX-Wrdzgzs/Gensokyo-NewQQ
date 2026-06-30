<template>
  <textarea
    class="editor"
    :value="modelValue"
    @input="$emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
    spellcheck="false"
  ></textarea>
</template>
<style scoped lang="scss">
.editor {
  width: 100%;
  height: 100%;
  min-height: 300px;
  padding: 12px;
  font-family: 'Roboto Mono', 'Consolas', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.5;
  border: 1px solid var(--gsk-border, #ddd);
  border-radius: 6px;
  background: var(--gsk-surface, #1e1e2e);
  color: var(--gsk-text, #cdd6f4);
  resize: vertical;
  tab-size: 2;
  outline: none;
  &:focus {
    border-color: var(--gsk-primary, #6366f1);
  }
}
</style>
<script setup lang="ts">
defineProps<{
  modelValue: string;
  language: string;
  theme?: string;
}>();
defineEmits(['update:modelValue']);
</script>